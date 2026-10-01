# Changelog

All notable changes are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [1.0.0] — Stable contract

### Changed

- **Stable v1 contract:** tool names, schemas, access levels, structured errors,
  and documented environment variables are now protected by a schema digest
  regression test. Breaking changes require a major version.
- **Access policy:** removed the ambiguous `MCP_READ_ONLY` compatibility
  variable. Use `MCP_ACCESS_LEVEL=read|write|full` explicitly.
- **Documentation:** clarified that archive retention is controlled by monday,
  and recorded that live smoke validation used an administrator token. Member
  permissions still require a separate member-owned token and have not been
  validated.

## [Unreleased]

### Added

- **Unreadable-column warnings:** monday omits columns the token cannot view
  from `column_values`, so a restricted column looked identical to an empty
  one. `create_item`, `update_item_column_values`, `set_item_status`,
  `set_item_date`, `assign_item_people`, `column_distribution` and
  `board_summary` now return an optional `warnings` list naming written or
  reported columns absent from the response. This additive output field
  changes the stable schema digest; no input or existing field changed.

## [0.3.0] — Operational parity

### Added

- **Usage and troubleshooting guides (#27):** `docs/USAGE.md` walks through
  the tools by task (connection, discovery, reads, validated writes, bulk
  plan → apply, archive vs. delete, collaboration, reports, templates);
  `docs/TROUBLESHOOTING.md` lists every startup, policy, validation and
  rate-limit message the server emits, captured from the real binary, with
  its fix.
- **Release workflow (#28):** a `vX.Y.Z` tag checks that the tag matches the
  code `Version` and a CHANGELOG section (`scripts/release_notes.sh`), builds
  `linux/amd64` + `linux/arm64`, pushes `ghcr.io/jersonmartinez/mcp-monday-projects`
  (`X.Y.Z`, `X.Y`, `latest`) and publishes the GitHub release. See
  `docs/RELEASING.md`.
- **Repository automation (#26):** `pr-checks.yaml` (Conventional Commits PR
  title, branch name, yamllint, shellcheck, markdownlint, relative doc links),
  `labels-sync.yaml` (labels declared in `.github/labels.yaml`, never deleted)
  and `docs-wiki-sync.yaml` (the Wiki is rebuilt from `docs/`). `make lint`
  and `make wiki-preview` run the same steps locally; `make validate` now
  includes `lint`.
- **Access levels (#23):** `MCP_ACCESS_LEVEL=read|write|full` (default
  `write`) decides which tools are registered. `full` adds seven permanent
  deletes (`delete_item`, `delete_group`, `delete_board`, `delete_column`,
  `delete_update`, `delete_folder`, `delete_workspace`) that require
  `confirm: true` and obey the workspace scope and allowlists.
  `server_info.access_level` reports the level; the smoke suite checks all
  three levels and gains an opt-in `--allow-delete` phase.
- **Workspace scope (#20):** `MONDAY_WORKSPACE_ID` confines every read and
  write to one workspace. An omitted `workspace_id` defaults to it, foreign
  boards and items are refused with a typed `ScopeError`, and
  `server_info.workspace_scope` reports the ID and name. Unscoped servers now
  instruct the client to pick a workspace (or ask the user) before writing.
- **Profiles (#24):** `MCP_PROFILE=<name>` loads `profiles/<name>.env`, a
  pinned target (token reference via `MONDAY_API_TOKEN_ENV`, workspace,
  access level, allowlists) that wins over the environment. Tokens are
  rejected in profile files and unknown keys fail at startup, as in
  mcp-github-projects. `server_info.profile` reports the active profile; the
  image mounts profiles at `/profiles` (`MCP_PROFILES_DIR`).
- **Structured `.env.example`** with sections and read / write / full presets.

- **Coverage gate (#25):** `internal/application` 58.7% → 81.5% and
  `internal/monday` 53.9% → 84.5% with table-driven adapter contract tests
  (exact variables, omitted nulls, reply mapping, not-found and GraphQL error
  paths) and a use-case suite run unscoped, scoped, and allowlisted.
  `scripts/coverage_gate.sh` fails CI and `make validate` below 80%.

### Changed

- **Dependencies (#26):** Go builder image 1.25.0 -> 1.27.1 (the `go.mod`
  minimum stays 1.25), `actions/checkout` v4 -> v7, `gitleaks-action` v2 -> v3.
  Supersedes Dependabot PRs #9, #10 and #11.
- `MCP_READ_ONLY` is a deprecated alias of `MCP_ACCESS_LEVEL=read`.

### Fixed

- **`MCP_LOG_LEVEL` was read but ignored** (logs were always INFO). It is now
  validated (`debug`, `info`, `warn`, `error`; anything else fails at startup)
  and applied to the stderr logger.
- **Docs audit (#27)** against the code: CAPABILITIES said archived objects
  are restorable for 30 days — monday keeps archives with no time limit (the
  30 days apply to deleted objects in monday's Trash), now stated precisely;
  README gains `MCP_LOG_LEVEL`, `MONDAY_API_TOKEN_ENV`, the full list of Make
  targets and the new guides; SETUP describes every workflow (coverage, race,
  PR Checks, labels, Wiki) instead of the original two; SMOKE warns that the
  `--report` file contains account URLs.

## [0.2.0] — Operational MVP

Epic #14. Closes #15, #16, #17, #18.

### Added

- **Board schema, lookup, search, pagination (#15):** `get_board_schema`,
  `get_workspace`, `list_folders`, `list_group_items`, `search_items`,
  `find_items_by_column_values`, `get_item`, `get_items`, `list_subitems`;
  cursor pagination (`cursor`, `has_more`) and filter rules on `list_items`;
  pagination and filters on workspace/board listings.
- **Users, teams, updates, collaboration (#16):** `list_users`,
  `search_users`, `get_user`, `list_teams`, `list_item_updates`,
  `list_board_updates`, `create_update`, `reply_to_update`, `like_update`,
  `notify_user`, `add_board_subscribers`, `list_tags`, `create_or_get_tag`.
- **Typed column-value validation and safe writes (#17):** domain validator
  for 23 writable column types with explicit rejection of computed types;
  `validate_column_values`, `describe_column_formats`, `create_subitem`,
  `set_item_status`, `set_item_date`, `assign_item_people`, `rename_item`,
  `move_item_to_board`, `duplicate_item`, `bulk_update_items`,
  `bulk_move_items`, `bulk_archive_items`; board/group/column/workspace/folder
  management (`create_board`, `update_board`, `archive_board`,
  `duplicate_board`, `create_group`, `update_group`, `duplicate_group`,
  `archive_group`, `create_column`, `update_column`, `create_workspace`,
  `create_folder`).
- **Operational reports and smoke suite (#18):** `board_summary`,
  `column_distribution`, `workload_report`, `overdue_items`, `stale_items`,
  `daily_standup`, `board_health_report`, `export_board_markdown`,
  `export_board_csv`, `workspace_overview`; `scripts/smoke.py` real-account
  suite with read, guard, sandbox, and provision phases.
- Write policy: `MCP_READ_ONLY`, `MONDAY_WRITE_BOARD_ALLOWLIST`,
  `MONDAY_WRITE_WORKSPACE_ALLOWLIST`; `MCP_REPORT_MAX_ITEMS`.
- Board templates (`list_board_templates`, `provision_board_from_template`).
- Diagnostics: `list_tool_catalog`, `get_me`, `get_api_status`; richer
  `server_info` (tool count, write policy).
- MCP prompts `board_health_review` and `standup_digest`; MCP tool
  annotations on every tool.
- Application and domain layers matching the documented architecture.
- Docs: TOOLS, COLUMN_VALUES, CAPABILITIES, REPORTS, SMOKE, ADR 0002,
  SECURITY. Make targets `fmt`, `probe`, `tools`, `smoke`,
  `smoke-provision`, `docs-tools`.

### Fixed

- `list_board_columns` queried `settings_str`, which API `2026-07` no longer
  exposes; columns now read `settings` (object or encoded string).
- `create_item` / `update_item_column_values` sent column values as a JSON
  object; monday requires a JSON-encoded string.
- `users(page: null)` and similar explicit nulls caused monday internal
  errors; nil variables are now omitted.
- GraphQL errors are typed (`RequestError` with monday's code and path) and
  complexity/rate-limit errors are retried within the configured bound.

## [0.1.0] — Foundation

- Go MCP server, validated configuration, bounded GraphQL transport, Docker
  Compose, `server_info`, workspace/board/item foundation tools.
