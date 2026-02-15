# intermute Roadmap

## Current status

intermute is in active MVP-to-RC maturity:
- Core APIs are implemented and internally used for agent-to-agent messaging.
- WebSocket delivery and cursor-based inbox semantics are operational.
- File reservation and conflict detection exist with shared-lock behavior and background sweep cleanup.
- Domain APIs for specs/epics/stories/tasks/insights/sessions are available.
- Auth defaults and bootstrap flows are documented and usable in local development.

## Short-term (now → next quarter)

1. Production hardening of reliability boundaries
- Add richer startup diagnostics and startup-time migration verification output.
- Tighten retry/backoff and circuit breaker observability for storage paths.
- Improve DB-level retention tooling for events/messages to control file growth.

2. Operations and governance
- Add first-class metrics endpoints and structured logs for:
  - queue/ack/read latency,
  - reservation conflict rates,
  - websocket fanout pressure.
- Clarify admin runbooks for rotating keys and rotating `intermute.keys.yaml`.
- Add explicit policy docs for backup/restore and disaster recovery for `intermute.db`.

3. Feature completeness around conflict semantics
- Expand reservation responses to include canonical conflict details.
- Add richer shared-lock policy options for explicit read/write classes.
- Improve `/api/reservations/check` UX for automated CI/agent decision loops.

## Mid-term (next quarter → 2 quarters)

1. API ergonomics and consistency
- Add endpoint and payload versioning strategy.
- Standardize pagination metadata and response envelopes across handlers.
- Publish an API contract artifact for downstream SDK generators.

2. Ecosystem and developer productivity
- Generate and publish versioned Go client package updates with examples for all domain endpoints.
- Add local developer fixtures for smoke and compatibility tests.
- Document recommended deployment topology for multi-service orchestration.

3. Service maturity
- Evaluate pluggable storage backend abstraction beyond single SQLite runtime.
- Add integration safety checks for high-concurrency multi-project load.
- Introduce bounded queueing and explicit saturation behavior under pressure.

## Longer-term (3+ quarters)

1. Horizontal scalability planning
- Assess read replica and partitioned persistence strategies if agent count grows.
- Add sharding and namespace-aware shippers for very large environments.

2. Advanced coordination primitives
- Extend reservation to hierarchical path scoping and lock intent metadata.
- Add conflict prediction endpoints to preflight file plans before work starts.

3. Governance and compliance
- Expand audit artifacts for mutation events and project-scoped access patterns.
- Add signed-operation metadata for stronger traceability.

## Backlog candidates (not yet in active plans)

- Full REST OpenAPI v3 publication and generated docs portal.
- Web dashboard for realtime reservation/thread visualization.
- Fine-grained RBAC and per-capability authorization.
- Event retention policies with configurable compaction and archiving.
- Integration tests that benchmark high-contention reservation behavior.

## Success criteria for roadmap execution

- Fewer stale locks and predictable cleanup under failure scenarios.
- Lower conflict false-positive/false-negative rate for glob overlap checks.
- Measurable reliability gains after retry/circuit-breaker and sweeper hardening.
- Simpler on-call operation: operators can recover from outages using published runbooks.
- Stable public contract for clients embedding intermute as orchestration backbone.
