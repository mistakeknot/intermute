# Brainstorm: Clavain Integration for Multi-Session Coordination in intermute

> **Date**: 2026-02-14
> **Topic**: How to prevent multiple Claude Code sessions from stepping on each other
> **Status**: Brainstorm complete

---

## The Problem

When multiple Claude Code sessions work on intermute simultaneously, several types of collisions occur:

1. **File conflicts**: Two sessions edit the same file, creating merge conflicts or lost changes
2. **Test interference**: Parallel `go test` runs compete for SQLite file locks
3. **Git state corruption**: Trunk-based commits to `main` can race and overwrite each other
4. **Duplicate work**: Sessions don't know what others are working on, wasting tokens
5. **Beads contention**: The `bd.sock` daemon is single-writer, blocking concurrent issue updates

## Why intermute Is Well-Suited for Multi-Agent Work

Before diving into solutions, it's worth noting that intermute's architecture already has natural partition boundaries:

| Package | Responsibility | Independence |
|---------|---------------|--------------|
| `internal/http/` | HTTP handlers, middleware, routing | High — mostly self-contained |
| `internal/storage/sqlite/` | Database layer, queries, migrations | High — pure data access |
| `internal/ws/` | WebSocket hub, connections, subscriptions | Medium — shares domain types |
| `internal/domain/` | Domain entities (specs, epics, stories, tasks) | Low — shared by all layers |
| `cmd/intermute/` | Main entry point, config | Low — rarely needs changes |

The event sourcing pattern (append to events table, materialize to indexes) is naturally concurrent-safe for writes. The composite primary keys (project, id) provide built-in multi-tenant isolation.

## Exploration: What Already Exists

### Clavain's Current Coordination Primitives

Clavain already has several relevant mechanisms:

- **Beads phase tracking**: brainstorm → strategize → plan → execute → ship gates prevent skipping steps
- **HANDOFF.md**: Session-end hook creates handoff files for the next session
- **Session sentinels**: `/tmp/clavain-*.lock` files prevent duplicate hook triggers
- **Work discovery**: `lib-discovery.sh` scans for open beads and ranks them by priority

**What's missing**:
- No awareness of *other active sessions*
- No file-level ownership or claiming
- No conflict detection before writes
- HANDOFF.md is sequential (one session to the next), not parallel (multiple sessions at once)

### Claude Code Agent Teams

The experimental Agent Teams feature (released Feb 5, 2026) addresses this directly:
- Team Lead coordinates, Workers implement in parallel
- Shared task list with claiming mechanism
- `Shift+Tab` delegate mode prevents lead from implementing
- `TeammateIdle` and `TaskCompleted` hooks enforce quality gates

**Trade-off**: Agent Teams runs all agents in one terminal session. This is great for coordinated bursts but doesn't help with *independent* sessions that happen to overlap.

### Git Worktrees + Clash

The de facto industry standard for agent isolation:
- Each agent gets its own checked-out copy (shared `.git`)
- Clash proactively detects conflicts across worktrees before they happen
- Can be wired into Claude Code's `PreToolUse` hook to warn before conflicting edits

**Trade-off**: Requires per-worktree bootstrapping. intermute is Go, so `go build` is fast, but each worktree still needs its own test database.

## Three Tiers of Integration

### Tier 1: Convention-Based (Zero Tooling)

Add file ownership boundaries to CLAUDE.md:

```markdown
## Multi-Session Coordination
- Session A: `internal/http/` — handlers and middleware
- Session B: `internal/storage/sqlite/` — storage layer
- Session C: `internal/ws/` — WebSocket hub
- Shared zone: `internal/domain/` — coordinate via Beads before editing
```

Use Beads assignee field to partition work:
```bash
bd create --title="Add rate limiter" --assign=session-a
```

**Pros**: Works today, no tooling changes, self-documenting
**Cons**: No enforcement, relies on session discipline, breaks down with shared files

### Tier 2: Beads-Driven Work Queue

Treat Beads as a distributed work queue:

1. **Planner session** creates an epic with dependency-linked subtasks
2. Each subtask's description includes `Files: internal/http/ratelimit.go`
3. Worker sessions run `bd ready` to find claimable work
4. `bd update <id> --status=in_progress` acts as a claim
5. Dependency ordering ensures tasks that touch shared code run sequentially

Enhance with Clavain hooks:
- **SessionStart**: Auto-run `bd ready --json`, present claimable tasks
- **PreToolUse (Write/Edit)**: Check if the file being edited belongs to any in-progress bead assigned to *another* session
- **SessionEnd**: Auto-close completed beads, create HANDOFF.md with file-level diff summary

**Pros**: Natural extension of existing workflow, Beads already tracks dependencies
**Cons**: File ownership is advisory (in descriptions), no real-time conflict detection

### Tier 3: Worktrees + Agent Teams + Clash

Full isolation with proactive conflict detection:

1. **Git worktrees** for filesystem isolation (each session owns a worktree)
2. **Clash** hook detects conflicts before writes
3. **Agent Teams** for coordinated parallel work within a single feature
4. **Beads** for cross-feature coordination and dependency tracking

```bash
# Setup
git worktree add ../intermute-ws-refactor -b work/ws-refactor
git worktree add ../intermute-pagination -b work/pagination

# Each Claude Code session runs in its own worktree
cd ../intermute-ws-refactor && claude
cd ../intermute-pagination && claude

# Clash monitors for conflicts
clash watch  # TUI showing conflict matrix
```

**Pros**: Maximum isolation, proactive conflict detection, scales to 5+ sessions
**Cons**: Per-worktree setup cost, trunk-based dev constraint (our CLAUDE.md says commit to main), merge conflicts deferred to integration

## Key Design Questions

### Q1: Worktrees vs. Trunk-Based Dev?

Our CLAUDE.md says "commit directly to main." Worktrees create branches. These are in tension.

**Options**:
- A) Relax the rule: allow short-lived branches for multi-agent work, merge back to main
- B) Keep trunk-based: use file ownership + Beads to prevent conflicts in a single checkout
- C) Hybrid: use worktrees for *the work*, but merge to main immediately on completion (no long-lived branches)

**Leaning toward C** — worktrees provide isolation during work, but the branch lives only as long as the task. `bd close` triggers merge to main.

### Q2: How Granular Should File Ownership Be?

- Package-level (`internal/http/` → Agent A) is coarse but simple
- File-level (`handlers.go` → Agent A, `middleware.go` → Agent B) is precise but verbose
- Function-level is impractical

**Leaning toward package-level** with Beads descriptions for finer guidance. The natural package boundaries in intermute already provide good isolation.

### Q3: What's the Minimum Viable Integration?

If we could only do ONE thing, what gives the most value?

**Beads-driven work partitioning with file annotations.** This requires zero new tooling — just discipline in how we create beads. Each bead's description lists the files it touches. Sessions check `bd list --status=in_progress` before editing shared files.

### Q4: Should the intermute Server Itself Help?

intermute is a messaging system. Could it serve as the coordination channel?

- Sessions could post to a `#coordination` project/channel
- "I'm editing `internal/http/handlers.go`" messages provide real-time awareness
- Other sessions see the message before editing the same file

**Interesting but circular** — we'd need intermute running to coordinate work on intermute. Fine for steady-state development, not for bootstrap.

## Concrete Recommendations

### Immediate (This Week)

1. **Add multi-session coordination section to CLAUDE.md** with package-level ownership rules
2. **Establish Beads conventions**: every task bead includes `Files:` annotation in description
3. **Add a PreToolUse hook** that checks `bd list --status=in_progress --json` for file conflicts before Write/Edit

### Near-Term (This Month)

4. **Integrate Clash** for proactive conflict detection across worktrees
5. **Create a `coordination.sh` script** that sets up worktrees, starts sessions, and monitors for conflicts
6. **Enhance Clavain's SessionStart hook** to show active sessions and their claimed files

### Future (When Needed)

7. **Agent Teams integration** for complex cross-cutting features
8. **Self-hosting coordination on intermute** once the server is stable enough
9. **Planner-Worker-Judge pattern** with dedicated planning and review agents

## What We're NOT Doing (And Why)

- **Full orchestration framework** (ccswarm, Vibe Kanban): intermute is a small project. The overhead exceeds the benefit until we routinely have 5+ agents.
- **Custom MCP server for coordination**: Agent Teams will likely subsume this. Building our own is premature.
- **File-level locking**: Too granular, too much friction. Package-level ownership with advisory Beads annotations is the right granularity.
- **Database-level coordination**: SQLite's WAL mode + MaxOpenConns(1) already handles data-layer concurrency. The coordination problem is at the *code editing* level, not the data level.

## Summary

The sweet spot for intermute is **Tier 2: Beads-driven work partitioning** with a path to **Tier 3 worktrees** when needed. This builds on infrastructure we already have (Beads, Clavain hooks) and doesn't fight our trunk-based dev workflow.

The key insight is that intermute's clean package boundaries already provide natural agent ownership zones. We just need to formalize them and add lightweight coordination through Beads annotations and a conflict-check hook.
