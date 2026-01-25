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

## Test
```
go test ./...
```
