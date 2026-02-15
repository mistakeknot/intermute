# Plan: Multi-Session Coordination for intermute
**Phase:** planned (as of 2026-02-14T23:16:43Z)

> **Bead:** intermute-bvy
> **PRD:** [docs/research/2026-02-14-prd-multi-session-coordination.md](../research/2026-02-14-prd-multi-session-coordination.md)
> **Date:** 2026-02-14

---

## Batch 1: Documentation (F1 + F2)

These are pure documentation changes — no code, no hooks.

### Task 1.1: Add Multi-Session Coordination section to CLAUDE.md

**File:** `CLAUDE.md`

Add after the "Design Decisions" section:

```markdown
## Multi-Session Coordination

When multiple Claude Code sessions work on intermute simultaneously:

### Package Ownership Zones
- **HTTP layer** (`internal/http/`): handlers, middleware, routing
- **Storage layer** (`internal/storage/`, `internal/storage/sqlite/`): database, queries, migrations
- **WebSocket layer** (`internal/ws/`): hub, connections, subscriptions
- **Shared zone** (`internal/domain/`, `internal/core/`): coordinate via Beads before editing
- **Rarely changed** (`cmd/intermute/`): no default owner

### Before Editing Shared Files
1. Check `bd list --status=in_progress` for other active work
2. If another bead claims files in the same package, coordinate or wait
3. Create your own bead with `Files:` annotation before starting

### Beads Convention
Every task bead MUST include a `Files:` line in its description:
```
Files: internal/http/handlers_domain.go, internal/http/router.go
```
Or package-level:
```
Files: internal/http/
```
```

**Verification:** Read the updated CLAUDE.md, confirm the section is clear and actionable.

### Task 1.2: Add Multi-Session section to AGENTS.md

**File:** `AGENTS.md`

Add a new section after "Gotchas" explaining the coordination convention for non-Claude agents:

```markdown
## Multi-Session Coordination

This project supports parallel Claude Code sessions. Coordination uses Beads (`bd`) for work partitioning:

- Every task bead includes `Files:` annotation listing affected files/packages
- `bd list --status=in_progress` shows currently claimed work
- Package boundaries (`internal/http/`, `internal/storage/`, `internal/ws/`) are natural ownership zones
- `internal/domain/` and `internal/core/` are shared — use dependency ordering (blocked-by) for sequential access
```

**Verification:** Read the updated AGENTS.md, confirm consistency with CLAUDE.md.

---

## Batch 2: Conflict Check Hook (F3)

This is the enforcement mechanism. It checks Beads state before file edits.

### Task 2.1: Create the conflict check script

**File:** `scripts/check-file-conflict.sh` (new)

```bash
#!/usr/bin/env bash
# PreToolUse hook: check if target file is claimed by another in-progress bead
# Exit 0 = proceed, Exit 2 = block with message
# Claude Code hooks receive JSON on stdin with .tool_input containing the tool's parameters

set -euo pipefail

# Read tool input from stdin (Claude Code hook API)
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.path // empty')
if [[ -z "$FILE_PATH" ]]; then
    exit 0  # Can't determine file, don't block
fi

# Normalize to relative path
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
FILE_PATH="${FILE_PATH#${REPO_ROOT}/}"

# Get in-progress beads with their descriptions
BEADS_OUTPUT=$(bd list --status=in_progress 2>/dev/null || echo "")
if [[ -z "$BEADS_OUTPUT" ]]; then
    exit 0  # No in-progress beads, no conflicts
fi

# Extract bead IDs and check each one's description for Files: annotations
# that match our target file's directory
TARGET_DIR=$(dirname "$FILE_PATH")

while IFS= read -r line; do
    # Parse bead ID from bd list output (portable ERE, no -P)
    BEAD_ID=$(echo "$line" | grep -oE '[A-Za-z]+-[a-z0-9]+' | head -1)
    [[ -z "$BEAD_ID" ]] && continue

    # Get bead details
    BEAD_DETAIL=$(bd show "$BEAD_ID" 2>/dev/null || continue)

    # Check for Files: annotation (case-insensitive, not anchored to line start
    # because bd show may indent the description)
    FILES_LINE=$(echo "$BEAD_DETAIL" | grep -i "Files:" || echo "")
    [[ -z "$FILES_LINE" ]] && continue

    # Check if our file or directory is mentioned (fixed-string match, no regex)
    if echo "$FILES_LINE" | grep -qiF -e "$TARGET_DIR/" -e "$FILE_PATH"; then
        TITLE=$(echo "$BEAD_DETAIL" | head -1)
        echo "WARNING: File '$FILE_PATH' may conflict with in-progress bead $BEAD_ID ($TITLE)"
        echo "Check: bd show $BEAD_ID"
        echo "Proceeding anyway (advisory warning)."
        exit 0  # Advisory only — exit 0 to not block
    fi
done <<< "$BEADS_OUTPUT"

exit 0
```

**Design decisions:**
- Advisory (exit 0) not blocking (exit 2) — start soft, can tighten later
- Checks directory-level match, not exact file — catches package-level claims
- Tolerates `bd` not being installed (exits 0 on error)
- Reads stdin JSON per Claude Code hook API (NOT environment variables)
- Uses `grep -oE` (portable ERE) instead of `grep -oP` (PCRE, not portable)
- Uses `grep -qiF` (fixed strings) to avoid regex injection from file paths
- Appends `/` to TARGET_DIR match to prevent `internal/http` matching `internal/http_test`
- Does not filter by session — warns about ALL in-progress beads (known limitation;
  filtering by session would require session-to-bead mapping not yet implemented)

**Verification:** Test manually with a mock bead:
```bash
bd create --title="Test bead" --description="Files: internal/http/" --priority=4
bd update <id> --status=in_progress
echo '{"tool_input":{"file_path":"internal/http/handlers.go"}}' | bash scripts/check-file-conflict.sh
bd close <id>
```

### Task 2.2: Register the hook in settings

**File:** `.claude/settings.local.json`

**Merge** a `hooks` key into the existing JSON (preserving `permissions` and `prompt`):

```json
{
  "permissions": {
    "allow": [
      "Bash(wc:*)",
      "Bash(grep:*)",
      "Bash(tree:*)",
      "Bash(go test:*)",
      "Bash(git add:*)",
      "Bash(git commit:*)",
      "Bash(git push)",
      "Bash(go build:*)",
      "Bash(curl:*)",
      "Bash(pkill:*)",
      "Bash(xargs dirname:*)"
    ]
  },
  "prompt": "Before starting work: (1) run 'bd list --status=in_progress' to see claimed work, (2) run 'bd ready' for available tasks, (3) always create a bead with Files: annotation before editing code.",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "bash scripts/check-file-conflict.sh"
          }
        ]
      }
    ]
  }
}
```

**Notes:**
- This replaces the entire `settings.local.json` content (preserves existing permissions, updates prompt, adds hooks)
- Removed the invalid `__NEW_LINE_*` entry from permissions during the update
- The `prompt` field incorporates the existing `bd onboard` guidance plus new session-awareness instructions
- Goes in `settings.local.json` (not committed) so each developer can opt in

**Verification:** Start a Claude Code session, attempt to edit a file claimed by a test bead, confirm the warning appears.

---

## Batch 3: Session Awareness (F4)

### Task 3.1: Create session status script

**File:** `scripts/session-status.sh` (new)

```bash
#!/usr/bin/env bash
# Shows in-progress work for session awareness
set -euo pipefail

echo "=== intermute: Active Work ==="
BEADS=$(bd list --status=in_progress 2>/dev/null || echo "")
if [[ -z "$BEADS" ]]; then
    echo "No in-progress work. All packages available."
    exit 0
fi

echo "$BEADS"
echo ""
echo "Check 'bd ready' for available tasks."
```

**Verification:** Run with and without in-progress beads. Confirm output is concise.

### Task 3.2: Startup prompt (already handled in Task 2.2)

The updated `prompt` field in `.claude/settings.local.json` (Task 2.2) already includes session-awareness instructions. No additional work needed here.

---

## Batch 4: Worktree Scripts (F5)

### Task 4.1: Create worktree setup script

**File:** `scripts/worktree-setup.sh` (new)

```bash
#!/usr/bin/env bash
# Creates an isolated worktree for multi-agent work
set -euo pipefail

NAME="${1:?Usage: worktree-setup.sh <name>}"

# Validate name (alphanumeric, hyphens, underscores only)
if [[ ! "$NAME" =~ ^[a-zA-Z0-9_-]+$ ]]; then
    echo "ERROR: Name must contain only alphanumeric characters, hyphens, and underscores"
    exit 1
fi

# Resolve paths from repo root, not CWD
REPO_ROOT=$(git rev-parse --show-toplevel)
WORKTREE_DIR="${REPO_ROOT}/../intermute-${NAME}"
BRANCH="work/${NAME}"

if [[ -d "$WORKTREE_DIR" ]]; then
    echo "Worktree already exists: $WORKTREE_DIR"
    exit 1
fi

git worktree add "$WORKTREE_DIR" -b "$BRANCH"

# Copy settings (create .claude dir first — it won't exist in a fresh worktree)
mkdir -p "$WORKTREE_DIR/.claude"
cp "${REPO_ROOT}/.claude/settings.local.json" "$WORKTREE_DIR/.claude/settings.local.json" 2>/dev/null || true

# Copy scripts directory for hooks
if [[ -d "${REPO_ROOT}/scripts" ]]; then
    cp -r "${REPO_ROOT}/scripts" "$WORKTREE_DIR/scripts"
fi

echo "Worktree created: $WORKTREE_DIR (branch: $BRANCH)"
echo ""
echo "Start a session:"
echo "  cd $WORKTREE_DIR && claude"
echo ""
echo "When done, merge and clean up:"
echo "  ${REPO_ROOT}/scripts/worktree-teardown.sh $NAME"
```

### Task 4.2: Create worktree teardown script

**File:** `scripts/worktree-teardown.sh` (new)

```bash
#!/usr/bin/env bash
# Merges worktree branch to main and removes the worktree
# MUST be run from the main working tree (not from inside a worktree)
set -euo pipefail

NAME="${1:?Usage: worktree-teardown.sh <name>}"
REPO_ROOT=$(git rev-parse --show-toplevel)
WORKTREE_DIR="${REPO_ROOT}/../intermute-${NAME}"
BRANCH="work/${NAME}"

if [[ ! -d "$WORKTREE_DIR" ]]; then
    echo "Worktree not found: $WORKTREE_DIR"
    exit 1
fi

# Ensure we're running from the main working tree
MAIN_WORKTREE=$(git worktree list --porcelain | head -1 | sed 's/worktree //')
if [[ "$REPO_ROOT" != "$MAIN_WORKTREE" ]]; then
    echo "ERROR: Run this from the main working tree ($MAIN_WORKTREE), not a worktree"
    exit 1
fi

# Check for uncommitted changes in worktree
if [[ -n "$(git -C "$WORKTREE_DIR" status --porcelain)" ]]; then
    echo "ERROR: Uncommitted changes in $WORKTREE_DIR"
    echo "Commit or stash before teardown."
    exit 1
fi

# Check for uncommitted changes in main tree
if [[ -n "$(git status --porcelain)" ]]; then
    echo "ERROR: Uncommitted changes in main working tree"
    echo "Commit or stash before merging."
    exit 1
fi

# Merge to main (we're already on main in the main working tree)
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "main" ]]; then
    echo "ERROR: Main working tree is on branch '$CURRENT_BRANCH', expected 'main'"
    exit 1
fi

git merge "$BRANCH" --no-edit
echo "Merged $BRANCH into main."

# Clean up
git worktree remove "$WORKTREE_DIR"
git branch -d "$BRANCH"
echo "Removed worktree and branch."
```

**Verification:** Create a worktree, make a small change, commit it, tear down, verify merge to main.

---

## Execution Order

| Batch | Tasks | Parallel? | Dependencies |
|-------|-------|-----------|-------------|
| 1 | 1.1, 1.2 | Yes | None |
| 2 | 2.1, 2.2 | Sequential (2.1 before 2.2) | Batch 1 (needs conventions defined) |
| 3 | 3.1, 3.2 | Yes | Batch 1 |
| 4 | 4.1, 4.2 | Yes | None |

Batches 1 and 4 can run in parallel. Batch 2 depends on Batch 1. Batch 3 depends on Batch 1.

## Verification Checklist

- [ ] CLAUDE.md has Multi-Session Coordination section
- [ ] AGENTS.md has Multi-Session Coordination section
- [ ] `scripts/check-file-conflict.sh` exists and is executable
- [ ] Hook is registered in `.claude/settings.local.json`
- [ ] `scripts/session-status.sh` exists and is executable
- [ ] `scripts/worktree-setup.sh` and `scripts/worktree-teardown.sh` exist and are executable
- [ ] All scripts have `chmod +x` applied
- [ ] `go test ./...` still passes (no regressions)
- [ ] Manual test: create bead with Files: annotation → conflict check warns when editing claimed file
