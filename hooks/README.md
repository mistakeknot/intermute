# Intermute Hooks

These scripts wire Claude hook events into Intermute's live-transport inbox flow.

## Files

- `intermute-session-start.sh`: SessionStart hook. If running inside tmux, it registers the current pane via `POST /api/windows`.
- `intermute-peer-inbox.sh`: PreToolUse hook. It prints unread deferred pokes so Claude sees them as next-turn context.
- `intermute-peer-inbox_test.sh`: Simple shell regression test for the PreToolUse hook contract.

## Environment

- `INTERMUTE_PROJECT`: project name
- `INTERMUTE_AGENT`: agent ID
- `INTERMUTE_WINDOW_UUID`: stable window UUID for this session
- `INTERMUTE_REGISTRATION_TOKEN`: registration token returned by agent registration
- `INTERMUTE_URL`: optional, defaults to `http://127.0.0.1:7338`

`intermute-peer-inbox.sh` uses `INTERMUTE_PROJECT`, `INTERMUTE_AGENT`, and optional `INTERMUTE_URL`.

`intermute-session-start.sh` uses all of the above and requires `jq`, `curl`, and `tmux` on `PATH`.

## Install

Configure your Claude hooks to call the shipped scripts directly from this directory:

- `SessionStart` -> `core/intermute/hooks/intermute-session-start.sh`
- `PreToolUse` -> `core/intermute/hooks/intermute-peer-inbox.sh`

## Verify

Run:

```bash
bash core/intermute/hooks/intermute-peer-inbox_test.sh
bash -n core/intermute/hooks/intermute-peer-inbox.sh
bash -n core/intermute/hooks/intermute-session-start.sh
```

To verify a deployed copy matches the tracked version:

```bash
sha256sum core/intermute/hooks/intermute-peer-inbox.sh core/intermute/hooks/intermute-session-start.sh
sha256sum /path/to/installed/intermute-peer-inbox.sh /path/to/installed/intermute-session-start.sh
```
