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

## Execution Defect Reporting

Agents report execution-time bugs via Intermute messages with these conventions:

- **Message ID as bug identity**: Use a stable, deterministic ID for deduplication: `defect:<hash>` where hash is derived from the failing evidence (e.g., test name + file path). The `(project, message_id)` composite key deduplicates automatically.
- **Subject prefix**: `[defect]` for structured filtering (e.g., `[defect] Parser fails on nested fn expressions`)
- **Topic**: `execution-defect` for cross-cutting discovery
- **Body format**: JSON with fields: `test_name`, `file_path`, `error_output`, `diagnosis`, `suggested_fix`, `severity` (P0-P3)
- **Kernel integration**: After sending the Intermute message, emit a review event: `ic events emit --source=review --type=execution_defect --context='{"finding_id":"defect:<hash>","agents":{"reporter":"<from>","target":"<to>"},"resolution":"reported","chosen_severity":"P1","impact":"execution_defect"}'`

This convention enables both human-readable messaging (Intermute) and machine-readable evidence (Interspect via review_events).

## Error Handling

- HTTP handlers return status codes with JSON error bodies for structured errors (conflicts, policy denied)
- Store methods wrap errors with context (`fmt.Errorf("operation: %w", err)`)
- Sentinel errors: `ErrNotFound`, `ErrConcurrentModification`, `ErrActiveSessionConflict`, `ErrPolicyDenied`
