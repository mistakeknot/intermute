# Architecture

## Stack

Go 1.24 (toolchain `go1.24.12`), SQLite (modernc.org/sqlite), nhooyr WebSocket, Cobra CLI

## Request Flow

1. HTTP request -> auth middleware -> handler -> service -> store (ResilientStore -> CircuitBreaker -> SQLite)
2. Handler serializes response (JSON) or error code
3. If broadcaster set, service notifies WebSocket hub
4. Hub broadcasts to all connected clients for (project, agent)

## Database Design (16 tables)

- **events** -- Append-only log; cursor=PK, type includes message.*, agent.*, spec.*, epic.*, story.*, task.*, insight.*, session.*, reservation.*, cuj.*
- **messages** -- Deduplicated by (project, message_id) composite key; supports cc, bcc, subject, topic, importance, ack_required
- **message_recipients** -- Per-recipient read/ack tracking: (project, message_id, agent_id) -> read_at, ack_at
- **inbox_index** -- Materialized view; agent -> [(cursor, message_id)] ordered by cursor
- **thread_index** -- Tracks (project, thread_id, agent) -> (last_cursor, message_count, last_message_*)
- **agents** -- Agent registry with capabilities, metadata, contact_policy, session_id
- **agent_contacts** -- Explicit contact whitelist: (agent_id, contact_agent_id)
- **file_reservations** -- Glob-pattern file locks with TTL and exclusive/shared modes
- **Domain tables** -- specs, epics, stories, tasks, insights, sessions (all with composite PK (project, id) and version for optimistic locking)
- **cujs** -- Critical User Journeys with steps, persona, priority, success criteria
- **cuj_feature_links** -- Many-to-many CUJ-to-feature association

## Authentication

- Localhost requests: allowed by default (AllowLocalhostWithoutAuth=true)
- Non-localhost requests: require `Authorization: Bearer <key>`
- When a bearer key is used, `project` is required on: `POST /api/agents` and `POST /api/messages`
- Keyring loaded from `INTERMUTE_KEYS_FILE` (fallback `./intermute.keys.yaml`); maps key -> project
- `intermute init --project <name>` creates a key entry in the keys file
- If the keys file is missing, the server bootstraps a dev key for project `dev` on startup

## Directory Structure

```
cmd/intermute/    Entry point, CLI flags, component wiring
client/           Go SDK (messaging, domain CRUD, WebSocket)
internal/         auth/, core/ (domain types), glob/ (NFA overlap), http/ (handlers+routers), storage/ (Store interfaces + sqlite/), ws/ (WebSocket hub), server/ (dual-listen), names/ (ship name gen)
pkg/embedded/     Embeddable server for in-process use (Autarch uses this)
```
