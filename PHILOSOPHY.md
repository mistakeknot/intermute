# intermute Philosophy

## Purpose
Multi-agent coordination service — the backend that makes agents aware of each other.

## North Star
Provide coordination primitives safe under concurrency: lock semantics, messaging, and auth must be reliable before optimization.

## Working Priorities
- Concurrency safety
- Reservation correctness
- Auth + messaging reliability

## Brainstorming Doctrine
1. Start from outcomes and failure modes, not implementation details.
2. Generate at least three options: conservative, balanced, and aggressive.
3. Explicitly call out assumptions, unknowns, and dependency risk across modules.
4. Prefer ideas that improve clarity, reversibility, and operational visibility.

## Planning Doctrine
1. Convert selected direction into small, testable, reversible slices.
2. Define acceptance criteria, verification steps, and rollback path for each slice.
3. Sequence dependencies explicitly and keep integration contracts narrow.
4. Reserve optimization work until correctness and reliability are proven.

## Decision Filters
- Does this reduce ambiguity for future sessions?
- Does this improve reliability without inflating cognitive load?
- Is the change observable, measurable, and easy to verify?
- Can we revert safely if assumptions fail?

## Evidence Base
- Brainstorms analyzed: 0
- Plans analyzed: 4
- Source confidence: artifact-backed (0 brainstorm(s), 4 plan(s))
- Representative artifacts:
  - `docs/plans/2026-01-25-intermute-auth-implementation-plan.md`
  - `docs/plans/2026-01-25-intermute-auth-ux-implementation-plan.md`
  - `docs/plans/2026-01-26-intermute-auth-ux-plan.md`
  - `docs/plans/2026-02-14-multi-session-coordination.md`
