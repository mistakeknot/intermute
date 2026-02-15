# Intermute Vision and Philosophy

Intermute is the backend coordination service for multi-agent systems in the Interverse constellation. It provides deterministic, project-scoped coordination for files, messages, and agent tasks using a small, HTTP-first API and real-time WebSocket eventing.

**Module:** `github.com/mistakeknot/intermute`  
**Go version:** `1.24`  
**Last updated:** `2026-02-15`  
**Service maturity:** `0.0.x` (development-stage, API shape is stable and operational)

## Why file coordination matters

Multi-agent work on the same repository quickly fails without explicit coordination, even when agents are using separate contexts. Intermute solves the conflict surface for file-level work by:

- Preventing overlapping edits through **advisory reservations** before agents claim working paths.
- Making coordination explicit through message routing, thread views, and read/ack statuses.
- Keeping all operations **project-scoped** so one team cannot accidentally observe or mutate another.

The value is not just avoiding collisions; it is reducing wasted effort. Agents spend less time reconciling duplicate work and more time advancing actual progress.

## What intermute provides

- **Agent lifecycle primitives**: register agents, heartbeat, discover peers.
- **Message coordination**: send/receive/inbox, threading, read/ack state.
- **File advisory locks**: reserve shared/exclusive path patterns with conflict checks.
- **Domain APIs**: specs, epics, stories, tasks, insights, sessions, CUJs.
- **Real-time delivery**: WebSocket streams for messages and domain events.
- **Resilience layer**: retries and circuit breakers around SQLite-backed persistence.

## Design philosophy

### 1) Advisory locks, not hard process locks

Reservations are advisory by design so coordination can be explicit and recoverable. Intermute enforces collision checks before creation and reports conflicts, but it does not assume every process in the world obeys intermute. This keeps the service effective in cooperative environments while avoiding single points of catastrophic blocking.

### 2) SQLite first, with explicit durability controls

SQLite is a pragmatic control plane for the service size and deployment model. Event sourcing and indexed projections are used to keep writes simple and queries efficient without adding distributed-state complexity.

### 3) Circuit breakers over brittle happy-path assumptions

I/O failures and lock contention are expected in concurrent agent workloads. Intermute treats resilience as a first-class layer: lock-aware retry, circuit breaker state transitions, and heartbeat-cleanup workflows keep failures recoverable rather than surprising.

### 4) Project as hard boundary

Almost every API operation is project-scoped (`?project=...` and composite keys in storage), keeping multi-tenant isolation explicit and accidental cross-talk low.

### 5) WebSocket as event surface, HTTP as command/control

Agents issue commands via HTTP and observe state changes through broadcast events by project/agent. This model avoids polling for common workflows and makes cooperative updates easier to process by consumers.

## Current risk envelope

Intermute does not replace filesystem locks in every runtime. It coordinates agents at the application layer and depends on participating services and agent clients to use it consistently for intended contention avoidance.

