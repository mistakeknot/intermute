# Research: Multi-Agent Coordination Patterns for AI Coding Agents

> **Date**: 2026-02-14
> **Scope**: Practical patterns for multiple AI coding agents working on the same codebase without conflicts
> **Applicability**: Intermute project (Go/SQLite codebase) and similar projects

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Claude Code Agent Teams](#2-claude-code-agent-teams)
3. [Git-Based Coordination Patterns](#3-git-based-coordination-patterns)
4. [File-Based Task Coordination](#4-file-based-task-coordination)
5. [SQLite Concurrent Access Patterns](#5-sqlite-concurrent-access-patterns)
6. [Beads and Issue Trackers for Work Partitioning](#6-beads-and-issue-trackers-for-work-partitioning)
7. [Third-Party Orchestration Tools](#7-third-party-orchestration-tools)
8. [Comparison Matrix](#8-comparison-matrix)
9. [Recommended Patterns for Intermute](#9-recommended-patterns-for-intermute)
10. [Sources](#10-sources)

---

## 1. Executive Summary

The multi-agent AI coding landscape has rapidly matured in early 2026, converging on a few dominant coordination patterns. The fundamental insight across all approaches is: **LLMs perform worse as context expands, so multi-agent systems formalize specialization, letting each agent focus deeply rather than juggling competing concerns.**

Three tiers of solutions exist:

| Tier | Approach | Complexity | Best For |
|------|----------|-----------|----------|
| **Low** | File ownership conventions + `CLAUDE.md` rules | Minimal setup | 2-3 agents, non-overlapping work |
| **Medium** | Git worktrees + task tracker (Beads/tick-md) | Moderate setup | 3-6 agents, feature-parallel work |
| **High** | Agent Teams / Vibe Kanban / ccswarm | Significant setup | 5+ agents, complex coordinated work |

The most important single principle: **partition work by file ownership, not by task type**. Two agents editing the same file will always create conflicts regardless of coordination infrastructure.

---

## 2. Claude Code Agent Teams

### 2.1 Overview

Released February 5, 2026 alongside Claude Opus 4.6, Agent Teams is Anthropic's first-party solution for multi-agent coordination. It is experimental and must be explicitly enabled.

**Enable via settings.json:**
```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

### 2.2 Architecture

An agent team consists of four components:

| Component | Role |
|-----------|------|
| **Team Lead** | Main Claude Code session that creates the team, spawns teammates, and coordinates work |
| **Teammates** | Separate Claude Code instances, each with its own context window |
| **Task List** | Shared list of work items stored as JSON files on disk |
| **Mailbox** | Inter-agent messaging system for communication |

Key architectural facts:
- Each teammate is a full Claude Code session loading the same project context (CLAUDE.md, MCP servers, skills)
- Teammates do NOT inherit the lead's conversation history
- The only coordination channels are task files on disk and SendMessage -- there is no shared memory
- Teams and tasks are stored locally at `~/.claude/teams/{team-name}/config.json` and `~/.claude/tasks/{team-name}/`

### 2.3 Task System

Tasks are stored as individual JSON files:

```json
{
  "id": "1",
  "subject": "Implement cursor-based pagination for inbox",
  "description": "Detailed instructions for the agent...",
  "status": "pending",
  "owner": "agent-name"
}
```

Task states: `pending` -> `in_progress` -> `completed`

**Claiming mechanism**: Teammates discover work through `TaskList()` calls, then use `TaskUpdate()` to claim a task. File locking prevents race conditions where two teammates try to claim the same task simultaneously.

**Dependency tracking**: Tasks can block other tasks. When a blocking task completes, downstream tasks automatically unblock. Teammates self-claim the next available unblocked task when they finish.

### 2.4 Communication

Four message types:
- `message` -- direct message to one teammate
- `broadcast` -- send to all teammates (use sparingly, costs scale with team size)
- `shutdown_request` -- ask a teammate to stop
- `shutdown_response` -- teammate's response to shutdown request

Messages are delivered automatically to recipients; the lead does not need to poll.

### 2.5 Delegate Mode

Activated with `Shift+Tab`, restricts the lead to coordination-only tools. The lead cannot write code, run tests, or do implementation work -- only manage tasks, communicate, and review output. This prevents the common problem of the lead implementing tasks itself instead of delegating.

### 2.6 Display Modes

- **In-process**: All teammates run in the main terminal. Use `Shift+Up/Down` to select teammates.
- **Split panes**: Each teammate gets its own tmux or iTerm2 pane for simultaneous visibility.

### 2.7 Quality Gates via Hooks

Two hooks enforce standards:
- `TeammateIdle` -- runs when a teammate is about to go idle. Exit code 2 sends feedback and keeps the teammate working.
- `TaskCompleted` -- runs when a task is being marked complete. Exit code 2 prevents completion with feedback.

### 2.8 Best Practices from Official Docs

1. **Give teammates enough context** in the spawn prompt (they don't inherit conversation history)
2. **Size tasks appropriately** -- 5-6 tasks per teammate keeps everyone productive
3. **Avoid file conflicts** -- break work so each teammate owns different files
4. **Start with research/review** before attempting parallel implementation
5. **Monitor and steer** -- check in on progress, redirect failing approaches
6. **Use plan approval** for risky work -- teammates plan in read-only mode until the lead approves

### 2.9 Known Limitations

- No session resumption with in-process teammates
- Task status can lag (teammates sometimes fail to mark tasks complete)
- One team per session, no nested teams
- All teammates inherit the lead's permission mode
- Split panes require tmux or iTerm2 (not VS Code terminal)

---

## 3. Git-Based Coordination Patterns

### 3.1 Git Worktrees (The Standard Approach)

Git worktrees are the de facto standard for isolating parallel AI agent work. Each agent gets its own checked-out copy of the repository sharing the same `.git` directory.

**Setup:**
```bash
# Create worktrees for each agent
git worktree add ../project-agent-1 -b agent-1/feature-a
git worktree add ../project-agent-2 -b agent-2/feature-b
git worktree add ../project-agent-3 -b agent-3/feature-c
```

**Advantages:**
- Complete filesystem isolation between agents
- Each agent can install dependencies, build, and test independently
- Branches merge through standard git workflow
- No custom tooling required

**Disadvantages:**
- Requires bootstrapping each worktree (dependency installation, build artifacts)
- Creates risk of merge conflicts discovered only at merge time
- Agents are blind to each other's changes during development
- Disk space scales linearly with worktree count

### 3.2 GitButler's Hook-Based Approach (No Worktrees)

GitButler eliminates worktrees by using Claude Code's lifecycle hooks to automatically organize simultaneous sessions into separate branches within a single working directory.

**How it works:**
1. When Claude Code runs multiple sessions, GitButler receives notifications through hooks about file edits and chat completions
2. Each session automatically gets its own branch based on session ID
3. Changes are automatically assigned to their corresponding branch
4. When a chat finishes, GitButler commits with context-driven messages

**Advantages over worktrees:**
- Single working directory, no duplication of dependencies or builds
- One commit per chat round with preserved context
- Automatic branch management with no manual setup per session

### 3.3 Clash: Proactive Conflict Detection

Clash is an open-source CLI tool that detects potential merge conflicts between worktrees before they become expensive to resolve.

**How it works:**
1. Uses `git merge-tree` (via the `gix` library) to perform three-way merges in memory
2. Discovers all worktrees, finds merge bases, simulates merges
3. Reports which files would conflict between specific worktree pairs
4. Entirely non-destructive -- never touches git state

**Integration with Claude Code hooks:**
```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Write|Edit|MultiEdit",
      "hooks": [{ "type": "command", "command": "clash check" }]
    }]
  }
}
```

When Claude attempts to edit a file, Clash checks for conflicts across all worktrees. If conflicts exist, the agent receives a prompt before proceeding.

**CLI usage:**
```bash
clash check src/main.go    # Check single file (exit 0=clean, 2=conflict)
clash status               # Full conflict matrix across all worktrees
clash status --json        # Machine-readable for automation
clash watch                # Real-time monitoring TUI
```

### 3.4 Hierarchical Agent Roles (Cursor's Lesson)

Cursor tried and failed with two naive approaches:
- **Equal-status agents with file locking**: Agents held locks too long; 20 agents slowed to throughput of 2-3
- **Optimistic concurrency control**: Agents became risk-averse, avoided hard tasks to minimize conflict risk

The pattern that worked uses three distinct roles:
- **Planners**: Continuously explore the codebase and create tasks
- **Workers**: Execute assigned tasks without coordinating with each other; push changes when done
- **Judges**: Determine whether to continue at each cycle end

This hierarchical structure -- planners managing workers, judges evaluating progress -- emerged as the only pattern that scales.

---

## 4. File-Based Task Coordination

### 4.1 tick-md: Markdown as Coordination Hub

tick-md turns a single `TICK.md` file into a full multi-agent task coordination system, built on Git with MCP integration.

**Core mechanism:**
- All task state lives in a single Markdown file tracked by Git
- Agents claim tasks through the MCP server, which locks the file to prevent concurrent edit conflicts
- Every state change (claim, status change, completion) becomes a Git commit with timestamp and author
- Tasks can block other tasks; when a blocker completes, dependent tasks automatically unblock

**Installation and usage:**
```bash
npm install -g tick-md
tick-md init              # Initialize TICK.md in project
tick-md add "task desc"   # Add a task
tick-md claim 1           # Agent claims task 1
tick-md done 1            # Mark complete, unblock dependents
```

**MCP integration**: Agents connect via Model Context Protocol, interacting with the Markdown file through standardized tool calls rather than custom parsing.

**Edge cases encountered in production:**
- Race conditions in simultaneous claims (solved by file locking)
- Circular dependency references
- OS-specific locking behavior differences
- Special character handling in YAML frontmatter

### 4.2 CLAUDE.md Convention-Based Coordination

The simplest approach: use `CLAUDE.md` to declare file ownership boundaries.

```markdown
## Agent Boundaries
- Agent 1 owns: `internal/http/` (HTTP handlers, middleware)
- Agent 2 owns: `internal/storage/` (SQLite, migrations)
- Agent 3 owns: `internal/ws/` (WebSocket hub, connections)
- Shared files: `internal/domain/` -- coordinate before editing
```

**Advantages:** Zero tooling, immediately understandable, works with any AI agent.
**Disadvantages:** No enforcement mechanism, requires discipline, breaks down with tightly coupled code.

### 4.3 Lock File Pattern

A lightweight file-based locking pattern using the filesystem:

```bash
# Agent claims a file by creating a lock
echo "agent-1" > .locks/internal/http/handlers.go.lock

# Before editing, check for locks
if [ -f .locks/path/to/file.lock ]; then
  echo "File claimed by $(cat .locks/path/to/file.lock)"
fi

# Release when done
rm .locks/internal/http/handlers.go.lock
```

This pattern can be formalized with a small CLI tool or MCP server. The `.locks/` directory should be gitignored but can be tracked for audit purposes.

---

## 5. SQLite Concurrent Access Patterns

### 5.1 Relevance to Multi-Agent Coordination

SQLite's concurrency model is directly relevant in two ways:
1. **As a coordination database**: Using SQLite itself to track agent task claims and status
2. **As a shared resource**: Multiple agents may need to modify the same SQLite database (e.g., Intermute's data store)

### 5.2 SQLite Concurrency Fundamentals

**Rollback journal mode** (default): Many readers, but writes require exclusive lock over entire database. Only one writer at a time; all others block.

**WAL (Write-Ahead Logging) mode**: Allows concurrent readers alongside a single writer. New writes go to a write-ahead log; readers see consistent snapshots. Still serializes multiple writers.

```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;  -- Wait up to 5s for lock
```

### 5.3 Lock Acquisition Behavior

SQLite uses hardcoded exponential backoff for lock acquisition:
```
1ms, 2ms, 5ms, 10ms, 15ms, 20ms, 25ms, 25ms, 25ms, 50ms, 50ms, 100ms...
```
Then 100ms repeatedly until timeout. There is no FIFO guarantee -- under contention, starvation is possible.

**Practical timeout recommendations by contention level:**
- Low (2-5 agents): 5-10 second timeout
- Medium (5-20 agents): 20-30 second timeout
- High (20+ agents): 60+ second timeout

Under 1000x concurrency, SkyPilot measured p50 write latency of 2.3 seconds with only 0.13% exceeding 60 seconds.

### 5.4 BEGIN CONCURRENT (Experimental)

Available in SQLite's `begin-concurrent` branch (not mainline), this enables multiple concurrent write transactions using optimistic page-level locking.

**How it works:**
1. `BEGIN CONCURRENT` defers database locking until `COMMIT`
2. Multiple transactions can proceed simultaneously
3. At commit time, the system checks if pages read by this transaction were modified by concurrent transactions
4. If no page-level conflicts: commit succeeds
5. If conflicts: returns `SQLITE_BUSY_SNAPSHOT` and the transaction must retry

**Key limitation:** Conflict detection is at the page level, not row level. Two transactions updating different rows on the same page will conflict even though there is no logical conflict.

### 5.5 Patterns for Multi-Agent SQLite Access

**Pattern 1: Single-Writer Coordinator**
Route all writes through a single coordinator process. Other agents send write requests via IPC/HTTP. Eliminates contention entirely.

```
Agent 1 --\
Agent 2 ----> Coordinator (single writer) ----> SQLite
Agent 3 --/
```

**Pattern 2: WAL Mode with Retry**
Enable WAL mode, set generous busy_timeout, and implement application-level retry with exponential backoff.

```go
// Go implementation pattern
db.SetMaxOpenConns(1)  // Serialize writes at connection level
_, err := db.Exec("PRAGMA journal_mode=WAL")
_, err = db.Exec("PRAGMA busy_timeout=30000")
```

**Pattern 3: Partitioned Writes**
Each agent writes to different tables or uses a unique prefix/partition key. No contention because agents never touch the same data.

**Pattern 4: Event Queue Table**
Agents append to a single events table (append-only, no conflicts), and a single coordinator materializes events into the actual data tables.

```sql
-- Each agent appends events
INSERT INTO agent_events (agent_id, event_type, payload, created_at)
VALUES (?, ?, ?, ?);

-- Single coordinator processes events in order
SELECT * FROM agent_events WHERE processed = 0 ORDER BY created_at;
```

### 5.6 Go-Specific Considerations

The pure Go SQLite driver (`modernc.org/sqlite` or `github.com/mattn/go-sqlite3`) has specific constraints:
- `mattn/go-sqlite3` explicitly states it does not support concurrent access
- `modernc.org/sqlite` (the pure Go driver used by Intermute) handles concurrency through Go's `database/sql` connection pool
- Setting `db.SetMaxOpenConns(1)` serializes all access through a single connection, preventing SQLITE_BUSY but creating a bottleneck
- PRAGMAs (WAL, busy_timeout) only apply to the connection they are executed on, which is problematic with pooled connections

**Recommendation for Intermute:** The current architecture (event sourcing pattern, append to events table, materialize to indexes) is already well-suited for multi-agent scenarios. Multiple agents can append events safely with WAL mode, and a single process handles materialization.

---

## 6. Beads and Issue Trackers for Work Partitioning

### 6.1 Beads (`bd`) Architecture

Beads is a git-backed issue tracker designed specifically for AI-assisted coding workflows. Its three-part architecture makes it uniquely suited for multi-agent coordination.

**Data flow:**
```
bd create  -->  SQLite (beads.db)  -->  JSONL export  -->  Git commit
                                                              |
                                                              v
Git pull   -->  JSONL import  -->  SQLite (beads.db)  -->  Agent queries
```

**Key commands for multi-agent coordination:**

```bash
bd create "Implement pagination" --epic auth-epic  # Create task under epic
bd ready                                           # List tasks with all dependencies met
bd ready --assignee agent-1                        # Filter by agent
bd assign 42 agent-2                               # Assign task to specific agent
bd status 42 in_progress                           # Claim task
bd status 42 done                                  # Mark complete, unblock dependents
bd list --json                                     # Machine-readable for automation
```

### 6.2 Multi-Agent Coordination with Beads

**Work partitioning pattern:**
1. Human or planner agent creates an epic with subtasks
2. Each subtask gets explicit file-ownership annotations in its description
3. Agents query `bd ready --assignee <self>` to find claimable work
4. Agent sets status to `in_progress`, preventing other agents from claiming
5. On completion, dependent tasks automatically become ready

**Example epic structure for Intermute:**
```
Epic: "Add rate limiting"
  Task 1: "Add rate limiter middleware" (owner: agent-1, files: internal/http/middleware.go)
  Task 2: "Add rate limit storage" (owner: agent-2, files: internal/storage/sqlite/ratelimit.go)
  Task 3: "Add rate limit tests" (owner: agent-3, files: internal/http/ratelimit_test.go)
    blocked_by: [1, 2]
  Task 4: "Add rate limit config" (owner: agent-1, files: cmd/intermute/config.go)
```

### 6.3 Beads + Vibe Kanban Integration

There is active development to integrate Beads with Vibe Kanban (GitHub issue #1394), combining Beads' task dependency tracking with Vibe Kanban's agent orchestration and review capabilities. The vision is: Beads handles planning (task dependencies, work discovery, context persistence), while Vibe Kanban provides orchestration and review.

### 6.4 Using Beads as Coordination Database

Because Beads stores data in SQLite, it can serve double duty as both issue tracker and coordination database:

```bash
# Agent startup: find available work
TASKS=$(bd ready --json | jq -r '.[0].id')

# Claim first available task
bd status $TASKS in_progress
bd assign $TASKS $(hostname)-agent-1

# On completion
bd status $TASKS done

# Check what's now unblocked
bd ready --json
```

This pattern treats Beads as a distributed work queue with dependency ordering, which is exactly the primitive needed for multi-agent coordination.

---

## 7. Third-Party Orchestration Tools

### 7.1 ccswarm

**Repository:** https://github.com/nwiizo/ccswarm
**Language:** Rust (crate available on crates.io)

A multi-agent orchestration system using Claude Code with Git worktree isolation and specialized AI agents.

**Key features:**
- Specialized agent pools: Frontend, Backend, DevOps, QA
- Actor Model for agent communication (message-passing, no shared state)
- Git worktree isolation per agent
- Session persistence with 93% token reduction via conversation history
- Terminal UI for real-time monitoring
- Auto-create functionality for task delegation

**Architecture patterns:**
- Type-State Pattern for compile-time state validation
- Channel-Based Orchestration for message-passing without locks
- Iterator Pipelines for zero-cost task processing abstractions

### 7.2 Agent-MCP

**Repository:** https://github.com/rinadelph/Agent-MCP

A framework for multi-agent systems using Model Context Protocol.

**Core architecture:**
- Shared persistent knowledge graph for unified project context
- Task decomposition into linear chains assigned to specialized agents
- Conflict prevention through explicit task ordering and dependency management
- Asynchronous inter-agent communication via MCP messaging

**Agent specialization model:**
```
Database Agent: table creation -> indexing -> session management
API Agent:      POST endpoints -> validation -> security
Frontend Agent: form components -> context providers -> integration
Test Agent:     unit tests -> integration tests -> e2e
```

### 7.3 Vibe Kanban

**Repository:** https://github.com/BloopAI/vibe-kanban
**Language:** TypeScript (52%) + Rust (45.8%)

A kanban board for orchestrating AI coding agents.

**Key features:**
- Supports Claude Code, Gemini CLI, Cursor CLI, Amp, OpenAI Codex, Qwen Code, and more
- Git worktree isolation for parallel execution
- Centralized MCP configuration for multiple agents
- Remote SSH deployment capability
- Task status tracking through kanban interface

**Workflow:**
1. Developer defines work items on kanban board
2. Tasks assigned to specific agents
3. Each agent works in isolated worktree
4. Monitor execution status in real-time
5. Review outputs before integration

### 7.4 agenttools/worktree

**Repository:** https://github.com/agenttools/worktree

CLI tool for managing Git worktrees with GitHub issues and Claude Code integration. Bridges issue tracking with worktree-based agent isolation.

---

## 8. Comparison Matrix

### 8.1 Coordination Approaches

| Approach | Isolation | Conflict Prevention | Task Management | Setup Cost | Token Cost |
|----------|-----------|-------------------|-----------------|------------|------------|
| CLAUDE.md conventions | None | Convention only | Manual | None | Baseline |
| Git worktrees (manual) | Full filesystem | Post-hoc merge | Manual | Low | Baseline |
| Git worktrees + Clash | Full filesystem | Proactive detection | Manual | Low | Baseline |
| GitButler hooks | Branch-level | Automatic branching | Automatic | Low | Baseline |
| tick-md | None | File lock on TICK.md | MCP-integrated | Low | Baseline |
| Beads (`bd`) | None | Status/assignment | Git-native, dependency-aware | Low | Baseline |
| Claude Agent Teams | Context window | Task claiming + file lock | Built-in shared list | Medium | 3-8x |
| ccswarm | Git worktrees | Actor model + worktrees | Built-in | High | 3-8x |
| Vibe Kanban | Git worktrees | Worktree + kanban | Kanban board | High | 3-8x |
| Agent-MCP | Task partitioning | Knowledge graph + ordering | MCP-native | High | 3-8x |

### 8.2 When to Use What

| Scenario | Recommended Approach |
|----------|---------------------|
| 2 agents, non-overlapping files | CLAUDE.md conventions + manual worktrees |
| 2-3 agents, same repo, some overlap | Beads + Clash for conflict detection |
| 3-5 agents, feature-parallel work | Agent Teams or manual worktrees + Beads |
| 5+ agents, complex interdependent work | Vibe Kanban or ccswarm |
| Research/review (no code changes) | Agent Teams with delegate mode |
| Debugging with competing hypotheses | Agent Teams with adversarial setup |
| Single agent, sequential tasks | Subagents (not agent teams) |

---

## 9. Recommended Patterns for Intermute

### 9.1 Current Architecture Assessment

Intermute's architecture is well-suited for multi-agent work:
- **Clean package boundaries**: `internal/http/`, `internal/storage/sqlite/`, `internal/ws/`, `internal/domain/`
- **Event sourcing pattern**: Append-only events table naturally handles concurrent writes
- **Composite primary keys**: Multi-tenant isolation prevents cross-agent data conflicts
- **Existing Beads integration**: Project already uses `bd` for issue tracking

### 9.2 Recommended Tier 1: Minimal Setup (Today)

For 2-3 agents working in parallel:

1. **Add file ownership rules to CLAUDE.md:**
```markdown
## Agent Boundaries
- Agent A: `internal/http/` handlers and middleware
- Agent B: `internal/storage/sqlite/` storage layer
- Agent C: `internal/ws/` WebSocket hub and connections
- Shared: `internal/domain/` -- coordinate via Beads before editing
```

2. **Use Beads for task coordination:**
```bash
bd create "Add rate limiting middleware" --assign agent-a
bd create "Add rate limit storage" --assign agent-b
bd create "Add rate limit tests" --blocked-by 1,2
```

3. **Run each agent in a separate git worktree:**
```bash
git worktree add ../intermute-agent-a -b work/agent-a
git worktree add ../intermute-agent-b -b work/agent-b
```

### 9.3 Recommended Tier 2: Enhanced Coordination

For 3-5 agents or work touching shared files:

1. **Install Clash for proactive conflict detection:**
```bash
cargo install clash-cli
```

2. **Add Clash hook to `.claude/settings.json`:**
```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Write|Edit|MultiEdit",
      "hooks": [{ "type": "command", "command": "clash check" }]
    }]
  }
}
```

3. **Enable Agent Teams for complex features:**
```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

### 9.4 Recommended Tier 3: Full Orchestration

For 5+ agents or complex cross-cutting features:

1. **Use Vibe Kanban or ccswarm** for full orchestration with worktree management
2. **Integrate Beads with the orchestrator** for dependency-aware task scheduling
3. **Implement the Planner-Worker-Judge pattern** from Cursor's experience:
   - Planner agent: explores codebase, creates Beads tasks with dependencies
   - Worker agents: claim and execute tasks in isolated worktrees
   - Judge agent: evaluates completion criteria, triggers next planning cycle

### 9.5 SQLite-Specific Recommendations

For Intermute's SQLite database under multi-agent scenarios:

1. **Enable WAL mode** in the connection setup (if not already):
```go
db.Exec("PRAGMA journal_mode=WAL")
db.Exec("PRAGMA busy_timeout=30000")
```

2. **Keep `MaxOpenConns(1)`** for write serialization (already in test configuration)

3. **Leverage event sourcing**: The existing append-only events table naturally handles concurrent appends under WAL mode. Multiple agents can safely append events as long as materialization is serialized.

4. **For coordination metadata**: Use a separate SQLite database (or Beads) for agent task claims and status, avoiding contention with the application database.

---

## 10. Sources

### Official Documentation
- [Claude Code Agent Teams Documentation](https://code.claude.com/docs/en/agent-teams) -- Anthropic's official guide to Agent Teams
- [SQLite: Begin Concurrent](https://www.sqlite.org/src/doc/begin-concurrent/doc/begin_concurrent.md) -- SQLite's experimental concurrent write transactions
- [SQLite: File Locking and Concurrency](https://sqlite.org/lockingv3.html) -- SQLite v3 locking architecture

### Tools and Frameworks
- [ccswarm](https://github.com/nwiizo/ccswarm) -- Multi-agent orchestration with worktree isolation (Rust)
- [Agent-MCP](https://github.com/rinadelph/Agent-MCP) -- Multi-agent coordination via Model Context Protocol
- [Clash](https://github.com/clash-sh/clash) -- Merge conflict detection across git worktrees
- [Vibe Kanban](https://github.com/BloopAI/vibe-kanban) -- Kanban board for AI coding agent orchestration
- [agenttools/worktree](https://github.com/agenttools/worktree) -- Git worktree management with GitHub issues integration
- [tick-md](https://purplehorizons.io/blog/tick-md-multi-agent-coordination-markdown) -- Markdown-based multi-agent task coordination
- [Beads](https://github.com/steveyegge/beads) -- Git-native issue tracker for AI coding agents
- [Beads Viewer](https://github.com/Dicklesworthstone/beads_viewer) -- Graph-aware TUI for Beads with dependency DAG visualization

### Articles and Guides
- [Managing Multiple Claude Code Sessions Without Worktrees](https://blog.gitbutler.com/parallel-claude-code) -- GitButler's hook-based approach
- [Building ccswitch: Managing Multiple Claude Code Sessions](https://www.ksred.com/building-ccswitch-managing-multiple-claude-code-sessions-without-the-chaos/) -- Session management tool
- [Claude Code Agent Teams: Orchestrate Multiple Sessions in Parallel](https://medium.com/coding-nexus/claude-code-agent-teams-orchestrate-multiple-claude-sessions-in-parallel-98eb6d14513e) -- Practical orchestration guide
- [From Tasks to Swarms: Agent Teams in Claude Code](https://alexop.dev/posts/from-tasks-to-swarms-agent-teams-in-claude-code/) -- Technical internals of Agent Teams
- [Claude Code Swarms](https://addyosmani.com/blog/claude-code-agent-teams/) -- Addy Osmani's patterns and best practices
- [Git Worktrees for Parallel AI Coding Agents](https://devcenter.upsun.com/posts/git-worktrees-for-parallel-ai-coding-agents/) -- Upsun Developer Center guide
- [Parallel Workflows: Git Worktrees and the Art of Managing Multiple AI Agents](https://medium.com/@dennis.somerville/parallel-workflows-git-worktrees-and-the-art-of-managing-multiple-ai-agents-6fa3dc5eec1d) -- Practical workflow patterns
- [AI Coding Agents in 2026: Coherence Through Orchestration, Not Autonomy](https://mikemason.ca/writing/ai-coding-agents-jan-2026/) -- Strategic perspective
- [Abusing SQLite to Handle Concurrency](https://blog.skypilot.co/abusing-sqlite-to-handle-concurrency/) -- SkyPilot's SQLite concurrency patterns
- [Beads: A Git-Friendly Issue Tracker for AI Coding Agents](https://betterstack.com/community/guides/ai/beads-issue-tracker-ai-agents/) -- Better Stack's Beads guide
- [Introducing Beads: A Coding Agent Memory System](https://steve-yegge.medium.com/introducing-beads-a-coding-agent-memory-system-637d7d92514a) -- Steve Yegge's introduction
- [Beads: Distributed Task Management for AI Agents](https://peterwarnock.com/blog/posts/beads-distributed-task-management/) -- Distributed coordination patterns
- [Vibe Kanban: Manage AI Coding Agents in Parallel](https://byteiota.com/vibe-kanban-manage-ai-coding-agents-in-parallel/) -- Practical Vibe Kanban guide
- [Claude Code Swarm Orchestration Skill](https://gist.github.com/kieranklaassen/4f2aba89594a4aea4ad64d753984b2ea) -- Complete multi-agent coordination reference
