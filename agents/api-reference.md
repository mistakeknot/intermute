# API Reference

## Health

- `GET /health` -- Health check (unauthenticated, DomainRouter only)

## Agent Management

- `POST /api/agents` -- Register agent (auto-generates Culture ship name if none provided)
- `GET /api/agents?project=...&capability=...` -- List agents (filter by capability, comma-separated)
- `POST /api/agents/{id}/heartbeat` -- Update last_seen
- `PATCH /api/agents/{id}/metadata` -- Merge metadata keys (PATCH semantics: incoming keys overwrite, absent keys preserved)
- `GET /api/agents/{id}/policy` -- Get contact policy
- `POST /api/agents/{id}/policy` -- Set contact policy (open, auto, contacts_only, block_all)

## Messaging

- `POST /api/messages` -- Send message (supports to, cc, bcc, subject, topic, importance, ack_required)
- `GET /api/inbox/{agent}?since_cursor=...&limit=...` -- Fetch inbox
- `GET /api/inbox/{agent}/counts` -- Inbox total/unread counts
- `GET /api/inbox/{agent}/stale-acks?ttl_seconds=...&limit=...` -- Ack-required messages past TTL
- `POST /api/messages/{id}/ack` -- Acknowledge message (body: `{"agent": "..."}`)
- `POST /api/messages/{id}/read` -- Mark as read (body: `{"agent": "..."}`)
- `POST /api/broadcast` -- Broadcast to all project agents (rate-limited: 10/min/sender)
- `GET /api/topics/{project}/{topic}?since_cursor=...&limit=...` -- Topic-based message discovery

## Threads

- `GET /api/threads?agent=...&cursor=...&limit=...` -- List threads (DESC by last_cursor, default limit 50)
- `GET /api/threads/{thread_id}?cursor=...` -- Fetch thread messages

## File Reservations

- `POST /api/reservations` -- Create reservation (glob pattern, exclusive/shared, TTL in minutes)
- `GET /api/reservations?project=...` or `?agent=...` -- List active reservations
- `GET /api/reservations/check?project=...&pattern=...&exclusive=...` -- Check conflicts without creating
- `DELETE /api/reservations/{id}` -- Release reservation (agent must match)

## Domain (specs/epics/stories/tasks/insights/sessions/cujs)

- `GET /api/{entity}?project=...` -- List entities (supports status/filter params per entity)
- `POST /api/{entity}` -- Create entity
- `GET /api/{entity}/{id}?project=...` -- Get entity
- `PUT /api/{entity}/{id}` -- Update entity (optimistic locking via version field)
- `DELETE /api/{entity}/{id}?project=...` -- Delete entity

## WebSocket

- `WS /ws/agents/{agent_id}?project=...` -- Real-time message stream
