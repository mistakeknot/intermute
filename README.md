# intermute

Multi-agent coordination service: the backend that makes agents aware of each other.

## What this does

When multiple agents work on the same project, they need to know who's editing what, pass messages, and avoid stepping on each other's changes. intermute is the coordination layer: agent registration with heartbeats, file reservations with conflict detection, message routing, and window identity tracking. It's a Go service backed by SQLite, exposing a REST API with WebSocket real-time delivery.

interlock (the Claude Code plugin) wraps intermute with MCP tools, hooks, and git enforcement. Most users interact with interlock; intermute is the engine underneath, and it also works standalone behind any MCP client or a plain HTTP client.

## Why intermute

- **Glob-overlap reservations with bounded cost.** File reservations use glob patterns (`src/**/*.go`), and conflict detection compares every pattern pair. `internal/glob` caps pattern complexity (`MaxTokens`, `MaxWildcards`) and is fuzz-tested so every comparison stays cheap regardless of input.
- **A resilience layer with a named wedge test.** Reservation writes and the background sweeper used to contend for SQLite's rollback-journal lock and wedge the server with `SQLITE_BUSY`. WAL mode plus a busy timeout fixed it; `TestNewStoreAvoidsSQLITEBUSYUnderConcurrentWrites` reproduces that concurrency pattern so it stays fixed.
- **Wake a peer over tmux, not just polling.** With `transport: "live"`, a message is pasted directly into the recipient's tmux pane when it's at a prompt, instead of waiting on the next inbox poll.

## Install

```bash
go install github.com/mistakeknot/intermute/cmd/intermute@latest
intermute serve                                    # starts on :7338
intermute serve --db /path/to/intermute.db --port 7338
```

## Quick start

`examples/two-agents.sh` runs the walkthrough below end to end against a fresh server on a temp database, and cleans up after itself:

```bash
./examples/two-agents.sh
```

What it does:

1. Registers two agents, alpha and beta.
2. alpha reserves `src/**/*.go` exclusively.
3. beta calls `/api/reservations/check` on the same pattern and sees the conflict.
4. beta messages alpha asking for the file.
5. alpha reads its inbox, then releases the reservation.
6. beta reserves the now-free pattern.

## API

Full reference with request/response shapes: [docs/api.md](docs/api.md).

| Route family | What it does |
|----------|-------------|
| `GET /health` | Health check, including storage liveness |
| `POST /api/agents`, `GET /api/agents` | Register / list agents in a project |
| `GET /api/agents/presence` | Compact "who's working on what" read model |
| `POST /api/agents/{id}/heartbeat`, `PATCH /api/agents/{id}/metadata`, `.../policy` | Keep-alive, metadata merge, contact policy |
| `POST /api/messages` | Send a message (to/cc/bcc, topic, ack_required) |
| `POST /api/messages/{id}/ack`, `.../read` | Acknowledge / mark a message read |
| `GET /api/inbox/{agent}` | Fetch inbox (cursor-based pagination) |
| `GET /api/inbox/{agent}/counts`, `.../stale-acks` | Inbox counts / overdue acks |
| `GET /api/threads` | List and fetch message threads |
| `GET /api/topics/{project}/{topic}` | Topic-based message discovery |
| `POST /api/broadcast` | Broadcast to all agents in a project |
| `POST /api/reservations`, `GET /api/reservations` | Create / list file reservations |
| `GET /api/reservations/check` | Check for conflicts without reserving |
| `DELETE /api/reservations/{id}` | Release a reservation |
| `GET /api/windows`, `POST /api/windows`, `DELETE /api/windows/{id}` | tmux window identity: list, upsert, expire |
| `/api/specs`, `/api/epics`, `/api/stories`, `/api/tasks`, `/api/insights`, `/api/sessions`, `/api/cujs` | Domain entity CRUD (list/create/get/update/delete) |
| `WS /ws/agents/{id}` | Real-time message delivery |

## Auth

Localhost requests are allowed without authentication: useful for single-machine multi-agent setups. Non-localhost requires `Authorization: Bearer <key>` with a project scope.

Keys are loaded from `INTERMUTE_KEYS_FILE` or `./intermute.keys.yaml`. If neither exists, the server bootstraps a dev key on startup.

## Client environment

```bash
INTERMUTE_URL=http://localhost:7338
INTERMUTE_API_KEY=...        # Required for non-localhost
INTERMUTE_PROJECT=...        # Required when API key is set
INTERMUTE_AGENT_NAME=...     # Optional override
```

## Testing

```bash
go build ./... && go vet ./... && go test -race ./...
```

## License

MIT
