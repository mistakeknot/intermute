# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.1.0] - 2026-09-01

First tagged release. intermute has been running as an internal service;
this release makes the repo usable standalone.

### Added

- `LICENSE` (MIT), `CONTRIBUTING.md`, `SECURITY.md` (Contributor Covenant
  2.1), GitHub issue/PR templates.
- `intermute version` subcommand and a `--version` flag on the root
  command, both reporting the same build-time-settable version.
- `docs/api.md`: every REST + WebSocket route documented, including the
  window-identity routes (`/api/windows`) and the inbox-pokes routes
  that had no prior documentation. `scripts/check-route-docs.sh` gates
  documentation drift in CI.
- `examples/two-agents.sh`: a self-contained walkthrough of the core
  coordination flow (register, reserve, detect a conflict, negotiate by
  message, release, re-reserve) against a fresh server on a temp
  database.
- `SECURITY.md` and README § Auth now state the loopback trust boundary
  in plain words: any local process can act as any agent unless it
  presents that agent's registration token. `intermute.keys.yaml.example`
  documents the `allow_localhost_without_auth: false` shared-host
  opt-out.
- CI matrix now runs on Ubuntu and macOS, and gates on `gofmt`.

### Changed

- README rewritten for a first-time reader: what this is, why it's
  worth using, install, quick start, and a full route table.
- `agents/api-reference.md` moved to `docs/api.md`.

### Removed

- Internal planning artifacts not relevant to external contributors:
  `.auracoil/`, `.remote-source`, `PHILOSOPHY.md`, `docs/plans/`,
  `docs/research/`, `docs/solutions/`, `docs/PRD.md`, `docs/roadmap.md`,
  the self-referential `intermute` symlink at the repo root.
- Ecosystem jargon and personal filesystem paths from `AGENTS.md`,
  `docs/vision.md`, and the `agents/` guides.

[0.1.0]: https://github.com/mistakeknot/intermute/releases/tag/v0.1.0
