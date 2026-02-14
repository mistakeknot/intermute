#!/usr/bin/env bash
# PreToolUse hook: check if target file is claimed by another in-progress bead
# Exit 0 = proceed (advisory warning), Exit 2 = block with message
# Claude Code hooks receive JSON on stdin with .tool_input containing parameters

set -euo pipefail

# Read tool input from stdin (Claude Code hook API)
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.path // empty')
if [[ -z "$FILE_PATH" ]]; then
    exit 0  # Can't determine file, don't block
fi

# Normalize to relative path
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
FILE_PATH="${FILE_PATH#${REPO_ROOT}/}"

# Get in-progress beads
BEADS_OUTPUT=$(bd list --status=in_progress 2>/dev/null || echo "")
if [[ -z "$BEADS_OUTPUT" ]]; then
    exit 0  # No in-progress beads, no conflicts
fi

# Extract target directory (with trailing slash to prevent partial matches)
TARGET_DIR="$(dirname "$FILE_PATH")/"

while IFS= read -r line; do
    # Parse bead ID (portable ERE, no PCRE)
    BEAD_ID=$(echo "$line" | grep -oE '[A-Za-z]+-[a-z0-9]+' | head -1)
    [[ -z "$BEAD_ID" ]] && continue

    # Get bead details
    BEAD_DETAIL=$(bd show "$BEAD_ID" 2>/dev/null) || continue

    # Check for Files: annotation (not anchored — bd show may indent)
    FILES_LINE=$(echo "$BEAD_DETAIL" | grep -i "Files:" || echo "")
    [[ -z "$FILES_LINE" ]] && continue

    # Fixed-string match to avoid regex injection from file paths
    if echo "$FILES_LINE" | grep -qiF -e "$TARGET_DIR" -e "$FILE_PATH"; then
        TITLE=$(echo "$BEAD_DETAIL" | head -1)
        echo "WARNING: File '$FILE_PATH' may conflict with in-progress bead $BEAD_ID ($TITLE)"
        echo "Check: bd show $BEAD_ID"
        echo "Proceeding anyway (advisory warning)."
        exit 0  # Advisory only — exit 0 to not block
    fi
done <<< "$BEADS_OUTPUT"

exit 0
