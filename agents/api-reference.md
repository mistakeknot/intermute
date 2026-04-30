# API Reference

## Health

- `GET /health` -- Health check (unauthenticated, DomainRouter only)

## Agent Management

- `POST /api/agents` -- Register agent (auto-generates Culture ship name if none provided)
- `GET /api/agents?project=...&capability=...` -- List agents (filter by capability, comma-separated)
- `GET /api/agents/presence?repo=...&active_bead_id=...` -- Compact presence read model for agents working in a repo and/or on a Beads issue
- `POST /api/agents/{id}/heartbeat` -- Update last_seen
- `PATCH /api/agents/{id}/metadata` -- Merge metadata keys (PATCH semantics: incoming keys overwrite, absent keys preserved)
- `GET /api/agents/{id}/policy` -- Get contact policy
- `POST /api/agents/{id}/policy` -- Set contact policy (open, auto, contacts_only, block_all)

### Agent presence

`GET /api/agents/presence` exposes a compact read path over agent metadata so operators can ask "who is working on this bead/repo?" without scraping Discord or full agent records.

Query parameters:

- `project` -- Optional project scope. Bearer-key requests default to the key's project and cannot query another project.
- `repo` -- Optional exact repository path/name from metadata key `repo`.
- `active_bead_id` -- Optional exact Beads issue ID from metadata key `active_bead_id`.

Response:

```json
{
  "agents": [
    {
      "agent_id": "agent-123",
      "kind": "claude-code",
      "status": "active",
      "last_seen": "2026-04-29T20:58:00Z",
      "repo": "/home/mk/projects/Sylveste/core/intermute",
      "files": ["internal/http/handlers_agents.go"],
      "objective": "Add bead presence read model",
      "confidence": "reported",
      "active_bead_id": "sylveste-kgfi.2",
      "thread_id": "sylveste-kgfi.2"
    }
  ]
}
```

Notes:

- `kind`, `repo`, `files`, `objective`, `confidence`, `active_bead_id`, and `thread_id` are projected from agent metadata keys (`agent_kind`, `repo`, `files_touched`, `objective`, `active_bead_confidence`, `active_bead_id`, `thread_id`).
- `confidence` follows the producer metadata vocabulary: `reported`, `observed`, or `unknown`.
- `status` prefers producer metadata key `status` when present, then falls back to the agent record status.
- `files_touched` is expected to be a JSON-encoded string array; invalid or blank values project as an empty `files` array.
- Ambiguous candidate-only metadata is not guessed: an `active_bead_id` query only matches the singular `active_bead_id` metadata value, not `active_bead_candidates`.
- `thread_id` can use the Beads issue ID as the message/thread correlation handle.

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
