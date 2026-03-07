# Resilience & Coordination

## Resilience Layers

- **ResilientStore** wraps every Store/DomainStore method with CircuitBreaker + RetryOnDBLock
- **CircuitBreaker** (threshold=5 failures, reset timeout=30s): closed -> open -> half-open
- **RetryOnDBLock**: retries transient SQLite "database is locked" errors
- **QueryLogger**: logs slow queries above 100ms threshold
- **Sweeper**: background goroutine (60s interval) cleaning expired reservations from inactive agents (5min heartbeat grace); emits `reservation.expired` events

## Intercore Coordination Bridge

Optional dual-write mode mirrors file reservations to Intercore's `coordination_locks` table.

- Enable: `--coordination-dual-write` flag on `serve`
- Auto-discovers `intercore.db` or use `--intercore-db` path
- Used during migration phase; Intermute remains the primary reservation store
