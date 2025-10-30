#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <version> <ipk-directory>" >&2
  exit 1
fi

VERSION="$1"
IPK_DIR="$2"
BRANCH="feeds/xp2p"

if [ ! -d "$IPK_DIR" ]; then
  echo "IPK directory '$IPK_DIR' not found" >&2
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
WORKTREE_DIR="$REPO_ROOT/.worktrees/xp2p-feed"

if ! git ls-remote --exit-code origin "$BRANCH" >/dev/null 2>&1; then
  echo "Remote branch '$BRANCH' not found. Create it before running this script." >&2
  exit 1
fi

git fetch origin "$BRANCH":"$BRANCH"

if git worktree list | grep -q "$WORKTREE_DIR"; then
  git worktree remove --force "$WORKTREE_DIR"
fi

git worktree add "$WORKTREE_DIR" "$BRANCH" >/dev/null

cleanup() {
  git worktree remove --force "$WORKTREE_DIR"
}
trap cleanup EXIT

mkdir -p "$WORKTREE_DIR/packages"
cp "$IPK_DIR"/*.ipk "$WORKTREE_DIR/packages/"

opkg-make-index "$WORKTREE_DIR/packages" >"$WORKTREE_DIR/Packages"
gzip -f "$WORKTREE_DIR/Packages"

pushd "$WORKTREE_DIR" >/dev/null
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git add packages Packages Packages.gz

if git diff --cached --quiet; then
  echo "Feed is already up to date."
else
  git commit -m "xp2p ${VERSION}"
  git push origin HEAD:"$BRANCH"
fi
popd >/dev/null
