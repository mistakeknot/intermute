# Conventions

## Naming

- Handlers: `handleXxx` (e.g., handleSendMessage)
- Requests: `xxxRequest` (e.g., sendMessageRequest)
- Responses: `xxxResponse` (e.g., sendMessageResponse)
- JSON structs: `xxxJSON` for API serialization

## Database

- Composite PKs: (project, id) for multi-tenancy
- Timestamps: time.Time in Go, RFC3339Nano in SQLite, ISO8601 in JSON
- JSON marshaling for complex types (capabilities, metadata, to_json, steps_json, etc.)
- Optimistic locking: domain entities use `version` field; `ErrConcurrentModification` on conflict

## Error Handling

- HTTP handlers return status codes with JSON error bodies for structured errors (conflicts, policy denied)
- Store methods wrap errors with context (`fmt.Errorf("operation: %w", err)`)
- Sentinel errors: `ErrNotFound`, `ErrConcurrentModification`, `ErrActiveSessionConflict`, `ErrPolicyDenied`
