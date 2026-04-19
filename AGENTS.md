# AGENTS.md -- Intermute

L1 (core) multi-agent coordination and messaging service for the Demarch platform. Handles agent lifecycle (registration, heartbeats, contact policies), project-scoped messaging with threading and topic discovery, file reservations with glob-based conflict detection, and event sourcing of domain entities (specs, epics, stories, tasks, insights, sessions, CUJs). Central hub for multi-agent orchestration with REST + WebSocket + optional Unix socket delivery.

## Canonical References
1. [`PHILOSOPHY.md`](../../PHILOSOPHY.md) -- direction for ideation and planning decisions.
2. `CLAUDE.md` -- implementation details, architecture, testing, and release workflow.

## Quick Reference

| Item | Value |
|------|-------|
| Module | `github.com/mistakeknot/intermute` |
| Layer | L1 (core) |
| Port | 7338 |
| Start | `go run ./cmd/intermute serve` |
| Tests | `go test ./...` |
| Database | SQLite (intermute.db) |
| Auth config | intermute.keys.yaml (`INTERMUTE_KEYS_FILE` env var) |

## Topic Guides

| Topic | File | Covers |
|-------|------|--------|
| Architecture | [agents/architecture.md](agents/architecture.md) | Stack, request flow, database design (16 tables), authentication, directory structure |
| API Reference | [agents/api-reference.md](agents/api-reference.md) | All REST + WebSocket endpoints: agents, messaging, threads, reservations, domain CRUD |
| Data Model | [agents/data-model.md](agents/data-model.md) | Core types (Agent, Message, Event, Reservation), domain types (Spec, Epic, Story, Task, CUJ), contact policy |
| Live Transport | [docs/live-transport.md](docs/live-transport.md) | tmux injection flow, hooks, focus/policy rules, residual risks, feature-flag rollback |
| Resilience | [agents/resilience.md](agents/resilience.md) | ResilientStore, circuit breaker, retry, sweeper, Intercore coordination bridge |
| Conventions | [agents/conventions.md](agents/conventions.md) | Naming (handlers, requests, JSON structs), database patterns, error handling |
| CLI Reference | [agents/cli-reference.md](agents/cli-reference.md) | Commands, server flags, auth model (keys YAML), client environment variables |
| Operations | [agents/operations.md](agents/operations.md) | Gotchas (10 items), downstream dependencies, testing, router variants, SQLite gotchas |
