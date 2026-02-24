# Task 2 Execution: Add `capabilities` field to interlock-register.sh

**Plan:** `/root/projects/Demarch/docs/plans/2026-02-22-agent-capability-discovery.md`
**Task:** Task 2 — Add `capabilities` field to interlock-register.sh
**Status:** Implemented (not committed)

## What Was Done

Modified `/home/mk/projects/Demarch/interverse/interlock/scripts/interlock-register.sh` to:

1. **Added capability extraction block** (lines 34-42) — reads per-agent capability files from `~/.config/clavain/capabilities-${AGENT_NAME}.json`
2. **Updated POST payload** (lines 44-54) — includes `--argjson capabilities "$CAPABILITIES"` and `capabilities: $capabilities` in the jq object

## Changes Made

### File: `interverse/interlock/scripts/interlock-register.sh`

**Before (lines 34-43):**
```bash
# POST to intermute /api/agents
RESPONSE=$(intermute_curl POST "/api/agents" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
        --arg id "claude-${SESSION_ID:0:8}" \
        --arg name "$AGENT_NAME" \
        --arg project "$PROJECT" \
        --arg session_id "$SESSION_ID" \
        '{id: $id, name: $name, project: $project, session_id: $session_id}')" \
    2>/dev/null) || exit 1
```

**After (lines 34-54):**
```bash
# Extract capabilities from per-agent capability file (written by each plugin's session hook)
CAPABILITIES="[]"
CAPS_FILE="${HOME}/.config/clavain/capabilities-${AGENT_NAME}.json"
if [[ -f "$CAPS_FILE" ]]; then
    AGENT_CAPS=$(jq -c '.' "$CAPS_FILE" 2>/dev/null)
    if [[ -n "$AGENT_CAPS" ]] && [[ "$AGENT_CAPS" != "null" ]]; then
        CAPABILITIES="$AGENT_CAPS"
    fi
fi

# POST to intermute /api/agents
RESPONSE=$(intermute_curl POST "/api/agents" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
        --arg id "claude-${SESSION_ID:0:8}" \
        --arg name "$AGENT_NAME" \
        --arg project "$PROJECT" \
        --arg session_id "$SESSION_ID" \
        --argjson capabilities "$CAPABILITIES" \
        '{id: $id, name: $name, project: $project, session_id: $session_id, capabilities: $capabilities}')" \
    2>/dev/null) || exit 1
```

## Design Decisions

1. **Per-agent file path pattern:** `~/.config/clavain/capabilities-${AGENT_NAME}.json` — this follows the XDG convention and avoids the `CLAUDE_PLUGIN_ROOT` problem (which points to interlock's cache, not the calling plugin).

2. **`jq -c` (compact), not `-r` (raw):** Raw output strips JSON array brackets, turning `["a","b"]` into bare strings. Compact output preserves valid JSON for `--argjson`.

3. **`--argjson` for capabilities:** Unlike `--arg` (which treats input as a string), `--argjson` parses the value as JSON, so the array is embedded directly into the payload object rather than being string-escaped.

4. **Default `"[]"` when no file exists:** Backward compatible — agents without capability files register with an empty capabilities array. The intermute server already handles empty/null capabilities in `capabilities_json`.

5. **Placement after PROJECT detection block:** The capability extraction logically belongs after agent identity (name + project) is established, since it depends on `$AGENT_NAME`.

## Verification Checklist

- [x] Capability extraction placed after PROJECT detection block (line 32)
- [x] Uses `jq -c` (compact output), not `-r`
- [x] Uses `--argjson` for capabilities (passes as JSON, not string)
- [x] Default `"[]"` when no capability file exists
- [x] File path uses `${AGENT_NAME}` variable
- [x] Both `-n` (non-empty) and `!= "null"` guards on jq output
- [x] `2>/dev/null` on jq to suppress parse errors from malformed files
- [x] No commit created (as instructed)

## How It Fits the Pipeline

```
Plugin session start hook (Task 4)
  → writes ~/.config/clavain/capabilities-<name>.json

interlock-register.sh (THIS TASK)
  → reads capability file
  → sends capabilities in POST payload to intermute

intermute /api/agents (Task 1)
  → stores capabilities_json in SQLite
  → supports ?capability= filter on GET

interlock list_agents MCP tool (Task 3)
  → passes capability param to intermute
  → agents can discover peers by capability
```

## File Locations

- Modified: `/home/mk/projects/Demarch/interverse/interlock/scripts/interlock-register.sh`
- Plan: `/root/projects/Demarch/docs/plans/2026-02-22-agent-capability-discovery.md`
