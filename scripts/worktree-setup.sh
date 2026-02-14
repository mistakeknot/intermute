#!/usr/bin/env bash
# Creates an isolated worktree for multi-agent work
set -euo pipefail

NAME="${1:?Usage: worktree-setup.sh <name>}"

# Validate name
if [[ ! "$NAME" =~ ^[a-zA-Z0-9_-]+$ ]]; then
    echo "ERROR: Name must contain only alphanumeric characters, hyphens, and underscores"
    exit 1
fi

# Resolve paths from repo root
REPO_ROOT=$(git rev-parse --show-toplevel)
WORKTREE_DIR="${REPO_ROOT}/../intermute-${NAME}"
BRANCH="work/${NAME}"

if [[ -d "$WORKTREE_DIR" ]]; then
    echo "Worktree already exists: $WORKTREE_DIR"
    exit 1
fi

git worktree add "$WORKTREE_DIR" -b "$BRANCH"

# Copy settings (create .claude dir first — it won't exist in a fresh worktree)
mkdir -p "$WORKTREE_DIR/.claude"
cp "${REPO_ROOT}/.claude/settings.local.json" "$WORKTREE_DIR/.claude/settings.local.json" 2>/dev/null || true

# Copy scripts directory for hooks
if [[ -d "${REPO_ROOT}/scripts" ]]; then
    cp -r "${REPO_ROOT}/scripts" "$WORKTREE_DIR/scripts"
fi

echo "Worktree created: $WORKTREE_DIR (branch: $BRANCH)"
echo ""
echo "Start a session:"
echo "  cd $WORKTREE_DIR && claude"
echo ""
echo "When done, merge and clean up:"
echo "  ${REPO_ROOT}/scripts/worktree-teardown.sh $NAME"
