# intermute

Coordination service for Autarch agents.

## MVP
- Agent registry + heartbeats
- Messaging (send/inbox/ack/read)
- REST as source of truth + WebSocket for real-time delivery
- Project-scoped API keys with localhost bypass

## Run
```bash
go run ./cmd/intermute
```

### Initialize Auth Keys
```bash
intermute init --project autarch
```

## Auth Model
- Localhost requests are allowed without auth by default.
- Non-localhost requests require `Authorization: Bearer <key>`.
- When a bearer key is used, `project` is required on:
  - `POST /api/agents`
  - `POST /api/messages`

Keys are loaded from:
1) `INTERMUTE_KEYS_FILE`
2) `./intermute.keys.yaml`

See `intermute.keys.yaml.example` for the expected structure.

If the keys file is missing, the server bootstraps a dev key for project `dev`
on startup and logs the generated key and file path.

## Client Environment
- `INTERMUTE_URL` (client-side) e.g. `http://localhost:7338`
- `INTERMUTE_API_KEY` (optional; required for non-localhost)
- `INTERMUTE_PROJECT` (required when `INTERMUTE_API_KEY` is set)
- `INTERMUTE_AGENT_NAME` (optional override)

## API (MVP)
- `POST /api/agents` - Register an agent
- `GET /api/agents?project=...` - List agents (project required for API key auth)
- `POST /api/agents/{id}/heartbeat` - Update agent last_seen
- `POST /api/messages` - Send a message
- `GET /api/inbox/{agent}?since_cursor=...` - Fetch messages
- `POST /api/messages/{id}/ack` - Acknowledge a message
- `POST /api/messages/{id}/read` - Mark message as read
- `WS /ws/agents/{id}` - Real-time message delivery

### WebSocket Projects (local dev)
When running without auth, use the `project` query param to scope WS streams:

```bash
ws://localhost:7338/ws/agents/agent-a?project=autarch
```

## Test
```bash
go test ./...
```
