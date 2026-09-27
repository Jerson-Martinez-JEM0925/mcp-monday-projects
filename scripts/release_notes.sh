#!/usr/bin/env bash
# Print the CHANGELOG section for a release tag and verify the tag is
# consistent with the code before anything is published.
#
# Usage: scripts/release_notes.sh v0.3.0 > release-notes.md
#
# Fails (exit 1) when:
#   - the tag is not vMAJOR.MINOR.PATCH,
#   - internal/mcpserver/server.go does not declare the same Version,
#   - CHANGELOG.md has no "## [MAJOR.MINOR.PATCH]" section, or it is empty.
set -euo pipefail

tag="${1:-}"
if [[ ! "$tag" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  echo "error: tag must look like v1.2.3, got '${tag}'" >&2
  exit 1
fi
version="${BASH_REMATCH[1]}"
root="$(cd "$(dirname "$0")/.." && pwd)"

code_version="$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$root/internal/mcpserver/server.go")"
if [[ "$code_version" != "$version" ]]; then
  echo "error: tag ${tag} but internal/mcpserver/server.go has Version = \"${code_version}\"" >&2
  exit 1
fi

notes="$(awk -v heading="## [${version}]" '
  index($0, heading) == 1 { found = 1; next }
  found && /^## \[/ { exit }
  found { print }
' "$root/CHANGELOG.md")"
if [[ -z "${notes//[[:space:]]/}" ]]; then
  echo "error: CHANGELOG.md has no non-empty '## [${version}]' section" >&2
  exit 1
fi
printf '%s\n' "$notes"
