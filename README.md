# Intermute

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

## Client Environment
- `INTERMUTE_URL` (client-side) e.g. `http://localhost:7338`
- `INTERMUTE_API_KEY` (optional; required for non-localhost)
- `INTERMUTE_PROJECT` (required when `INTERMUTE_API_KEY` is set)
- `INTERMUTE_AGENT_NAME` (optional override)

## API (MVP)
- `POST /api/agents`
- `POST /api/agents/{id}/heartbeat`
- `POST /api/messages`
- `GET /api/inbox/{agent}?since_cursor=...`
- `POST /api/messages/{id}/ack`
- `POST /api/messages/{id}/read`
- `WS /ws/agents/{id}`

### WebSocket Projects (local dev)
When running without auth, use the `project` query param to scope WS streams:

```bash
ws://localhost:7338/ws/agents/agent-a?project=autarch
```

## Test
```bash
go test ./...
```
