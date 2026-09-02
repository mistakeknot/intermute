# intermute

Multi-agent coordination service: the backend that makes agents aware of each other.

## What this does

When multiple agents work on the same project, they need to know who's editing what, pass messages, and avoid stepping on each other's changes. intermute provides the coordination layer: agent registration with heartbeats, message routing, and file reservation tracking. It's a Go service backed by SQLite, exposing a REST API with WebSocket real-time delivery.

interlock (the Claude Code plugin) wraps intermute with MCP tools, hooks, and git enforcement. Most users interact with interlock; intermute is the engine underneath.

## Run

```bash
go run ./cmd/intermute    # Starts on :7338
```

Or as a systemd service for persistent operation.

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
go test ./...
```

## License

MIT
