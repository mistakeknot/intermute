# AGENTS.md

## Overview

Intermute is the L1 (core) multi-agent coordination and messaging service for the Demarch platform. It handles agent lifecycle (registration, heartbeats, contact policies), project-scoped messaging with threading and topic discovery, file reservations with glob-based conflict detection, and event sourcing of domain entities (specs, epics, stories, tasks, insights, sessions, CUJs). Acts as the central hub for multi-agent orchestration with REST + WebSocket + optional Unix socket delivery.

## Quick Reference

| Item | Value |
|------|-------|
| Module | `github.com/mistakeknot/intermute` |
| Layer | L1 (core) |
| Port | 7338 |
| Start | `go run ./cmd/intermute serve` |
| Tests | `go test ./...` |
| Database | SQLite (intermute.db) |
| Auth config | intermute.keys.yaml (`INTERMUTE_KEYS_FILE` env var) |

## Architecture

**Stack:** Go 1.24 (toolchain `go1.24.12`), SQLite (modernc.org/sqlite), nhooyr WebSocket, Cobra CLI

**Request Flow:**
1. HTTP request → auth middleware → handler → service → store (ResilientStore → CircuitBreaker → SQLite)
2. Handler serializes response (JSON) or error code
3. If broadcaster set, service notifies WebSocket hub
4. Hub broadcasts to all connected clients for (project, agent)

**Database Design (16 tables):**
- **events** — Append-only log; cursor=PK, type includes message.*, agent.*, spec.*, epic.*, story.*, task.*, insight.*, session.*, reservation.*, cuj.*
- **messages** — Deduplicated by (project, message_id) composite key; supports cc, bcc, subject, topic, importance, ack_required
- **message_recipients** — Per-recipient read/ack tracking: (project, message_id, agent_id) → read_at, ack_at
- **inbox_index** — Materialized view; agent → [(cursor, message_id)] ordered by cursor
- **thread_index** — Tracks (project, thread_id, agent) → (last_cursor, message_count, last_message_*)
- **agents** — Agent registry with capabilities, metadata, contact_policy, session_id
- **agent_contacts** — Explicit contact whitelist: (agent_id, contact_agent_id)
- **file_reservations** — Glob-pattern file locks with TTL and exclusive/shared modes
- **Domain tables** — specs, epics, stories, tasks, insights, sessions (all with composite PK (project, id) and version for optimistic locking)
- **cujs** — Critical User Journeys with steps, persona, priority, success criteria
- **cuj_feature_links** — Many-to-many CUJ-to-feature association

**Authentication:**
- Localhost requests: allowed by default (AllowLocalhostWithoutAuth=true)
- Non-localhost requests: require `Authorization: Bearer <key>`
- When a bearer key is used, `project` is required on: `POST /api/agents` and `POST /api/messages`
- Keyring loaded from `INTERMUTE_KEYS_FILE` (fallback `./intermute.keys.yaml`); maps key → project
- `intermute init --project <name>` creates a key entry in the keys file
- If the keys file is missing, the server bootstraps a dev key for project `dev` on startup

## Directory Structure

```
cmd/intermute/       Entry point; wires store, auth, WebSocket hub, sweeper, HTTP service
client/              Go SDK for agents (messaging, domain CRUD, WebSocket subscriptions)
internal/
  auth/              Keyring loading, HTTP middleware, dev key bootstrapping
  cli/               CLI helpers (key file initialization)
  core/              Domain types: Message, Agent, Event, Reservation, Spec, Epic, Story, Task, Insight, Session, CUJ, ContactPolicy
  glob/              NFA glob overlap detection for reservation conflict checking
  http/              REST handlers, routers (NewRouter, NewDomainRouter), DomainService
  names/             Culture ship-style name generator for agents
  server/            HTTP + Unix socket dual-listen server
  storage/           Store and DomainStore interfaces, InMemory implementation
    sqlite/          SQLite implementation: schema, migrations, ResilientStore, CircuitBreaker, Sweeper, CoordinationBridge, query logger, retry
  ws/                WebSocket hub for real-time message delivery
pkg/embedded/        Embeddable server for in-process use (New, NewWithAuth)
scripts/             Utilities (check-file-conflict.sh, session-status.sh, worktree-*.sh)
```

## Key Files

| File | Purpose |
|------|---------|
| `cmd/intermute/main.go` | Entry point; wires all components, CLI flags |
| `internal/http/router.go` | Messaging-only router (NewRouter) |
| `internal/http/router_domain.go` | Full router with messaging + domain + health (NewDomainRouter) |
| `internal/http/service.go` | Service struct with broadcast rate limiter |
| `internal/http/handlers_*.go` | REST handlers for agents, messages, threads, reservations, domain, health |
| `internal/storage/storage.go` | Store interface + InMemory implementation |
| `internal/storage/domain.go` | DomainStore interface (extends Store with CRUD for all entities) |
| `internal/storage/sqlite/schema.sql` | DDL for all 16 tables and indexes |
| `internal/storage/sqlite/resilient.go` | ResilientStore: circuit breaker + retry wrapper |
| `internal/storage/sqlite/sweeper.go` | Background expired reservation cleanup |
| `internal/storage/sqlite/coordination_bridge.go` | Dual-write mirror to Intercore coordination_locks |
| `internal/core/models.go` | Message, Agent, Event, Reservation, RecipientStatus, StaleAck types |
| `internal/core/domain.go` | Spec, Epic, Story, Task, Insight, Session, CUJ types; ContactPolicy; sentinel errors |
| `internal/glob/overlap.go` | NFA glob pattern overlap detection (DoS guard: max 50 tokens, 10 wildcards) |
| `pkg/embedded/server.go` | Embeddable server (Config, New, NewWithAuth, Start, Stop, URL, Store) |
| `client/client.go` | Go SDK: Register, Send, Inbox, Heartbeat, Reservations |
| `client/domain.go` | Go SDK: domain entity CRUD (specs, epics, stories, tasks, insights, sessions) |
| `client/websocket.go` | Go SDK: WebSocket subscription with auto-reconnect |

## API Endpoints

**Health:**
- `GET /health` — Health check (unauthenticated, DomainRouter only)

**Agent Management:**
- `POST /api/agents` — Register agent (auto-generates Culture ship name if none provided)
- `GET /api/agents?project=...&capability=...` — List agents (filter by capability, comma-separated)
- `POST /api/agents/{id}/heartbeat` — Update last_seen
- `PATCH /api/agents/{id}/metadata` — Merge metadata keys (PATCH semantics: incoming keys overwrite, absent keys preserved)
- `GET /api/agents/{id}/policy` — Get contact policy
- `POST /api/agents/{id}/policy` — Set contact policy (open, auto, contacts_only, block_all)

**Messaging:**
- `POST /api/messages` — Send message (supports to, cc, bcc, subject, topic, importance, ack_required)
- `GET /api/inbox/{agent}?since_cursor=...&limit=...` — Fetch inbox
- `GET /api/inbox/{agent}/counts` — Inbox total/unread counts
- `GET /api/inbox/{agent}/stale-acks?ttl_seconds=...&limit=...` — Ack-required messages past TTL
- `POST /api/messages/{id}/ack` — Acknowledge message (body: `{"agent": "..."}`)
- `POST /api/messages/{id}/read` — Mark as read (body: `{"agent": "..."}`)
- `POST /api/broadcast` — Broadcast to all project agents (rate-limited: 10/min/sender)
- `GET /api/topics/{project}/{topic}?since_cursor=...&limit=...` — Topic-based message discovery

**Threads:**
- `GET /api/threads?agent=...&cursor=...&limit=...` — List threads (DESC by last_cursor, default limit 50)
- `GET /api/threads/{thread_id}?cursor=...` — Fetch thread messages

**File Reservations:**
- `POST /api/reservations` — Create reservation (glob pattern, exclusive/shared, TTL in minutes)
- `GET /api/reservations?project=...` or `?agent=...` — List active reservations
- `GET /api/reservations/check?project=...&pattern=...&exclusive=...` — Check conflicts without creating
- `DELETE /api/reservations/{id}` — Release reservation (agent must match)

**Domain (specs/epics/stories/tasks/insights/sessions/cujs):**
- `GET /api/{entity}?project=...` — List entities (supports status/filter params per entity)
- `POST /api/{entity}` — Create entity
- `GET /api/{entity}/{id}?project=...` — Get entity
- `PUT /api/{entity}/{id}` — Update entity (optimistic locking via version field)
- `DELETE /api/{entity}/{id}?project=...` — Delete entity

**WebSocket:**
- `WS /ws/agents/{agent_id}?project=...` — Real-time message stream

## Data Model

**Core Types:**
- `Agent`: id, session_id, name, project, capabilities[], metadata{}, status, contact_policy, last_seen, created_at
- `Message`: id, thread_id, project, from, to[], cc[], bcc[], subject, topic, body, metadata{}, attachments[], importance, ack_required, status, created_at, cursor
- `Event`: id, type, agent, project, message, created_at, cursor
- `Reservation`: id, agent_id, project, path_pattern, exclusive, reason, ttl, created_at, expires_at, released_at
- `RecipientStatus`: agent_id, kind (to/cc/bcc), read_at, ack_at
- `StaleAck`: message, kind, read_at, age_seconds

**Domain Types:**
- `Spec`: Product specification (draft → research → validated → archived), version for optimistic locking
- `Epic`: Feature container within spec (open → in_progress → done)
- `Story`: User story with acceptance criteria (todo → in_progress → review → done)
- `Task`: Execution unit assigned to agent (pending → running → blocked → done)
- `Insight`: Research finding with score, source, category, URL
- `Session`: Agent execution context (running → idle → error)
- `CriticalUserJourney (CUJ)`: First-class CUJ entity with steps[], persona, priority (high/medium/low), entry_point, exit_point, success_criteria[], error_recovery[] (draft → validated → archived)
- `CUJStep`: order, action, expected, alternatives[]

**Contact Policy:** Controls who can send messages to an agent.
- `open` — accept from anyone (default)
- `auto` — auto-allow agents with overlapping file reservations, explicit contacts, or thread participants
- `contacts_only` — explicit whitelist + thread participant exception
- `block_all` — reject everything

## Resilience

- **ResilientStore** wraps every Store/DomainStore method with CircuitBreaker + RetryOnDBLock
- **CircuitBreaker** (threshold=5 failures, reset timeout=30s): closed → open → half-open
- **RetryOnDBLock**: retries transient SQLite "database is locked" errors
- **QueryLogger**: logs slow queries above 100ms threshold
- **Sweeper**: background goroutine (60s interval) cleaning expired reservations from inactive agents (5min heartbeat grace); emits `reservation.expired` events

## Intercore Coordination Bridge

Optional dual-write mode mirrors file reservations to Intercore's `coordination_locks` table.
- Enable: `--coordination-dual-write` flag on `serve`
- Auto-discovers `intercore.db` or use `--intercore-db` path
- Used during migration phase; Intermute remains the primary reservation store

## Conventions

**Naming:**
- Handlers: `handleXxx` (e.g., handleSendMessage)
- Requests: `xxxRequest` (e.g., sendMessageRequest)
- Responses: `xxxResponse` (e.g., sendMessageResponse)
- JSON structs: `xxxJSON` for API serialization

**Database:**
- Composite PKs: (project, id) for multi-tenancy
- Timestamps: time.Time in Go, RFC3339Nano in SQLite, ISO8601 in JSON
- JSON marshaling for complex types (capabilities, metadata, to_json, steps_json, etc.)
- Optimistic locking: domain entities use `version` field; `ErrConcurrentModification` on conflict

**Error Handling:**
- HTTP handlers return status codes with JSON error bodies for structured errors (conflicts, policy denied)
- Store methods wrap errors with context (`fmt.Errorf("operation: %w", err)`)
- Sentinel errors: `ErrNotFound`, `ErrConcurrentModification`, `ErrActiveSessionConflict`, `ErrPolicyDenied`

## Commands

```bash
# Run server
go run ./cmd/intermute serve

# Run with all options
go run ./cmd/intermute serve --host 0.0.0.0 --port 7338 --db ./intermute.db \
  --socket /var/run/intermute.sock --coordination-dual-write

# Initialize auth keys for a project
go run ./cmd/intermute init --project autarch --keys-file ./intermute.keys.yaml

# Run tests
go test ./...

# Build binary
go build -o intermute ./cmd/intermute
```

### Server CLI flags

`intermute serve` supports:
- `--host` (default: `127.0.0.1`)
- `--port` (default: `7338`)
- `--db` (default: `intermute.db`)
- `--socket` (default: empty; Unix domain socket path)
- `--coordination-dual-write` (default: false; mirror to Intercore)
- `--intercore-db` (default: empty; auto-discovered if omitted)

## Authentication Model

```yaml
# intermute.keys.yaml
default_policy:
  allow_localhost_without_auth: true
projects:
  project-a:
    keys:
      - secret-key-1
  project-b:
    keys:
      - secret-key-2
```

When using API key auth, POST operations must include `project` field matching the key's project.

## Client Environment

- `INTERMUTE_URL` (client-side) e.g. `http://localhost:7338`
- `INTERMUTE_API_KEY` (optional; required for non-localhost)
- `INTERMUTE_PROJECT` (required when `INTERMUTE_API_KEY` is set)
- `INTERMUTE_AGENT_NAME` (optional override)

## Gotchas

1. **Project scoping** — All queries must filter by project; composite PKs enforce at schema level
2. **Cursor semantics** — `since_cursor` uses `>` not `>=`; first fetch should use cursor=0
3. **Message deduplication** — Re-posting same message_id overwrites thread_id and body
4. **Thread indexing** — Only messages with thread_id are indexed; non-threaded messages excluded
5. **Localhost bypass** — Applies to 127.0.0.1 only; LAN origins require API key
6. **Contact policy on send** — Recipients filtered by policy; partial delivery possible (some denied, some allowed); response includes `denied` list
7. **Session stale threshold** — 5 minutes; after this, session_id can be reused by new agent registration
8. **Broadcast rate limit** — 10 per minute per (project, sender); returns 429 with Retry-After header
9. **Topic lowercasing** — Topics are lowercased at write time for case-insensitive discovery
10. **Optimistic locking** — Domain entity updates check version; `ErrConcurrentModification` on conflict

## Downstream Dependencies

| Consumer | Uses | Monorepo Location |
|----------|------|-------------------|
| Autarch | `pkg/embedded/`, domain API, core types | `apps/autarch` |

**After pushing changes to:**
- `pkg/embedded/` — Autarch embeds this server
- `internal/core/domain.go` — Domain types used by Autarch client
- `internal/http/handlers_domain.go` — API endpoints Autarch calls
- `internal/storage/sqlite/schema.sql` — Schema changes

**Notify downstream:**
```bash
cd /home/mk/projects/Demarch/apps/autarch
go get github.com/mistakeknot/intermute@latest
go build ./cmd/autarch/  # Verify it compiles
```

## Testing

```bash
go test ./...            # All tests
go test -v ./...         # Verbose
go test -cover ./...     # Coverage
go test -race ./...      # Race detection
go test ./internal/storage/sqlite  # Single package
```

**Test patterns:**
- `sqlite_test.go`: In-memory SQLite with cursor/thread/migration tests
- `handlers_*_test.go`: httptest.Server integration tests
- `client_test.go` / `domain_test.go`: Mock server for SDK validation
- `circuitbreaker_test.go`, `retry_test.go`: Resilience layer unit tests
- `coordination_bridge_test.go`: Dual-write bridge tests
- `contact_policy_test.go`, `topic_test.go`: Feature-specific tests
- `race_test.go`: Concurrent access tests
- `smoke_test.go`: End-to-end integration smoke test
- Total: 158 test functions across 27 test files
- Auth bypass in tests: `httptest.NewServer` binds to 127.0.0.1, so localhost auth bypass applies

## Operational Notes

### Router Variants
- `NewRouter` (messaging + reservations): used for lightweight messaging-only deployments
- `NewDomainRouter` (messaging + reservations + domain + health): used by `serve` command and `pkg/embedded`
- Both routers include `/api/reservations` routes
- `DomainService` embeds `*Service`, so one struct handles both messaging + domain ops

### SQLite Gotchas (Go)
- `":memory:"` with `sql.Open("sqlite", ...)` creates separate DB per connection in pool
- Concurrent tests need file-backed DB with `db.SetMaxOpenConns(1)` to avoid SQLITE_BUSY
- PRAGMAs (WAL, busy_timeout) only apply to connection they're run on
- Production uses `ResilientStore` which wraps with circuit breaker + retry for transient errors
