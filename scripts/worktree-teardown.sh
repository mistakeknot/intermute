#!/usr/bin/env bash
# Merges worktree branch to main and removes the worktree
# MUST be run from the main working tree (not from inside a worktree)
set -euo pipefail

NAME="${1:?Usage: worktree-teardown.sh <name>}"
REPO_ROOT=$(git rev-parse --show-toplevel)
WORKTREE_DIR="${REPO_ROOT}/../intermute-${NAME}"
BRANCH="work/${NAME}"

if [[ ! -d "$WORKTREE_DIR" ]]; then
    echo "Worktree not found: $WORKTREE_DIR"
    exit 1
fi

# Ensure we're running from the main working tree
MAIN_WORKTREE=$(git worktree list --porcelain | head -1 | sed 's/worktree //')
if [[ "$REPO_ROOT" != "$MAIN_WORKTREE" ]]; then
    echo "ERROR: Run this from the main working tree ($MAIN_WORKTREE), not a worktree"
    exit 1
fi

# Check for uncommitted changes in worktree
if [[ -n "$(git -C "$WORKTREE_DIR" status --porcelain)" ]]; then
    echo "ERROR: Uncommitted changes in $WORKTREE_DIR"
    echo "Commit or stash before teardown."
    exit 1
fi

# Check for uncommitted changes in main tree
if [[ -n "$(git status --porcelain)" ]]; then
    echo "ERROR: Uncommitted changes in main working tree"
    echo "Commit or stash before merging."
    exit 1
fi

# Verify we're on main
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "main" ]]; then
    echo "ERROR: Main working tree is on branch '$CURRENT_BRANCH', expected 'main'"
    exit 1
fi

git merge "$BRANCH" --no-edit
echo "Merged $BRANCH into main."

# Clean up
git worktree remove "$WORKTREE_DIR"
git branch -d "$BRANCH"
echo "Removed worktree and branch."
