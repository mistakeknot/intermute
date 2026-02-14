# PRD: Multi-Session Coordination for Intermute

> **Bead:** Intermute-bvy
> **Date**: 2026-02-14
> **Status**: Draft
> **Source**: [Brainstorm](./2026-02-14-brainstorm-clavain-multi-session-coordination.md)

---

## Problem Statement

Multiple Claude Code sessions working on Intermute simultaneously collide in three ways: file edit conflicts (lost changes), test interference (SQLite lock contention), and duplicate work (sessions unaware of each other's scope). There is no enforcement mechanism — coordination is entirely ad hoc.

## Target Users

- The project owner (running 2-3 concurrent Claude Code sessions on Intermute)
- Future contributors who may spin up sessions in parallel

## Success Criteria

1. **Zero lost changes**: No session's work is silently overwritten by another
2. **Clear ownership**: Before editing a file, a session knows if another session claims it
3. **No new runtime dependencies**: Solutions must work with existing tooling (Beads, Clavain hooks, git)
4. **Incremental adoption**: Each feature is independently useful; no big-bang migration

## Non-Goals

- Supporting 5+ concurrent agents (would require full orchestration framework)
- Building a custom MCP server or coordination service
- Modifying Intermute's application code (this is a *development process* improvement)
- File-level or function-level locking (too granular for this codebase)

---

## Features

### Feature 1: CLAUDE.md Package Ownership Convention

**What**: Add a `## Multi-Session Coordination` section to Intermute's CLAUDE.md defining which packages each session should own.

**Why**: The cheapest possible coordination — every session reads CLAUDE.md on startup. Package-level boundaries match Intermute's natural architecture.

**Spec**:
- Define 3 ownership zones: HTTP layer, Storage layer, WebSocket layer
- Mark `internal/domain/` as shared (coordinate via Beads before editing)
- Mark `cmd/intermute/` as rarely-changed (no default owner)
- Include a "how to claim" instruction pointing to Beads

**Files touched**: `CLAUDE.md`

### Feature 2: Beads File Annotation Convention

**What**: Establish a convention that every task bead includes a `Files:` line in its description listing the files/packages it will modify.

**Why**: Makes file ownership queryable. Sessions can check `bd list --status=in_progress` to see what files are claimed by other work.

**Spec**:
- Convention: `Files: internal/http/handlers.go, internal/http/middleware.go`
- Or package-level: `Files: internal/http/`
- Document the convention in CLAUDE.md and AGENTS.md
- When creating beads through Clavain workflow, always include Files annotation

**Files touched**: `CLAUDE.md`, `AGENTS.md`

### Feature 3: PreToolUse Conflict Check Hook

**What**: A Claude Code hook that runs before Write/Edit operations, checking if the target file is claimed by an in-progress bead assigned to a different session.

**Why**: This is the enforcement mechanism that turns conventions into guardrails. Without it, the conventions are aspirational.

**Spec**:
- Trigger: `PreToolUse` on `Write|Edit` matchers
- Script reads the file path from the tool input
- Queries `bd list --status=in_progress --json` (or parses text output)
- Extracts `Files:` annotations from each in-progress bead's description
- If the target file matches a bead NOT owned by the current session:
  - Exit code 2 (blocks with feedback): "File `X` is claimed by bead `Y` (assigned to session Z). Coordinate before editing."
- If no conflict: Exit code 0 (proceed)
- Performance: Must complete in <500ms to not slow down editing

**Files touched**: `.claude/settings.local.json` (hook registration), new script at `scripts/check-file-conflict.sh`

### Feature 4: Session Awareness at Startup

**What**: Enhance the SessionStart experience to show what other work is in progress and which files are claimed.

**Why**: Prevention is better than detection. If a session knows what's already claimed at startup, it can choose non-conflicting work.

**Spec**:
- On session start, display:
  - Count of in-progress beads
  - For each in-progress bead: title, assignee, claimed files
  - Suggested available packages (not claimed by any in-progress bead)
- Implementation: Add to Clavain's SessionStart hook or create Intermute-specific hook
- Output should be concise (5-10 lines max)

**Files touched**: `.claude/settings.local.json` or Clavain hook configuration

### Feature 5: Worktree Setup Script (Tier 3 Path)

**What**: A script that creates git worktrees for isolated multi-agent work, with per-worktree test database setup.

**Why**: For features that touch shared code (like `internal/domain/`), conventions alone aren't enough. Worktrees provide filesystem isolation.

**Spec**:
- Script: `scripts/worktree-setup.sh <name>`
- Creates `../intermute-<name>` worktree with branch `work/<name>`
- Copies `.claude/settings.local.json` to the worktree
- Creates isolated test database directory
- Prints instructions for starting a Claude Code session
- Complementary teardown: `scripts/worktree-teardown.sh <name>` merges to main and removes

**Files touched**: New `scripts/worktree-setup.sh`, `scripts/worktree-teardown.sh`

---

## Implementation Priority

| Feature | Priority | Effort | Dependencies |
|---------|----------|--------|-------------|
| F1: CLAUDE.md conventions | P1 | 30 min | None |
| F2: Beads file annotations | P1 | 30 min | None |
| F3: Conflict check hook | P2 | 2-3 hours | F2 (needs annotations to check against) |
| F4: Session awareness | P3 | 1-2 hours | F2 |
| F5: Worktree scripts | P3 | 2-3 hours | None |

**Recommended execution order**: F1 → F2 → F3 → F4 → F5

F1 and F2 are documentation-only and can be done immediately. F3 is the key enforcement mechanism. F4 and F5 are quality-of-life improvements that can wait.

---

## Risks

| Risk | Mitigation |
|------|-----------|
| `bd list` is too slow for PreToolUse hook | Cache bead state at session start, refresh on SessionStart and every 5 min |
| Beads file annotations become stale | Include "check Files: annotation" in Clavain's bead creation workflow |
| Sessions ignore CLAUDE.md conventions | F3 hook enforces; F4 makes conflicts visible early |
| Worktree branches diverge too far | Keep branches short-lived; merge on `bd close` |

## Open Questions

1. Should the conflict check hook also warn about *uncommitted* changes in the current working tree (not just bead-claimed files)?
2. Should we integrate Clash for real-time conflict detection, or is the Beads-based approach sufficient for 2-3 agents?
3. How should `internal/domain/` conflicts be handled — sequential beads (blocked-by) or explicit coordination messages?
