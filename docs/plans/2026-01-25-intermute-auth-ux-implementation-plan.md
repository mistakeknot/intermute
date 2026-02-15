# intermute Auth UX Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Bead:** `Vauxpraudemonium-1wl` (Improve auth usability: init + dev key)

**Goal:** Make auth setup user-friendly by adding `intermute init --project <name>` and auto-generating a dev key when no keys file exists.

**Architecture:** Keep dependencies minimal by extending `cmd/intermute/main.go` with a lightweight subcommand parser. Add an auth bootstrap helper that (1) generates a keys file via `init` and (2) auto-creates a localhost-only dev key when the keys file is missing, logging clear instructions. Use the existing auth keyring loader and keep REST/WS enforcement unchanged.

**Tech Stack:** Go 1.24.12 toolchain, net/http, SQLite (modernc.org/sqlite), YAML (gopkg.in/yaml.v3).

---

### Task 1: Add CLI init command scaffolding

**Files:**
- Modify: `cmd/intermute/main.go`
- Create: `internal/cli/init.go`
- Create: `internal/cli/init_test.go`

**Step 1: Write the failing test**

In `internal/cli/init_test.go`, add a test that runs init logic into a temp dir and expects:
- A keys file is created
- The requested project exists with at least one key

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/intermute && go test ./internal/cli -v`
Expected: FAIL (package not implemented)

**Step 3: Write minimal implementation**

Implement `internal/cli/init.go` with:
- `InitKeysFile(path, project string) (generatedKey string, err error)`
- Generates a secure random key
- Writes YAML structure compatible with `internal/auth/config.go`

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/intermute && go test ./internal/cli -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/intermute add internal/cli
git -C /root/projects/intermute commit -m "feat(cli): add intermute init"
```

---

### Task 2: Wire `intermute init` into main

**Files:**
- Modify: `cmd/intermute/main.go`
- Modify: `README.md`

**Step 1: Write the failing test**

Add a small unit test in `internal/cli/init_test.go` (or a new one) that exercises a `Run(args []string)` helper and expects it to call init behavior when args start with `init`.

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/intermute && go test ./internal/cli -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Update `cmd/intermute/main.go` to:
- Detect `init` subcommand
- Support `intermute init --project autarch` and optional `--keys-file`
- Print:
  - where the file was written
  - the generated key
  - exact `export` commands

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/intermute && go test ./internal/cli -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/intermute add cmd/intermute/main.go README.md
git -C /root/projects/intermute commit -m "feat(cli): wire intermute init command"
```

---

### Task 3: Auto-generate a dev key when keys file is missing

**Files:**
- Modify: `internal/auth/config.go`
- Modify: `cmd/intermute/main.go`
- Create: `internal/auth/bootstrap.go`
- Create: `internal/auth/bootstrap_test.go`

**Step 1: Write the failing test**

In `internal/auth/bootstrap_test.go`, add a test that:
- Uses a temp dir with no keys file
- Calls bootstrap helper
- Expects:
  - keys file created
  - localhost bypass preserved
  - dev key is stored under a known dev project (e.g., `dev`)

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/intermute && go test ./internal/auth -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Implement a bootstrap helper that:
- Checks if the resolved keys path exists
- If missing, generates a dev key and writes a minimal keys file
- Returns the generated key so `main` can log it clearly

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/intermute && go test ./internal/auth -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/intermute add internal/auth cmd/intermute/main.go
git -C /root/projects/intermute commit -m "feat(auth): bootstrap dev keys when missing"
```

---

### Task 4: Project-scoped WS ergonomics and docs updates

**Files:**
- Modify: `internal/ws/gateway.go`
- Modify: `internal/ws/gateway_test.go`
- Modify: `README.md`
- Modify: `intermute.keys.yaml.example`

**Step 1: Write the failing test**

Add a WS test covering localhost dev mode that:
- connects without auth
- includes `?project=dev`
- receives events for that project only

**Step 2: Run test to verify it fails**

Run: `cd /root/projects/intermute && go test ./internal/ws -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Ensure WS uses project from:
1) auth context (non-localhost)
2) `?project=` (localhost)

This is mostly present; adjust only if tests reveal gaps.

**Step 4: Run test to verify it passes**

Run: `cd /root/projects/intermute && go test ./internal/ws -v`
Expected: PASS

**Step 5: Commit**

```bash
git -C /root/projects/intermute add internal/ws README.md intermute.keys.yaml.example
git -C /root/projects/intermute commit -m "docs(auth): improve dev and ws setup guidance"
```

---

### Task 5: Full verification + Autarch compatibility check

**Files:**
- Modify: `README.md` (if needed)

**Step 1: Run full verification**

Run:
- `cd /root/projects/intermute && go test ./...`
- `cd /root/projects/Autarch && go test ./...`

Expected: PASS

**Step 2: Manual smoke test commands**

Document and run:

```bash
cd /root/projects/intermute
rm -f intermute.keys.yaml

go run ./cmd/intermute init --project autarch
INTERMUTE_KEYS_FILE=./intermute.keys.yaml go run ./cmd/intermute
```

Then in another shell:

```bash
export INTERMUTE_URL=http://localhost:7338
export INTERMUTE_API_KEY=<printed-key>
export INTERMUTE_PROJECT=autarch
./dev bigend
```

**Step 3: Commit**

```bash
git -C /root/projects/intermute add docs/plans/2026-01-25-intermute-auth-ux-implementation-plan.md README.md
git -C /root/projects/intermute commit -m "docs: add auth ux plan and smoke test steps"
```
