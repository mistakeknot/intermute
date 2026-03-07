# Data Model

## Core Types

- `Agent`: id, session_id, name, project, capabilities[], metadata{}, status, contact_policy, last_seen, created_at
- `Message`: id, thread_id, project, from, to[], cc[], bcc[], subject, topic, body, metadata{}, attachments[], importance, ack_required, status, created_at, cursor
- `Event`: id, type, agent, project, message, created_at, cursor
- `Reservation`: id, agent_id, project, path_pattern, exclusive, reason, ttl, created_at, expires_at, released_at
- `RecipientStatus`: agent_id, kind (to/cc/bcc), read_at, ack_at
- `StaleAck`: message, kind, read_at, age_seconds

## Domain Types

- `Spec`: Product specification (draft -> research -> validated -> archived), version for optimistic locking
- `Epic`: Feature container within spec (open -> in_progress -> done)
- `Story`: User story with acceptance criteria (todo -> in_progress -> review -> done)
- `Task`: Execution unit assigned to agent (pending -> running -> blocked -> done)
- `Insight`: Research finding with score, source, category, URL
- `Session`: Agent execution context (running -> idle -> error)
- `CriticalUserJourney (CUJ)`: First-class CUJ entity with steps[], persona, priority (high/medium/low), entry_point, exit_point, success_criteria[], error_recovery[] (draft -> validated -> archived)
- `CUJStep`: order, action, expected, alternatives[]

## Contact Policy

Controls who can send messages to an agent.

- `open` -- accept from anyone (default)
- `auto` -- auto-allow agents with overlapping file reservations, explicit contacts, or thread participants
- `contacts_only` -- explicit whitelist + thread participant exception
- `block_all` -- reject everything
