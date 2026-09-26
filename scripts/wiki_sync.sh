#!/usr/bin/env bash
# Mirror docs/ into the repository Wiki.
#
# Pages are built by scripts/wiki_pages.py (one flat page per docs/**/*.md,
# links resolved). Pages that no longer exist in docs/ are removed from the
# Wiki, so docs/ stays the single source of truth.
#
# Requires: REPO (owner/name); GH_TOKEN with contents: write unless DRY_RUN=1.
# Usage:    REPO=owner/name scripts/wiki_sync.sh
#           DRY_RUN=1 REPO=owner/name scripts/wiki_sync.sh   # build ./wiki_out only
set -euo pipefail

: "${REPO:?REPO=owner/name is required}"
out_dir="${OUT_DIR:-wiki_out}"

python3 "$(dirname "$0")/wiki_pages.py" "$REPO" "$out_dir"

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  exit 0
fi

: "${GH_TOKEN:?GH_TOKEN is required to push to the Wiki}"
wiki_dir="$(mktemp -d)"
if ! git clone --quiet "https://x-access-token:${GH_TOKEN}@github.com/${REPO}.wiki.git" "$wiki_dir" 2>/dev/null; then
  # GitHub creates the Wiki git repository only after its first page is saved
  # in the web UI. Until then there is nothing to push to: report and pass.
  echo "::notice::Wiki not initialised yet. Create any page once in the repository's Wiki tab, then re-run this workflow."
  exit 0
fi

rsync -a --delete --exclude='.git' "$out_dir/" "$wiki_dir/"
cd "$wiki_dir"
git config user.name 'github-actions[bot]'
git config user.email 'github-actions[bot]@users.noreply.github.com'
git add -A
if git diff --cached --quiet; then
  echo "Wiki already up to date."
  exit 0
fi
git commit --quiet -m "docs: sync from ${GITHUB_SHA:-local} [skip ci]"
git push --quiet origin HEAD
echo "Wiki updated."
