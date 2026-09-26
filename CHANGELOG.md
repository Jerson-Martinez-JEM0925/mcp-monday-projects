# Changelog

All notable changes are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

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
