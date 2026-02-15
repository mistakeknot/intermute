---
module: Storage
date: 2026-02-11
problem_type: database_issue
component: database
symptoms:
  - "Silent `_ = json.Unmarshal(...)` hides data corruption on reads"
  - "Silent `_ = json.Marshal(...)` risks persisting garbage on writes"
  - "Corrupt JSON in one row silently returns zero-value slices to callers"
  - "Write-path marshal failures silently insert NULL or empty strings into SQLite"
root_cause: missing_validation
framework_version: 1.24.0
resolution_type: code_fix
severity: critical
tags: [json-marshal, json-unmarshal, sqlite, event-sourcing, data-integrity, error-handling]
---

# Troubleshooting: Silent JSON Marshal/Unmarshal Errors in Event-Sourced SQLite Store

## Problem

All `json.Marshal` and `json.Unmarshal` calls in the storage layer discarded errors with `_ =`, meaning write-path failures would silently persist corrupt/empty data into SQLite, and read-path failures would silently return zero-value slices to callers. In an event-sourced system where events are append-only and materialized to multiple index tables, a single corrupt write propagates across inbox_index, thread_index, and message_recipients with no indication of failure.

## Environment
- Module: Storage (internal/storage/sqlite)
- Framework Version: Go 1.24.0
- Affected Component: SQLite storage layer — both messaging (sqlite.go) and domain entities (domain.go)
- Date: 2026-02-11

## Symptoms
- `_ = json.Unmarshal(...)` on read paths means a corrupt `to_json`, `cc_json`, or `bcc_json` column silently yields `[]string{}` — callers see an empty recipient list with no error
- `_ = json.Marshal(...)` on write paths means if marshal fails (e.g., channel types, nil interface values), the INSERT/UPDATE proceeds with empty or garbage data
- In domain.go, `AcceptanceCriteria`, `Steps`, `SuccessCriteria`, and `ErrorRecovery` JSON fields had the same pattern
- Agent registration (`RegisterAgent`) silently discarded marshal errors for `capabilities_json` and `metadata_json`

## What Didn't Work

**Direct solution:** The problem was identified through multi-agent review (flux-drive with correctness, safety, and quality reviewers) and fixed on the first attempt. The key design decision was choosing between fail-hard on all paths vs. a two-tier strategy.

**Design debate:** Three reviewers disagreed on read-path handling:
- Correctness reviewer: fail hard on all paths (return errors)
- Quality reviewer: log warnings and continue (don't break queries for one bad row)
- Safety reviewer: fail hard but deploy read-path fix after write-path fix

Resolution: adopted log-and-continue for reads, fail-hard for writes.

## Solution

**Two-tier error handling strategy:**

1. **Write paths (Marshal before INSERT/UPDATE):** Return errors immediately to prevent corrupt data from entering the database
2. **Read paths (Unmarshal after SELECT):** Log warnings with entity IDs and continue with zero-value slices, so one corrupt row doesn't break entire queries

**Code changes — Write path (fail hard):**

```go
// Before (broken):
toJSON, _ := json.Marshal(msg.To)
ccJSON, _ := json.Marshal(msg.CC)
bccJSON, _ := json.Marshal(msg.BCC)

// After (fixed):
toJSON, err := json.Marshal(msg.To)
if err != nil {
    return fmt.Errorf("marshal to: %w", err)
}
ccJSON, err := json.Marshal(msg.CC)
if err != nil {
    return fmt.Errorf("marshal cc: %w", err)
}
bccJSON, err := json.Marshal(msg.BCC)
if err != nil {
    return fmt.Errorf("marshal bcc: %w", err)
}
```

**Code changes — Read path (log and continue):**

```go
// Before (broken):
_ = json.Unmarshal([]byte(toJSON), &msg.To)
_ = json.Unmarshal([]byte(ccJSON), &msg.CC)
_ = json.Unmarshal([]byte(bccJSON), &msg.BCC)

// After (fixed):
if err := json.Unmarshal([]byte(toJSON), &msg.To); err != nil {
    log.Printf("WARN: corrupt to_json for message %s: %v", msg.ID, err)
}
if err := json.Unmarshal([]byte(ccJSON), &msg.CC); err != nil {
    log.Printf("WARN: corrupt cc_json for message %s: %v", msg.ID, err)
}
if err := json.Unmarshal([]byte(bccJSON), &msg.BCC); err != nil {
    log.Printf("WARN: corrupt bcc_json for message %s: %v", msg.ID, err)
}
```

**Companion fix — Transaction wrapping:**

The write-path fix was paired with wrapping `AppendEvent` in a transaction (BEGIN/COMMIT with `defer tx.Rollback()`), so that multi-table materialization (event → message → inbox_index → thread_index → recipients) is atomic. Helper methods (`upsertMessageTx`, `insertRecipientsTx`) now accept `*sql.Tx` instead of using `s.db` directly.

## Why This Works

1. **Root cause:** Go's `json.Marshal`/`json.Unmarshal` return errors for invalid inputs (unsupported types, malformed JSON), but the codebase universally discarded these errors with `_ =`.

2. **Write-path fail-hard prevents corruption at the source:** If `json.Marshal` fails, the INSERT never executes, so no corrupt data enters the database. This is critical in an event-sourced system where events are append-only — you can't fix a bad write after the fact.

3. **Read-path log-and-continue preserves availability:** If one row has corrupt JSON (from before this fix, or from an external SQLite edit), logging a warning and returning a zero-value slice is better than failing the entire query. The caller gets degraded data for that one row but the inbox/thread listing still works.

4. **Transaction wrapping ensures atomicity:** Even with marshal errors properly caught, a partial write (event inserted but materialization failed) would leave the database in an inconsistent state. The transaction ensures all-or-nothing.

## Prevention

- **Grep for `_ = json.` regularly:** Run `grep -rn '_ = json\.' internal/storage/` as a CI check. Zero matches should be the target.
- **Wrap multi-table writes in transactions:** Any function that writes to more than one table should use `BEGIN`/`COMMIT` with `defer tx.Rollback()`.
- **Pass `*sql.Tx` to helper methods:** Don't let helpers use `s.db` directly when they're called within a transaction — they need the same `*sql.Tx` to participate in the transaction.
- **Use `log.Printf` with entity IDs on read paths:** Always include the entity ID in the warning so corrupt rows can be identified and repaired.
- **Consider a linter rule:** A custom `go vet` check or `golangci-lint` rule could flag `_ = json.Marshal` and `_ = json.Unmarshal` patterns.

## Related Issues

- Companion fix: Transaction wrapping for AppendEvent (beads: intermute-lcr)
- Companion fix: HTTP 409 Conflict for ErrConcurrentModification (beads: intermute-l0y)
- See also: Event sourcing pattern in `internal/storage/sqlite/sqlite.go` — events table → materialized indexes
