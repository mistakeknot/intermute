# intermute Product Requirements Document

## 1) Purpose

intermute is a Go service that coordinates work between multiple autonomous agents by managing:
- real-time agent registration and heartbeats,
- message delivery with thread/inbox semantics,
- advisory file reservations for conflict avoidance, and
- shared persistence for orchestration entities (specs, epics, stories, tasks, insights, sessions).

It is intended to be an operational infrastructure component, not a UI tool or a single-agent assistant.

This document defines requirements for the current service as shipped in `/root/projects/Interverse/services/intermute`.

## 2) Product scope

### In scope
- A single binary with HTTP + WebSocket interfaces.
- Lightweight multi-tenant project isolation using API keys.
- Durable state in SQLite with event-like append behavior for observability and recovery.
- Advisory file locking primitives for parallel agent collaboration.
- Domain entity APIs consumed by orchestrators and dashboards.

### Out of scope
- Authorization granularity beyond project-level bearer tokens and localhost bypass.
- Git-native merge conflict resolution.
- Long-term analytics or BI features; intermute is a coordination state layer.

## 3) Target users

- Agent runtimes (MCP-compatible agents and internal tooling).
- Orchestrators that spawn and coordinate many agents.
- Operator scripts that monitor service health and reconcile workflows.

## 4) User stories

1. As an orchestrator, I register an agent and receive a stable identifier so that all routing can be tracked through a project.
2. As an agent, I send structured messages with threading so that other agents can reconstruct conversational context.
3. As an agent, I reserve a set of files before editing so I avoid editing conflicts with peers.
4. As an operator, I observe inbox state and reservation health to identify stalled agents and blocked workflows.
5. As a scheduler, I create/update/read domain entities (specs, epics, stories, tasks, insights, sessions) in one service.

## 5) Requirements

### 5.1 Functional requirements

#### A. Agent lifecycle
- `POST /api/agents` creates an agent with `project`, generated/assigned `session_id`, `name`, `capabilities`, and metadata.
- `GET /api/agents` lists active or registered agents for a project.
- `POST /api/agents/{id}/heartbeat` updates `last_seen`.

#### B. Message routing and acknowledgements
- `POST /api/messages` accepts sender/recipients/thread/body.
- Message de-duplication by `(project, message_id)` overwrites thread/body safely for idempotent retries.
- `GET /api/inbox/{agent}` returns unread/new message streams by cursor.
- `GET /api/inbox/{agent}/counts` returns per-project inbox counts for operators.
- `POST /api/messages/{id}/ack` and `POST /api/messages/{id}/read` persist event log entries.
- `GET /api/threads` and `GET /api/threads/{thread_id}` reconstruct message order.

#### C. Real-time delivery
- `GET /ws/agents/{agent_id}` delivers pushed messages to subscribed clients.
- Socket identity uses agent id and optional project query for project-scoped delivery.

#### D. File reservation system
- `POST /api/reservations` creates a reservation with:
  - `agent`, `project`, `path`, and optional `glob_patterns`.
- conflict detection checks overlap between reserved path patterns using glob semantics.
- `shared=true` reservations can co-exist with other shared reservations; `shared=false` conflicts with other active reservations by default.
- `GET /api/reservations/check` evaluates whether a path is currently locked by another agent.
- `DELETE /api/reservations/{id}` releases ownership proactively.
- Background sweeper releases stale reservations based on heartbeat/lifecycle policy.

#### E. Domain APIs
- Create/read/update/delete for:
  - `/api/specs`
  - `/api/epics`
  - `/api/stories`
  - `/api/tasks`
  - `/api/insights`
  - `/api/sessions`
- Domain payloads preserve project scoping and event timestamps.

#### F. Operations and health
- `GET /health` returns service liveness for container checks.
- CLI `serve` options for host/port/db/socket support and startup.
- CLI `init` seeds project keys for non-interactive bootstrap.

### 5.2 Reliability requirements
- Storage failures must be retried with bounded backoff where appropriate.
- Repeated lock-related failures should trip circuit-breaking to avoid thundering herd behavior.
- Service should continue serving reads when transient DB failures are being protected by retry/circuit breaker policies.
- Sweeper and reconciliation paths must prevent stale reservation leaks.

### 5.3 Security requirements
- Localhost requests are accepted without bearer auth by default.
- Non-localhost requests require `Authorization: Bearer <key>`.
- When bearer auth is used, all mutating operations must carry project context and resolve to key-owned project.
- Multi-tenancy boundary is enforced by project in all persistent operations.

## 6) Architecture

### 6.1 Service composition
- Package entrypoint: `cmd/intermute`
  - Commands:
    - `serve` (runtime server)
    - `init` (bootstrap key file)
- Server wiring: `internal/server`
- HTTP transport/router handlers: `internal/http`
- WebSocket hub: `internal/ws`
- Storage abstraction + SQLite implementation: `internal/storage` + `internal/storage/sqlite`
- Auth middleware and bootstrap config: `internal/auth`
- Core domain + message types: `internal/core`
- File-glob conflict engine: `internal/glob`
- Agent identity naming: `internal/names`
- Client SDK: `client`

### 6.2 Persistence architecture
- Core message and event data modeled via event stream + message/index tables.
- Event sequence (`cursor`) supports incremental inbox and thread reads.
- Domain tables map to orchestration primitives.
- Reservation state includes heartbeat-aware expiry metadata.
- All timestamps stored in UTC and exposed as API ISO timestamps.

### 6.3 Technology
- Service module: `github.com/mistakeknot/intermute`.
- Language: Go (current module target Go 1.24).
- Primary DB: SQLite via `modernc.org/sqlite`.
- WebSocket: `nhooyr.io/websocket`.

## 7) Key workflows

### 7.1 Agent registration and registration drift
1. Client calls `POST /api/agents`.
2. Server validates auth/project.
3. New agent record is persisted.
4. Agents send periodic heartbeats (`POST /api/agents/{id}/heartbeat`).
5. Operators infer stale agents by last_seen and reservation cleanup.

### 7.2 Message and thread flow
1. Producer posts message to `/api/messages`.
2. Server persists message row and append-only event.
3. Thread index updated and per-recipient inbox entry created.
4. Connected websocket clients receive pushes to agent channel.
5. Consumers can fetch `/api/threads/{thread_id}` for full conversation reconstruction.

### 7.3 Reserve and release flow
1. Agent requests `POST /api/reservations` with path(s).
2. Overlap check resolves path conflicts:
   - same project only,
   - conflicting live locks fail unless sharing policy allows coexistence.
3. Reservation is returned with expiry and metadata.
4. Agent completes work and sends `DELETE /api/reservations/{id}`.
5. Stale reservations are eventually cleaned by sweeper.

### 7.4 Conflict check flow
1. Agent calls `/api/reservations/check` with candidate path.
2. Service returns matching active reservations + conflict reason.
3. Caller can defer work, switch paths, or request shared lock.

## 8) Acceptance criteria (MVP baseline)

- All listed endpoints are functional with JSON error/status behaviors consistent across handlers.
- New agents can register and consume real-time pushes.
- End-to-end message ingestion, inbox pagination, thread retrieval, ack/read events works.
- Reservation conflict rules are deterministic under concurrent requests.
- Sweeper and heartbeat-based liveness prevent abandoned locks.
- CLI bootstrap and local dev mode work without external dependencies.

## 9) Risks and assumptions

- Current auth model is intentionally coarse-grained and assumes trusted project boundaries.
- SQLite write concurrency and DB file lifecycle are handled by single-service deployment expectations.
- Long-term retention and compaction policies for events/messages are not yet standardized.
- This PRD assumes single-region deployment and no cross-region replication.
