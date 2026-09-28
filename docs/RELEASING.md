# Releasing

Releases follow [Semantic Versioning](https://semver.org/). A release is a
`vX.Y.Z` tag on `main`; `.github/workflows/release.yaml` does the rest.

The `1.x` line is the stable contract. Tool names, schemas, documented
environment variables, access-level semantics and structured error fields are
covered by regression tests. Removing or renaming one requires a major version.
The release process does not claim Member-level permissions have been tested:
Monday personal tokens are owned by the individual user, and a Member must
create their own token for that validation.

## What the workflow does

1. `scripts/release_notes.sh vX.Y.Z` refuses the release unless the tag
   matches `Version` in `internal/mcpserver/server.go` and `CHANGELOG.md` has
   a non-empty `## [X.Y.Z]` section. That section becomes the release notes.
2. Builds the runtime image for `linux/amd64` and `linux/arm64`.
3. Pushes it to GHCR as `ghcr.io/jersonmartinez/mcp-monday-projects` with the
   tags `X.Y.Z`, `X.Y` and `latest`.
4. Publishes the GitHub release for the tag.

## Steps

1. In a PR: move the `[Unreleased]` entries of `CHANGELOG.md` under a new
   `## [X.Y.Z] — <title>` heading and set `const Version = "X.Y.Z"` in
   `internal/mcpserver/server.go`. Check locally:

   ```bash
   bash scripts/release_notes.sh vX.Y.Z
   ```

2. Merge the PR once every check is green.
3. Tag the merge commit on `main` and push the tag:

   ```bash
   git fetch origin && git tag -a vX.Y.Z origin/main -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. Watch the **Release** workflow; when it is green, confirm the image:

   ```bash
   docker pull ghcr.io/jersonmartinez/mcp-monday-projects:X.Y.Z
   docker run --rm -e MONDAY_API_TOKEN=x ghcr.io/jersonmartinez/mcp-monday-projects:X.Y.Z </dev/null
   ```

The first time a package is published, GHCR may create it as **private**. To
allow anonymous `docker pull`, open the package on GitHub → *Package
settings* → *Change visibility* → *Public* (the repository is public).

## If a release fails

The workflow publishes nothing before the version check passes, and the
GitHub release is created last, so a failed run can be fixed and re-run from
the Actions tab for the same tag. Do not move or re-push a tag that already
produced a published image; release a new patch version instead.
