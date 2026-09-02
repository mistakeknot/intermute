# Operations

## Gotchas

1. **Project scoping** -- All queries must filter by project; composite PKs enforce at schema level
2. **Cursor semantics** -- `since_cursor` uses `>` not `>=`; first fetch should use cursor=0
3. **Message deduplication** -- Re-posting same message_id overwrites thread_id and body
4. **Thread indexing** -- Only messages with thread_id are indexed; non-threaded messages excluded
5. **Localhost bypass** -- Applies to 127.0.0.1 only; LAN origins require API key
6. **Contact policy on send** -- Recipients filtered by policy; partial delivery possible (some denied, some allowed); response includes `denied` list
7. **Session stale threshold** -- 5 minutes; after this, session_id can be reused by new agent registration
8. **Broadcast rate limit** -- 10 per minute per (project, sender); returns 429 with Retry-After header
9. **Topic lowercasing** -- Topics are lowercased at write time for case-insensitive discovery
10. **Optimistic locking** -- Domain entity updates check version; `ErrConcurrentModification` on conflict

## Downstream Dependencies

`pkg/embedded/` is imported by services that embed this server, and the
domain types in `internal/core/domain.go`, the handlers in
`internal/http/handlers_domain.go`, and `internal/storage/sqlite/schema.sql`
are the surface those consumers depend on. A change to any of them is an
API change: bump the version in `cmd/intermute/main.go`, add a
`CHANGELOG.md` entry, and tag.

## Testing

158 test functions across 27 files. `go test ./...` or `go test -race ./...`. Auth bypass in tests works because `httptest.NewServer` binds to 127.0.0.1.

## Router Variants

- `NewRouter` (messaging + reservations): used for lightweight messaging-only deployments
- `NewDomainRouter` (messaging + reservations + domain + health): used by `serve` command and `pkg/embedded`
- Both routers include `/api/reservations` routes
- `DomainService` embeds `*Service`, so one struct handles both messaging + domain ops

## SQLite Gotchas (Go)

- `":memory:"` with `sql.Open("sqlite", ...)` creates separate DB per connection in pool
- Concurrent tests need file-backed DB with `db.SetMaxOpenConns(1)` to avoid SQLITE_BUSY
- PRAGMAs (WAL, busy_timeout) only apply to connection they're run on
- Production uses `ResilientStore` which wraps with circuit breaker + retry for transient errors
