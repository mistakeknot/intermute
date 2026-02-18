# intermute

Multi-agent coordination service — the backend that makes agents aware of each other.

## What This Does

When multiple agents work on the same project, they need to know who's editing what, pass messages, and avoid stepping on each other's changes. intermute provides the coordination layer: agent registration with heartbeats, message routing, and file reservation tracking. It's a Go service backed by SQLite, exposing a REST API with WebSocket real-time delivery.

interlock (the Claude Code plugin) wraps intermute with MCP tools, hooks, and git enforcement. Most users interact with interlock; intermute is the engine underneath.

## Run

```bash
go run ./cmd/intermute    # Starts on :7338
```

Or as a systemd service for persistent operation.

## API

| Endpoint | What it does |
|----------|-------------|
| `POST /api/agents` | Register an agent |
| `GET /api/agents?project=...` | List agents in a project |
| `POST /api/agents/{id}/heartbeat` | Keep-alive signal |
| `POST /api/messages` | Send a message |
| `GET /api/inbox/{agent}?since_cursor=...` | Fetch messages (cursor-based pagination) |
| `POST /api/messages/{id}/ack` | Acknowledge receipt |
| `POST /api/messages/{id}/read` | Mark as read |
| `WS /ws/agents/{id}` | Real-time message delivery |

## Auth

Localhost requests are allowed without authentication — useful for single-machine multi-agent setups. Non-localhost requires `Authorization: Bearer <key>` with a project scope.

Keys are loaded from `INTERMUTE_KEYS_FILE` or `./intermute.keys.yaml`. If neither exists, the server bootstraps a dev key on startup.

## Client Environment

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
