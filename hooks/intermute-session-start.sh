#!/usr/bin/env bash
# SessionStart hook: register this tmux pane as the agent's current live target.

set -euo pipefail

project="${INTERMUTE_PROJECT:-}"
agent="${INTERMUTE_AGENT:-}"
window_uuid="${INTERMUTE_WINDOW_UUID:-}"
token="${INTERMUTE_REGISTRATION_TOKEN:-}"
url="${INTERMUTE_URL:-http://127.0.0.1:7338}"

if [[ -z "$project" || -z "$agent" || -z "$window_uuid" || -z "$token" ]]; then
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "intermute-session-start: jq not on PATH; skipping window upsert" >&2
    exit 0
fi

if ! command -v tmux >/dev/null 2>&1; then
    exit 0
fi

tmux_target="$(tmux display-message -p '#S:#W.#P' 2>/dev/null || true)"
if [[ -z "$tmux_target" ]]; then
    exit 0
fi

payload="$(
    jq -n \
        --arg project "$project" \
        --arg window_uuid "$window_uuid" \
        --arg agent_id "$agent" \
        --arg tmux_target "$tmux_target" \
        --arg registration_token "$token" \
        '{
            project: $project,
            window_uuid: $window_uuid,
            agent_id: $agent_id,
            tmux_target: $tmux_target,
            registration_token: $registration_token
        }'
)"

if ! curl -fsS \
    -X POST \
    -H 'Content-Type: application/json' \
    -d "$payload" \
    "$url/api/windows" >/dev/null; then
    echo "intermute-session-start: window upsert failed at $url" >&2
fi
