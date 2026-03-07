# CLI Reference

## Commands

```bash
# Run server
go run ./cmd/intermute serve

# Run with all options
go run ./cmd/intermute serve --host 0.0.0.0 --port 7338 --db ./intermute.db \
  --socket /var/run/intermute.sock --coordination-dual-write

# Initialize auth keys for a project
go run ./cmd/intermute init --project autarch --keys-file ./intermute.keys.yaml

# Run tests
go test ./...

# Build binary
go build -o intermute ./cmd/intermute
```

## Server CLI Flags

`intermute serve` supports:
- `--host` (default: `127.0.0.1`)
- `--port` (default: `7338`)
- `--db` (default: `intermute.db`)
- `--socket` (default: empty; Unix domain socket path)
- `--coordination-dual-write` (default: false; mirror to Intercore)
- `--intercore-db` (default: empty; auto-discovered if omitted)

## Authentication Model

```yaml
# intermute.keys.yaml
default_policy:
  allow_localhost_without_auth: true
projects:
  project-a:
    keys:
      - secret-key-1
  project-b:
    keys:
      - secret-key-2
```

When using API key auth, POST operations must include `project` field matching the key's project.

## Client Environment

- `INTERMUTE_URL` (client-side) e.g. `http://localhost:7338`
- `INTERMUTE_API_KEY` (optional; required for non-localhost)
- `INTERMUTE_PROJECT` (required when `INTERMUTE_API_KEY` is set)
- `INTERMUTE_AGENT_NAME` (optional override)
