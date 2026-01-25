# AGENTS.md

## Overview

Intermute is a real-time coordination and messaging service for Autarch agents. It handles agent lifecycle (registration, heartbeats), project-scoped messaging with threading, and event sourcing of domain entities (specs, epics, stories, tasks). Acts as the central hub for multi-agent orchestration with REST + WebSocket delivery.

## Quick Reference

| Item | Value |
|------|-------|
| Port | 7338 |
| Start | `go run ./cmd/intermute` |
| Tests | `go test ./...` |
| Database | SQLite (intermute.db) |
| Auth config | intermute.keys.yaml (`INTERMUTE_KEYS_FILE` env var) |

## Architecture

**Stack:** Go 1.24, SQLite (modernc.org), nhooyr WebSocket

**Request Flow:**
1. HTTP request → auth middleware → handler → service → store
2. Handler serializes response (JSON) or error code
3. If broadcaster set, service notifies WebSocket hub
4. Hub broadcasts to all connected clients for (project, agent)

**Database Design:**
- **events** - Append-only log; cursor=PK, type=(message.created|ack|read|heartbeat)
- **messages** - Deduplicated by (project, message_id) composite key
- **inbox_index** - Materialized view; agent → [(cursor, message_id)] ordered by cursor
- **thread_index** - Tracks (project, thread_id, agent) → (last_cursor, message_count)
- **agents** - Agent registry with capabilities and metadata
- **Domain tables** - specs, epics, stories, tasks, insights, sessions

**Authentication:**
- Localhost requests: allowed by default (AllowLocalhostWithoutAuth=true)
- API key requests: require `Authorization: Bearer <key>` + `project` param
- Keyring loaded from intermute.keys.yaml; maps key → project

## Directory Structure

```
cmd/intermute/       Entry point; wires store, auth, WebSocket hub, HTTP service
client/              Go SDK for agents to interact with Intermute
internal/
  auth/              Keyring loading and HTTP middleware for bearer token validation
  cli/               CLI helpers (key file initialization)
  core/              Domain types: Message, Agent, Event, Spec, Epic, Story, Task, Insight, Session
  http/              REST handlers for agents, messages, threads, and domain entities
  names/             Culture ship-style name generator for agents
  server/            Server wiring and startup
  storage/           Store interface with InMemory implementation
    sqlite/          SQLite implementation with schema and migrations
  ws/                WebSocket hub for real-time message delivery
```

## Key Files

| File | Purpose |
|------|---------|
| `cmd/intermute/main.go` | Entry point; wires components |
| `internal/http/router.go` | HTTP multiplexer; mounts /api/* and /ws/* |
| `internal/http/handlers_*.go` | REST handlers for agents, messages, threads, domain |
| `internal/storage/storage.go` | Store interface + InMemory implementation |
| `internal/storage/sqlite/sqlite.go` | SQLite implementation with migrations |
| `internal/storage/sqlite/schema.sql` | DDL for all tables and indexes |
| `internal/ws/gateway.go` | WebSocket hub for real-time delivery |
| `internal/core/models.go` | Message, Agent, Event types |
| `internal/core/domain.go` | Spec, Epic, Story, Task, Insight, Session types |
| `client/client.go` | Go SDK for agent communication |

## API Endpoints

**Agent Management:**
- `POST /api/agents` - Register agent
- `GET /api/agents?project=...` - List agents
- `POST /api/agents/{id}/heartbeat` - Update last_seen

**Messaging:**
- `POST /api/messages` - Send message
- `GET /api/inbox/{agent}?since_cursor=...` - Fetch inbox
- `POST /api/messages/{id}/ack` - Acknowledge message
- `POST /api/messages/{id}/read` - Mark as read

**Threads:**
- `GET /api/threads?agent=...&cursor=...` - List threads (DESC by last_cursor)
- `GET /api/threads/{thread_id}?cursor=...` - Fetch thread messages

**Domain (specs/epics/stories/tasks/insights/sessions):**
- `GET /api/{entity}?project=...` - List entities
- `POST /api/{entity}` - Create entity
- `GET /api/{entity}/{id}?project=...` - Get entity
- `PUT /api/{entity}/{id}` - Update entity
- `DELETE /api/{entity}/{id}?project=...` - Delete entity

**WebSocket:**
- `WS /ws/agents/{agent_id}?project=...` - Real-time message stream

## Data Model

**Core Types:**
- `Agent`: id, session_id, name, project, capabilities[], metadata{}, status, created_at, last_seen
- `Message`: id, thread_id, project, from, to[], body, created_at, cursor
- `Event`: id, type, agent, project, message, created_at, cursor

**Domain Types:**
- `Spec`: Product specification (draft → research → validated → archived)
- `Epic`: Feature container within spec (open → in_progress → done)
- `Story`: User story with acceptance criteria (todo → in_progress → review → done)
- `Task`: Execution unit assigned to agent (pending → running → blocked → done)
- `Insight`: Research finding with scoring
- `Session`: Agent execution context (running → idle → error)

## Conventions

**Naming:**
- Handlers: `handleXxx` (e.g., handleSendMessage)
- Requests: `xxxRequest` (e.g., sendMessageRequest)
- Responses: `xxxResponse` (e.g., sendMessageResponse)
- JSON structs: `xxxJSON` for API serialization

**Database:**
- Composite PKs: (project, id) for multi-tenancy
- Timestamps: time.Time in Go, RFC3339Nano in SQLite, ISO8601 in JSON
- JSON marshaling for complex types (capabilities, metadata, to_json)

**Error Handling:**
- HTTP handlers return status codes (no JSON error bodies)
- Store methods wrap errors with context (`fmt.Errorf("operation: %w", err)`)

## Commands

```bash
# Run server
go run ./cmd/intermute

# Run tests
go test ./...

# Run with custom keys file
INTERMUTE_KEYS_FILE=/path/to/keys.yaml go run ./cmd/intermute

# Build binary
go build -o intermute ./cmd/intermute
```

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

## Gotchas

1. **Project scoping** - All queries must filter by project; composite PKs enforce at schema level
2. **Cursor semantics** - `since_cursor` uses `>` not `>=`; first fetch should use cursor=0
3. **Message deduplication** - Re-posting same message_id overwrites thread_id and body
4. **Thread indexing** - Only messages with thread_id are indexed; non-threaded messages excluded
5. **Domain handlers** - Specs, epics, stories, tasks, insights, sessions fully implemented
6. **Localhost bypass** - Applies to 127.0.0.1 only; LAN origins require API key
7. **No ack persistence** - Ack/read events logged but no status columns updated

## Testing

```bash
# All tests
go test ./...

# Verbose
go test -v ./...

# Coverage
go test -cover ./...

# Single package
go test ./internal/storage/sqlite
```

Test patterns:
- `sqlite_test.go`: In-memory SQLite with cursor/thread/migration tests
- `handlers_*_test.go`: httptest.Server integration tests
- `client_test.go`: Mock server for SDK validation
