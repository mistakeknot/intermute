#!/usr/bin/env bash
# Shows in-progress work for session awareness
set -euo pipefail

echo "=== intermute: Active Work ==="
BEADS=$(bd list --status=in_progress 2>/dev/null || echo "")
if [[ -z "$BEADS" ]]; then
    echo "No in-progress work. All packages available."
    exit 0
fi

echo "$BEADS"
echo ""
echo "Check 'bd ready' for available tasks."
