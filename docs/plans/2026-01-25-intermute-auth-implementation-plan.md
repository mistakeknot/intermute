# Intermute Auth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Bead:** `Vauxpraudemonium-1wl` (Intermute auth model)

**Goal:** Add project-scoped API key auth with localhost bypass, enforced across REST and WebSocket, without breaking local dev.

**Architecture:** Load project keys from `INTERMUTE_KEYS_FILE` (fallback `./intermute.keys.yaml`) into an in-memory key→project map. Use an auth middleware that bypasses localhost, otherwise requires `Authorization: Bearer <key>` and attaches the project to request context. Enforce project isolation in storage and handlers.

**Tech Stack:** Go 1.24.12 toolchain, net/http middleware, SQLite (modernc.org/sqlite), nhooyr.io/websocket.

---

### Task 1: Auth config loading + middleware skeleton

**Files:**
- Create: `internal/auth/config.go`
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/middleware_test.go`
- Modify: `cmd/intermute/main.go`

**Step 1: Write the failing test**

In `internal/auth/middleware_test.go`, add a test that:
- Allows localhost without auth
- Rejects non-localhost without bearer token

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/Intermute && go test ./internal/auth -v`
Expected: FAIL (auth package missing)

**Step 3: Write minimal implementation**

Implement:
- Key file resolution via `INTERMUTE_KEYS_FILE` fallback
- Localhost detection
- Bearer parsing and key→project lookup
- Context attachment for project/auth mode

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/Intermute && go test ./internal/auth -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/Intermute add internal/auth cmd/intermute/main.go
git -C /root/projects/Intermute commit -m "feat(auth): add key config and middleware"
```

---

### Task 2: Project isolation in core models + storage interface

**Files:**
- Modify: `internal/core/models.go`
- Modify: `internal/storage/storage.go`
- Modify: `internal/storage/storage_test.go`

**Step 1: Write the failing test**

Update `internal/storage/storage_test.go` to require `InboxSince` to accept a project argument and return only project-scoped messages.

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/Intermute && go test ./internal/storage -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add `Project` to message/event models and update the storage interface to include `project` in `InboxSince`.

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/Intermute && go test ./internal/storage -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/Intermute add internal/core internal/storage
git -C /root/projects/Intermute commit -m "refactor(storage): add project-scoped inbox interface"
```

---

### Task 3: SQLite schema migration + project-scoped queries

**Files:**
- Modify: `internal/storage/sqlite/schema.sql`
- Modify: `internal/storage/sqlite/sqlite.go`
- Modify: `internal/storage/sqlite/sqlite_test.go`

**Step 1: Write the failing test**

Add a test that inserts messages for two projects and verifies `InboxSince(projectA, agent, cursor)` returns only projectA messages.

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/Intermute && go test ./internal/storage/sqlite -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Implement:
- Project columns in `events`, `messages`, `inbox_index`
- Migration that recreates `messages`/`inbox_index` if needed
- Project-aware inserts and queries

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/Intermute && go test ./internal/storage/sqlite -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/Intermute add internal/storage/sqlite
git -C /root/projects/Intermute commit -m "feat(storage): enforce project isolation"
```

---

### Task 4: Handler enforcement for project requirements

**Files:**
- Modify: `internal/http/handlers_agents.go`
- Modify: `internal/http/handlers_messages.go`
- Modify: `internal/http/handlers_agents_test.go`
- Modify: `internal/http/handlers_messages_test.go`
- Modify: `internal/http/router.go`

**Step 1: Write the failing test**

Add tests that:
- Require `project` for non-localhost/authenticated requests
- Reject project mismatch vs authenticated project

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/Intermute && go test ./internal/http -v`
Expected: FAIL

**Step 3: Write minimal implementation**

- Apply auth middleware to REST routes
- Enforce project presence/match when auth mode is non-localhost
- Pass project into storage calls

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/Intermute && go test ./internal/http -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/Intermute add internal/http
git -C /root/projects/Intermute commit -m "feat(api): enforce project auth rules"
```

---

### Task 5: WebSocket auth + project scoping

**Files:**
- Modify: `internal/ws/gateway.go`
- Modify: `internal/ws/gateway_test.go`
- Modify: `internal/http/router.go`

**Step 1: Write the failing test**

Update WS tests to:
- Connect with bearer key for non-localhost
- Ensure WS respects project scoping

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/Intermute && go test ./internal/ws -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Wrap WS handler with the same auth middleware and associate connections with project+agent.

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/Intermute && go test ./internal/ws -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/Intermute add internal/ws internal/http/router.go
git -C /root/projects/Intermute commit -m "feat(ws): apply auth middleware"
```

---

### Task 6: Client auth header + project field

**Files:**
- Modify: `client/client.go`
- Modify: `client/client_test.go`

**Step 1: Write the failing test**

Add a test that verifies the client sets `Authorization: Bearer ...` when configured and includes `project` on register/send.

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/Intermute && go test ./client -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add client options for API key + project and set the header on requests.

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/Intermute && go test ./client -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/Intermute add client
git -C /root/projects/Intermute commit -m "feat(client): add bearer auth and project"
```

---

### Task 7: Docs + full verification

**Files:**
- Modify: `README.md`
- Create: `intermute.keys.yaml.example`

**Step 1: Write the failing check**

Document the exact commands to run the server and exercise auth behavior.

**Step 2: Run full verification**

Run:
- `cd /root/projects/Intermute && go test ./...`
- `cd /root/projects/Autarch && go test ./...`

Expected: PASS

**Step 3: Commit**

```bash
git -C /root/projects/Intermute add README.md intermute.keys.yaml.example docs/plans/2026-01-25-intermute-auth-implementation-plan.md
git -C /root/projects/Intermute commit -m "docs(auth): add key config and usage"
```
