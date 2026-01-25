# Intermute

Coordination service for Autarch agents.

## MVP
- Agent registry + heartbeats
- Messaging (send/inbox/ack/read)
- REST as source of truth + WebSocket for real-time delivery

## Run
```
go run ./cmd/intermute
```

## Environment
- `INTERMUTE_URL` (client-side) e.g. `http://localhost:7338`
- `INTERMUTE_AGENT_NAME` (optional override)
- `INTERMUTE_PROJECT` (optional)

## API (MVP)
- `POST /api/agents`
- `POST /api/agents/{id}/heartbeat`
- `POST /api/messages`
- `GET /api/inbox/{agent}?since_cursor=...`
- `POST /api/messages/{id}/ack`
- `POST /api/messages/{id}/read`
- `WS /ws/agents/{id}`

## Test
```
go test ./...
```
