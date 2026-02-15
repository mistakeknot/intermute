# intermute Codebase Exploration Report

**Date:** 2026-02-14  
**Scope:** Project structure, coordination mechanisms, agent parallelization challenges  
**Purpose:** Understand coordination requirements for multiple agents working simultaneously

---

## Executive Summary

intermute is a **real-time coordination and messaging service** for Autarch agents (~13,000 lines of Go). It uses SQLite as the append-only event store with materialized indexes, supports multi-tenancy via composite primary keys (project, id), and has WebSocket real-time delivery. The codebase has **no native CI/CD pipeline, no feature branches, and trunk-based development** (main only).

**Key Coordination Challenges for Multi-Agent Work:**
1. **SQLite concurrency limits** — Single-connection pool can cause SQLITE_BUSY under parallel writes
2. **Event sourcing atomicity** — Multi-table materializations (event → message → indexes) must be transactional
3. **Race conditions** — RWMutex in WebSocket hub and client need careful locking
4. **Project isolation** — Composite PKs (project, id) are enforced but no query-level filtering validation
5. **Test coordination** — 111 tests using shared intermute.db; in-memory SQLite creates separate DBs per connection
6. **Beads task tracking** — Issues tracked in `.beads/` SQLite with auto-sync to JSONL, no multi-agent coordination

---

## 1. Project Directory Structure

```
/root/projects/intermute/
├── cmd/intermute/              Entry point (179 LoC)
│   ├── main.go                 Wires store, auth, WS hub, HTTP service
│   ├── init.go                 Key generation for projects
│   └── serve.go                Server startup
├── client/                     Go SDK for agents (84 LoC)
│   ├── client.go               HTTP client wrapper
│   ├── domain.go               Domain API calls
│   ├── websocket.go            WS connection (RWMutex locking)
│   └── client_test.go
├── internal/                   13,000 LoC
│   ├── auth/                   Bearer token validation, keyring loading
│   ├── cli/                    Command initialization helpers
│   ├── core/                   Message, Agent, Event types
│   ├── glob/                   File reservation overlap detection
│   ├── http/                   REST handlers + tests (75.5% coverage)
│   ├── names/                  Culture ship name generator
│   ├── server/                 Server wiring
│   ├── storage/                Store interface + implementations
│   │   ├── sqlite/             SQLite implementation (race tests)
│   │   │   ├── schema.sql      DDL + indexes
│   │   │   ├── sqlite.go       Core store logic
│   │   │   ├── domain.go       Domain entity CRUD
│   │   │   ├── sqlite_test.go  Integration tests
│   │   │   ├── domain_test.go  Domain entity tests
│   │   │   └── race_test.go    --race flag tests (concurrent writes)
│   │   ├── storage.go          Interface + InMemory impl
│   │   └── storage_test.go
│   └── ws/                     WebSocket hub (RWMutex)
├── pkg/embedded/               Embedded server for Autarch
│   ├── server.go               StartServer() entry point
│   ├── client.go               Domain API calls
│   ├── domain.go               Domain types
│   ├── websocket.go            WS client
│   └── tests
├── docs/
│   ├── plans/                  Auth implementation plans (3 docs)
│   ├── research/               Analysis docs
│   └── solutions/database-issues/
│       └── silent-json-errors-sqlite-storage-20260211.md
├── .beads/                     Issue tracking (SQLite + JSONL)
│   ├── config.yaml             No explicit multi-repo config
│   ├── beads.db                Primary database
│   ├── issues.jsonl            Issue records
│   └── daemon.sock             RPC socket (single daemon per project)
├── .auracoil/                  GPT-5.2 Pro review system
│   ├── config.yaml             maxFiles: 50, maxTotalSize: 500KB
│   ├── generated/AGENTS.2026-02-05.md
│   ├── reviews/review-2026-02-07.json
│   └── state.json              Last reviewed commit: 2e9e98e
├── .serena/                    Serena LSP configuration
│   ├── project.yml             go language server
│   └── memories/               (empty as of Feb 2026)
├── .claude/                    Claude Code settings
│   └── settings.local.json     Permissions allow: go test, git, curl, pkill
├── go.mod                      Go 1.24, modernc.org/sqlite, nhooyr websocket
├── go.sum
├── .gitignore                  Beads config + *.db* + daemon files
├── CLAUDE.md                   Claude-specific settings (brief)
├── AGENTS.md                   Comprehensive documentation (261 lines)
└── README.md                   MVP, auth model, endpoints

Total Go files: 43 files, ~13,000 LoC
Test files: 19 test files, 2,322 LoC
```

---

## 2. Existing Configuration & Coordination Tools

### 2.1 `.claude/settings.local.json` (Claude Code Permissions)

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
  "prompt": "Before starting any work, run 'bd onboard' to understand the current project state and available issues."
}
```

**Analysis:** Permits git commits directly to main (no branch protection), grep/test/build commands. Prompt suggests using Beads for task discovery.

### 2.2 `.beads/` (Beads Issue Tracking)

- **Active daemon:** `daemon.sock` listening for RPC
- **Database:** SQLite (`beads.db`, `beads.db-shm`, `beads.db-wal`)
- **JSONL fallback:** `issues.jsonl` (11KB, ~100 issues as of Feb 14)
- **Config:** `config.yaml` (no explicit multi-repo or multi-daemon config)
- **RPC socket:** Single socket per project — only one daemon can run at a time

**Gotcha:** Daemon is single-threaded for RPC; if two Claude Code sessions try to use Beads simultaneously on the same project, second will block or get `EADDRINUSE`.

### 2.3 `.auracoil/` (GPT-5.2 Pro Review)

- **Last reviewed:** Feb 7, 2026, commit `2e9e98e`
- **Config:** `config.yaml` with `maxFiles: 50`, `maxTotalSize: 500KB`, `maxTokens: 100000`
- **Exclusions:** node_modules, .git, dist, __pycache__, vendor, coverage
- **Model:** `gpt-5.2-pro` with 600s timeout
- **State tracking:** `state.json` stores last reviewed commit to avoid re-analyzing
- **Artifacts:** `cache/`, `oracle/`, `reviews/`, `solutions/` directories

**Pattern:** Review state is tracked by commit hash. If two agents push commits simultaneously, Auracoil may not detect one or both.

### 2.4 `.serena/` (Serena LSP Configuration)

- **Project:** intermute
- **Language:** go (single)
- **Memories directory:** Present but empty
- **Read-only mode:** false
- **No excluded tools**

**Note:** Serena doesn't have native multi-agent coordination; it's per-session.

### 2.5 Git Configuration

- **Branch:** main (only branch)
- **Remote:** https://github.com/mistakeknot/intermute (fetch + push)
- **Workflow:** Trunk-based (direct commits to main, no feature branches)
- **Untracked:** `.remote-source`, `.serena/`, `intermute.keys.yaml` (auth keys)
- **Commit history:** 15 recent commits, avg commit message format `[category]: [description] — [details]`

**Coordination issue:** No branch-per-task isolation; all agents work on main. Merge conflicts and lost commits are possible if two agents push in rapid succession.

---

## 3. Documentation & AGENTS.md

### 3.1 AGENTS.md (261 lines)

**Comprehensive reference covering:**
- Architecture (event sourcing, materialized indexes)
- API endpoints (agents, messages, threads, domain entities)
- Database schema (events, messages, inbox_index, thread_index, domain tables)
- Authentication model (localhost bypass, bearer tokens, project-scoped keys)
- Conventions (handler/request/response naming, DB composites, error handling)
- Downstream dependencies (Autarch embeds pkg/embedded/)
- Testing patterns (httptest, WebSocket, race tests)

**Missing documentation:**
- Concurrency model (RWMutex strategies not documented)
- SQLite connection pooling (MaxOpenConns not mentioned)
- Beads integration (no guidance on using Beads across agents)
- Multi-agent coordination patterns
- Reservation/glob overlap detection (internal/glob/ not explained)

### 3.2 Other Documentation

| File | Purpose |
|------|---------|
| CLAUDE.md | Brief quick reference, links to AGENTS.md |
| README.md | MVP summary, auth model, endpoints |
| docs/plans/*.md | Auth implementation plans (3 docs, Jan 25-26) |
| docs/solutions/database-issues/ | Silent JSON error fix (Feb 11) |

**Gap:** No coordination or multi-agent documentation.

---

## 4. Test Structure & Coverage

### 4.1 Test Organization

**19 test files, 2,322 LoC, 111 test functions**

| Module | Tests | Coverage | Notes |
|--------|-------|----------|-------|
| internal/http | 11 files | 75.5% | handlers_agents, messages, threads, domain, reservations |
| internal/storage/sqlite | 3 files | 67.2% | sqlite, domain, race tests |
| internal/ws | 1 file | 79.3% | gateway tests |
| internal/auth | 2 files | (uncalculated) | bootstrap, middleware |
| internal/cli | 1 file | (uncalculated) | init tests |
| Other | 1 file | (uncalculated) | smoke test, names, storage |

### 4.2 Key Test Files

- **`internal/http/test_helpers_test.go`** — `testEnv` struct with `newTestEnv(t)`, httptest.NewServer setup
- **`internal/storage/sqlite/race_test.go`** — Tests with `go test -race` flag; sets `db.SetMaxOpenConns(1)` to avoid SQLITE_BUSY
- **`internal/storage/sqlite/sqlite_test.go`** — Integration tests for cursor, thread, migration logic
- **Handlers tests** — Use `NewRouter` or `NewDomainRouter`, issue requests to httptest.Server

### 4.3 Concurrency Testing

**Race flag:** Tests pass with `-race` (Go's race detector).  
**Connection pool:** `sqlite_test.go` uses in-memory SQLite (`:memory:`) which creates **separate DBs per connection** in the pool.  
**Single-connection mode:** `race_test.go` sets `SetMaxOpenConns(1)` to work around SQLITE_BUSY.

**Coordination issue:** Tests using `:memory:` and default pool settings (many connections) won't catch real concurrency bugs in production (single file-backed SQLite).

---

## 5. Code Structure & Dependencies

### 5.1 Core Dependencies (go.mod)

```
github.com/google/uuid v1.6.0           — UUID generation
github.com/spf13/cobra v1.10.2          — CLI framework
gopkg.in/yaml.v3 v3.0.1                 — YAML parsing (auth keys)
modernc.org/sqlite v1.29.0              — Pure Go SQLite (no CGO)
nhooyr.io/websocket v1.8.7              — WebSocket server
```

**No dependencies on:**
- Beads SDK (Beads is external tool, not imported)
- Auracoil (external review service)
- Serena (external LSP tool)
- Any ORM or database abstraction layers

### 5.2 Key Components

**Authentication:**
- `internal/auth/config.go` — Loads keyring from YAML
- `internal/auth/middleware.go` — HTTP middleware for bearer validation
- `internal/auth/bootstrap.go` — Creates dev key on startup if missing

**Storage:**
- `internal/storage/storage.go` — Interface + InMemory implementation
- `internal/storage/sqlite/sqlite.go` — File-backed SQLite, event sourcing
- `internal/storage/sqlite/schema.sql` — DDL with composite PKs (project, id)

**HTTP:**
- `internal/http/router.go` — Simple multiplexer (agents, messages, threads, domain)
- `internal/http/handlers_*.go` — Request/response handlers
- No middleware for request logging, tracing, or correlation IDs

**WebSocket:**
- `internal/ws/gateway.go` — Hub with RWMutex; broadcasts to (project, agent) subscribers
- `client/websocket.go` — Client-side WS (RWMutex for message queue)

**Globbing:**
- `internal/glob/overlap.go` — Overlap detection for file reservations (7.7 KB)
- Used to prevent two agents from reserving overlapping paths

---

## 6. Coordination Challenges Identified

### 6.1 Database Concurrency

**Problem:** SQLite has limited write concurrency.

- **Production setup:** File-backed DB with default Go pool (many open connections)
- **Default WAL mode:** Allows concurrent reads + one writer, but long transactions block writes
- **No explicit pooling:** `db.SetMaxOpenConns(1)` only set in race tests, not production
- **AppendEvent transactions:** Recently added (fix P0); multi-table writes now wrap in `BEGIN`/`COMMIT`

**Coordination impact:** If two agents send messages simultaneously, second may get SQLITE_BUSY. No exponential backoff or retry logic in current code.

### 6.2 Event Sourcing & Atomicity

**Problem:** Events materialized to multiple indexes; partial writes leave DB inconsistent.

- **Schema:** events (PK: cursor) → materializes to messages, inbox_index, thread_index, message_recipients
- **Fix applied (Feb 11):** Transactions wrapping AppendEvent now enforce atomicity
- **JSON marshaling:** Fixed silent `_ = json.Marshal/Unmarshal` errors (critical bug #20260211)

**Coordination impact:** Even with transactions, if an agent crashes mid-write, WAL recovery may be incomplete. No saga/compensating-transaction pattern implemented.

### 6.3 WebSocket Hub Race Conditions

**Problem:** RWMutex in `internal/ws/gateway.go` may deadlock or lose messages.

- **Pattern:** Subscribe/unsubscribe modify map while Broadcast iterates
- **Locking:** RWMutex with separate locks for subscriptions and broadcaster
- **Tests:** Pass with `-race` but test coverage is 79.3%

**Coordination impact:** High-frequency WebSocket subscribers (many agents) may see race conditions not caught by current test suite.

### 6.4 Project Isolation

**Problem:** Composite PKs (project, id) enforced at schema but no query-level filtering validation.

- **Pattern:** All queries must include `WHERE project = ?` filter
- **Handlers:** Accept `project` param from request, pass to storage layer
- **Missing:** No validation that authenticated key's project matches request project

**Coordination impact:** If Agent A from project-x somehow gets Agent B's API key from project-y, A could read/write B's messages.

### 6.5 Beads Daemon Coordination

**Problem:** Single daemon per project; two Claude Code sessions can't access Beads simultaneously.

- **RPC socket:** `.beads/bd.sock` (one socket per project)
- **Lock file:** `.beads/daemon.lock` (single writer)
- **JSONL fallback:** Auto-import from `issues.jsonl` if database is stale, but lock contention possible

**Coordination impact:** Two agents trying to create issues in parallel will get socket errors. Manual `bd sync` may be needed to merge changes.

### 6.6 Git Workflow (Trunk-Based, No Protection)

**Problem:** All work directly on main; no branch protection or pre-commit checks.

- **Current:** Commits are pushed directly to main
- **Missing:** Pre-push hooks, required status checks, code review workflow
- **Test CI:** No GitHub Actions or similar automation

**Coordination impact:** Two agents can push conflicting commits; second push may clobber first or fail with merge error. No "who pushed what and when" audit trail.

### 6.7 Auracoil Review State

**Problem:** Review state tracked by commit hash; simultaneous pushes may confuse state tracking.

- **State:** `state.json` stores `lastReviewedCommit`
- **Logic:** Auracoil skips re-analysis if commit == lastReviewedCommit
- **Issue:** If two agents push commits A and B simultaneously, Auracoil may see A, analyze, then see B pushed and lose track

**Coordination impact:** Some commits may skip review or trigger redundant analyses.

### 6.8 Test Database Contention

**Problem:** Tests use shared `intermute.db` file; in-memory SQLite creates separate DBs per connection.

- **In-memory pattern:** `:memory:` databases are per-connection, so parallel test runners create separate DBs
- **File-backed pattern:** Concurrent test runs on same `intermute.db` will trigger SQLITE_BUSY
- **No test isolation:** Tests don't clean up data between runs; subsequent tests may hit stale state

**Coordination impact:** Running `go test ./...` in parallel (`-p 8`) will deadlock or fail unpredictably on SQLite tests.

---

## 7. Synchronization Mechanisms (Current)

### 7.1 What Exists

1. **Git:** Single main branch, no protection
2. **Beads:** Issue tracking with JSONL export (auto-sync to git optional)
3. **Auracoil:** Review state persisted in state.json
4. **.beads/daemon.lock:** Single-writer lock on daemon process
5. **SQLite WAL:** Write-ahead logging for crash recovery

### 7.2 What's Missing

- **Task coordination:** No shared task list visible to agents
- **Conversation history:** Claude Code sessions are per-user, not shared
- **Correlation IDs:** No request tracing or agent action tracing
- **Locking mechanisms:** No distributed locks for shared resources
- **Merge conflict resolution:** Manual on Git conflicts
- **Audit logging:** No who/what/when trail for agent actions
- **Pre-commit validation:** No linters, formatters, or tests run before push

---

## 8. Recent Development Activity

### 8.1 Recent Commits (Last 15)

```
c9965df test: comprehensive test suite — 111 tests, 75.5% HTTP coverage, race-safe
e73bfff fix(P2): 7 improvements — query logging, glob extraction, context.Context, thread denormalization
1b383b3 fix(P1): 7 bugs — sentinel errors, GROUP BY, WS cleanup, RWMutex, inbox LIMIT, WS auth, TOCTOU
16de7d8 fix(P0): 4 critical bugs — JSON error handling, tx atomicity, 409 conflicts, flaky test
d6b4a4e docs: apply Auracoil review — 8 GPT-5.2 Pro suggestions
2e9e98e fix(P1-SEC): enforce reservation ownership on release
8509987 fix(P1-SEC): X-Forwarded-For spoofing bypass in auth middleware
68de450 fix(P0): file reservation glob overlap detection with transactional safety
02b390c feat(domain): expand domain APIs and client
de005a6 docs(plans): add auth ux implementation notes
345e18a feat: extend client with reservation and inbox count methods
d81e918 feat: add inbox counts endpoint
c896bd4 feat: add file reservations for agent coordination
d04e5ae feat: add per-recipient tracking for messages
0099469 feat: add Subject, CC, BCC, Importance, AckRequired to Message
```

**Pattern:** 
- Frequent bug fixes (P0-P2 priority labels)
- Recent focus on concurrency (RWMutex, thread denormalization, race tests)
- Security fixes (auth bypass, X-Forwarded-For, reservation ownership)
- Features: file reservations (for agent coordination), domain APIs

### 8.2 Git Status

- **Current branch:** main
- **Commits ahead of origin:** 4
- **Untracked:** `.remote-source`, `.serena/`, `intermute.keys.yaml`

---

## 9. Configuration Files Summary

| File | Purpose | Multi-Agent Aware |
|------|---------|-------------------|
| `.claude/settings.local.json` | Claude Code permissions | No; allows direct git push |
| `.beads/config.yaml` | Beads daemon config | No; no multi-repo or multi-daemon settings |
| `.auracoil/config.yaml` | Auracoil review config | No; state tracked per commit |
| `.auracoil/state.json` | Review state persistence | Partially; can lose track if multiple commits pushed |
| `.serena/project.yml` | Serena LSP config | No; per-session |
| `.gitignore` | Git exclusions | Partial; includes Beads daemon files |
| `go.mod` | Dependencies | N/A |
| `AGENTS.md` | Comprehensive docs | No; doesn't cover multi-agent coordination |
| `CLAUDE.md` | Claude-specific settings | No; brief, no coordination guidance |

---

## 10. Recommendations for Multi-Agent Coordination

### 10.1 Immediate (Critical)

1. **Add SQLite connection pooling config** → Set `MaxOpenConns(1)` in production to match test setup
2. **Add request correlation IDs** → Generate per-request UUID for tracing agent actions
3. **Implement retry logic** → Exponential backoff for SQLITE_BUSY errors
4. **Document WebSocket RWMutex strategy** → Clarify when locks are held and for how long

### 10.2 Short-term (High Priority)

1. **Create `.clavain/` config** → Document multi-agent coordination rules, shared resources, locking
2. **Add Beads coordination guide** → Explain daemon socket, lock contention, workarounds
3. **Extend AGENTS.md** → Add concurrency section with SQLite patterns, RWMutex strategy, race test guidance
4. **Add CI/CD pre-push checks** → Run tests, linters, formatters before accepting commits
5. **Implement branch protection** → Require passing tests + code review for main merges

### 10.3 Medium-term (Nice to Have)

1. **Distributed coordination layer** → Redis locks for global resources (file reservations, etc.)
2. **Saga/compensating-transaction pattern** → For multi-table writes that might partially fail
3. **Audit logging** → Log all agent actions (create, update, delete) with timestamp, agent ID, correlation ID
4. **Shared task queue** → Decouple from Beads; use intermute's own messaging for task distribution
5. **Agent health checks** → Heartbeat validation, automatic zombie agent cleanup

---

## 11. Conclusion

intermute is a **well-structured event-sourced messaging service** with solid fundamentals (SQLite, WebSocket, REST + async delivery). Recent commits show a focus on **concurrency and security fixes**.

**For multi-agent coordination:**
- **Bottlenecks:** SQLite write concurrency, Beads daemon single-writer RPC, Git trunk-based (no isolation)
- **Gaps:** No coordination docs, no distributed locking, no request tracing, no merge conflict resolution strategy
- **Opportunities:** File reservation overlap detection (glob) is novel; expand to other shared resources; Beads JSONL fallback can serve as heartbeat mechanism

The codebase is ready for **parallel development with documentation and tooling** — not architectural changes needed yet.

---

**Report compiled:** 2026-02-14 by Claude Code exploration tool  
**Analysis scope:** Full codebase walk, config files, test structure, coordination mechanisms
