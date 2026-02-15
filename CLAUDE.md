# CLAUDE.md

> **Documentation is in AGENTS.md** - This file contains Claude-specific settings only.
> For project documentation, architecture, and conventions, see [AGENTS.md](./AGENTS.md).

## Quick Commands

```bash
go run ./cmd/intermute   # Run server on :7338
go test ./...            # Run all tests
```

## Claude-Specific Settings

- Prefer `Read` tool over `cat` for file contents
- Use `go test ./...` to verify changes compile and pass tests
- Project uses Go 1.24 with SQLite (pure Go driver, no CGO)

## Design Decisions (Do Not Re-Ask)

- Cursor-based pagination (not offset/limit) for inbox and threads
- Composite primary keys (project, id) for multi-tenant isolation
- Event sourcing pattern: append to events table, materialize to indexes
- Thread indexing tracks all participants (sender + recipients)
- Domain entities (specs/epics/stories/tasks) follow same CRUD pattern

## Multi-Session Coordination

When multiple Claude Code sessions work on intermute simultaneously:

### Package Ownership Zones
- **HTTP layer** (`internal/http/`): handlers, middleware, routing
- **Storage layer** (`internal/storage/`, `internal/storage/sqlite/`): database, queries, migrations
- **WebSocket layer** (`internal/ws/`): hub, connections, subscriptions
- **Shared zone** (`internal/domain/`, `internal/core/`): coordinate via Beads before editing
- **Rarely changed** (`cmd/intermute/`, `client/`): no default owner

### Before Editing Shared Files
1. Check `bd list --status=in_progress` for other active work
2. If another bead claims files in the same package, coordinate or wait
3. Create your own bead with `Files:` annotation before starting

### Beads File Convention
Every task bead MUST include a `Files:` line in its description listing affected files or packages:
```
Files: internal/http/handlers_domain.go, internal/http/router.go
```
Or package-level: `Files: internal/http/`
