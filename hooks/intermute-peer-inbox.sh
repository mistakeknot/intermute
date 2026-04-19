#!/usr/bin/env bash
# PreToolUse hook: surface deferred intermute peer pokes into Claude context.

set -euo pipefail

project="${INTERMUTE_PROJECT:-}"
agent="${INTERMUTE_AGENT:-}"
url="${INTERMUTE_URL:-http://127.0.0.1:7338}"

if [[ -z "$project" || -z "$agent" ]]; then
    exit 0
fi

if ! command -v intermute >/dev/null 2>&1; then
    exit 0
fi

if ! output="$(intermute inbox --unread-pokes \
    --agent="$agent" \
    --project="$project" \
    --url="$url" \
    --mark-surfaced 2>/dev/null)"; then
    exit 0
fi

if [[ -z "$output" ]]; then
    exit 0
fi

printf '%s\n' "$output"
