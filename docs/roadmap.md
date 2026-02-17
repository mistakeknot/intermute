# intermute Roadmap

## Current status

intermute is in active MVP-to-RC maturity:
- Core APIs are implemented and internally used for agent-to-agent messaging.
- WebSocket delivery and cursor-based inbox semantics are operational.
- File reservation and conflict detection exist with shared-lock behavior and background sweep cleanup.
- Domain APIs for specs/epics/stories/tasks/insights/sessions are available.
- Auth defaults and bootstrap flows are documented and usable in local development.

## Now (short-term hardening)

1. Production hardening of reliability boundaries
- [IMT-N1] **Startup diagnostics and migration verification** — add richer startup output and migration checks.
- [IMT-N2] **Retry and backoff hardening** — tighten circuit breaker telemetry on storage path failures.
- [IMT-N3] **DB retention controls** — operational tooling for bounded event/message growth.

2. Operations and governance
- [IMT-N4] **Metrics and structured logs** — expose queue/ack/read latency and conflict telemetry.
- [IMT-N5] **Operations runbooks** — codify key rotation, backup, and disaster-recovery workflows.
- [IMT-N6] **Observability policies** — document logging and incident guidance for production operators.

3. Feature completeness around conflict semantics
- [IMT-N7] **Canonical conflict responses** — include machine-readable conflict metadata in reservation responses.
- [IMT-N8] **Read/write policy options** — expose explicit lock classes and conflict precedence.
- [IMT-N9] **Reservation check UX** — improve endpoint output for CI and agent tooling.

## Next (mid-term maturation)

1. API ergonomics and consistency
- [IMT-N10] **API versioning strategy** — publish endpoint and payload migration policy.
- [IMT-N11] **Pagination consistency** — standardize envelopes and metadata across handlers.
- [IMT-N12] **SDK-ready contracts** — ship a stable contract artifact for client generation.

2. Ecosystem and developer productivity
- [IMT-N13] **Versioned client generation** — publish Go client artifacts with endpoint examples.
- [IMT-N14] **Developer test fixtures** — create smoke and compatibility fixture suite.
- [IMT-N15] **Deployment topology guidance** — document recommended multi-service layouts.

3. Service maturity
- [IMT-N16] **Pluggable storage research** — evaluate backends beyond single-file SQLite.
- [IMT-N17] **High-concurrency checks** — validate behavior under multi-project contention.
- [IMT-N18] **Saturation controls** — add bounded queueing behavior under pressure.

## P2 — Coordination resilience at scale

1. Horizontal scalability planning
- [IMT-P1] **Read-replica planning** — assess partitioned persistence and replica-based scale.
- [IMT-P2] **Namespace-aware shippers** — design shippers for large, high-cardinality estates.

2. Advanced coordination primitives
- [IMT-P3] **Hierarchical locking** — add path-scoped locks and lock-intent semantics.
- [IMT-P4] **Preflight conflict prediction** — expose endpoints to validate plans before execution.

3. Governance and compliance
- [IMT-P5] **Mutation access audit** — expand traceability for project-scoped operations.
- [IMT-P6] **Signed operation metadata** — improve provenance for sensitive mutation requests.

## Backlog candidates (not yet in active plans)

- [IMT-P7] **REST v3 governance package** — publish OpenAPI v3 and generated documentation portal.
- [IMT-P8] **Reservations dashboard** — add visualization for threads and locks in real time.
- [IMT-P9] **Fine-grained authorization** — per-capability RBAC and scoped keys.
- [IMT-P10] **Event retention policy control** — configurable compaction, archive, and deletion policies.
- [IMT-P11] **High-contention benchmark harness** — dedicated load tests for conflicting reservation races.

## Success criteria for roadmap execution

- Fewer stale locks and predictable cleanup under failure scenarios.
- Lower conflict false-positive/false-negative rate for glob overlap checks.
- Measurable reliability gains after retry/circuit-breaker and sweeper hardening.
- Simpler on-call operation: operators can recover from outages using published runbooks.
- Stable public contract for clients embedding intermute as orchestration backbone.

## From Interverse Roadmap

Items from the [Interverse roadmap](../../../docs/roadmap.json) that involve this module:

- **iv-jc4j** [Next] Heterogeneous collaboration and routing experiments inspired by SC-MAS/Dr. MAS
