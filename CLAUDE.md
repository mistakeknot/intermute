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
