# intermute Auth UX Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Bead:** `intermute-fw3 (Task reference)`

**Goal:** Add a user-friendly `intermute init --project <name>` command and auto-generate a dev key when the keyring file is missing.

**Architecture:** Extend the Cobra CLI to write a keys file using existing `internal/cli` helpers. Update auth loading to optionally create a dev key file on first run, keeping localhost bypass behavior and using the existing keyring format.

**Tech Stack:** Go 1.24, Cobra CLI, YAML (gopkg.in/yaml.v3), SQLite.

### Task 1: Wire `intermute init` CLI command

**Files:**
- Modify: `cmd/intermute/main.go`
- Modify: `internal/cli/init.go`
- Test: `internal/cli/init_test.go`

**Step 1: Write the failing test**

```go
func TestInitCommandCreatesKey(t *testing.T) {
    tmp := t.TempDir()
    keyPath := filepath.Join(tmp, "intermute.keys.yaml")

    cmd := initCmd()
    cmd.SetArgs([]string{"--project", "demo", "--keys", keyPath})

    if err := cmd.Execute(); err != nil {
        t.Fatalf("execute init: %v", err)
    }

    data, err := os.ReadFile(keyPath)
    if err != nil {
        t.Fatalf("read keys file: %v", err)
    }
    if !bytes.Contains(data, []byte("demo")) {
        t.Fatalf("expected project section to be written")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/intermute -run TestInitCommandCreatesKey -v`
Expected: FAIL with missing init command / initCmd symbol

**Step 3: Write minimal implementation**

- Add `init` subcommand in `cmd/intermute/main.go`.
- Flags:
  - `--project` (required)
  - `--keys` (optional, default `auth.ResolveKeysPath()`)
- Call `cli.InitKeysFile(path, project)` and print the generated key.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/intermute -run TestInitCommandCreatesKey -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/intermute/main.go internal/cli/init.go internal/cli/init_test.go

git commit -m "feat(auth): add intermute init command"
```

### Task 2: Auto-generate dev key when keys file missing

**Files:**
- Modify: `internal/auth/config.go`
- Test: `internal/auth/middleware_test.go`

**Step 1: Write the failing test**

```go
func TestLoadKeyringCreatesDevKeyWhenMissing(t *testing.T) {
    tmp := t.TempDir()
    keysPath := filepath.Join(tmp, "intermute.keys.yaml")

    ring, err := LoadKeyring(keysPath)
    if err != nil {
        t.Fatalf("load keyring: %v", err)
    }
    if _, err := os.Stat(keysPath); err != nil {
        t.Fatalf("expected keys file to be created")
    }

    if len(ring.keyToProject) != 1 {
        t.Fatalf("expected 1 dev key, got %d", len(ring.keyToProject))
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth -run TestLoadKeyringCreatesDevKeyWhenMissing -v`
Expected: FAIL because no keys file is created and keyring empty

**Step 3: Write minimal implementation**

- In `LoadKeyring`, if file missing:
  - generate a dev key (32 bytes base64url)
  - write keys file at the provided path
  - set `default_policy.allow_localhost_without_auth: true`
  - create a `projects` entry with project name `dev` (or `default`) containing the new key
  - return a keyring with that key mapped to the dev project

**Step 4: Run test to verify it passes**

Run: `go test ./internal/auth -run TestLoadKeyringCreatesDevKeyWhenMissing -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/config.go internal/auth/middleware_test.go

git commit -m "feat(auth): auto-create dev key on missing keyring"
```

### Task 3: CLI docs for init and keyring behavior

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

**Step 1: Write the failing doc expectation test (if docs tests exist)**

If no doc tests exist, skip to Step 2.

**Step 2: Update docs**

- Add `intermute init --project <name>` example
- Explain default keys file path (env `INTERMUTE_KEYS_FILE` else `./intermute.keys.yaml`)
- Note auto-created dev key behavior on first run

**Step 3: (Optional) Run docs/tests**

If docs tests exist, run them.

**Step 4: Commit**

```bash
git add AGENTS.md README.md

git commit -m "docs(auth): document init and dev key behavior"
```

### Task 4: Full verification

**Step 1: Run full tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Commit any remaining fixes**

```bash
git add -A

git commit -m "test: verify auth ux changes"
```

