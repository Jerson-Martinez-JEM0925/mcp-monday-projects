# Tool reference

Every tool speaks typed JSON: inputs are validated by JSON Schema before the
handler runs, and every result carries `structuredContent` that matches the
tool's output schema. Errors are returned as tool results with `isError: true`
and an actionable message; the token and raw API payloads are never echoed.

## Conventions

| Convention | Behavior |
|---|---|
| IDs | monday IDs are numeric strings (`"1234567890"`). Non-numeric IDs are rejected before any API call. |
| Pagination | List tools take `limit` (bounded, default 25) and `page`; item tools return a `cursor` and `has_more`. Pass the cursor back to continue. |
| Column values | Write tools accept friendly values (`"Done"`, `"2026-10-01"`, `[user_id]`) and validate them against the board schema. See [COLUMN_VALUES.md](COLUMN_VALUES.md). |
| Safe writes | There is no delete tool. Destructive verbs archive (restorable from monday). `archive_board` and `archive_group` require `confirm: true`. |
| Bulk | Bulk tools accept at most 50 rows and default to `dry_run: true`. A bulk update writes nothing if any row is invalid. |
| Write policy | `MCP_READ_ONLY=true` hides every write tool. `MONDAY_WRITE_BOARD_ALLOWLIST` / `MONDAY_WRITE_WORKSPACE_ALLOWLIST` restrict mutations. See [CAPABILITIES.md](CAPABILITIES.md). |
| Annotations | Each tool advertises MCP `readOnlyHint`, `destructiveHint`, `idempotentHint`, and `openWorldHint` so clients can gate confirmations. |

## Catalog

<!-- tools:begin -->

_72 tools, generated from `list_tool_catalog`._

### Diagnostics

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `get_api_status` | read | `account.read` | Return monday's remaining per-minute complexity budget, reset time, and the API version that served the request. |
| `get_me` | read | `account.read` | Return the user and account behind the configured token (connectivity check). Never returns the token. |
| `list_tool_catalog` | read | `account.read` | List every registered tool with its category, read-only/destructive hints, and required capability. Optionally filter by category. |
| `server_info` | read | `account.read` | Return safe server metadata: version, runtime, API version, tool count, and the effective write policy (read-only mode and allowlists). |

### Workspaces and folders

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `create_folder` | write | `workspaces.write` | Create a folder in a workspace (workspace must be allowed by the write policy). |
| `create_workspace` | write | `workspaces.write` | Create a workspace. Refused while a write allowlist is configured. |
| `get_workspace` | read | `workspaces.read` | Get one workspace by ID. |
| `list_folders` | read | `workspaces.read` | List folders, optionally within one workspace. |
| `list_workspaces` | read | `workspaces.read` | List visible monday.com workspaces with pagination and kind/state filters. |

### Boards and templates

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `add_board_subscribers` | write | `boards.write` | Subscribe users to a board as subscribers or owners. |
| `archive_board` | write · archive | `boards.write` | Archive (never delete) a board. Requires confirm=true. |
| `create_board` | write | `boards.write` | Create a board in a workspace. Boards created here become writable for this server session even under an allowlist. |
| `duplicate_board` | write | `boards.write` | Duplicate a board's structure, optionally with items and updates, into a workspace/folder. |
| `get_board` | read | `boards.read` | Get one monday.com board by ID. |
| `get_board_schema` | read | `boards.read` | Get a board's full structure: columns (with status/dropdown labels), groups, owners, subscribers, and tags, plus the status/date/people columns reports will use. |
| `list_board_templates` | read | `boards.read` | List the built-in board blueprints (devops, incident, release, project) with their groups and typed columns. |
| `list_boards` | read | `boards.read` | List boards with pagination and workspace/state/kind filters, including item counts and URLs. |
| `provision_board_from_template` | write | `boards.write` | Create a ready-to-use board from a template in one call: typed columns with labels, ordered groups, and optional validated seed items. Additive only; nothing is deleted. |
| `update_board` | write | `boards.write` | Rename a board or change its description. |

### Groups

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `archive_group` | write · archive | `groups.write` | Archive (never delete) a group and its items. Requires confirm=true. |
| `create_group` | write | `groups.write` | Create a group, optionally positioned before/after another group. |
| `duplicate_group` | write | `groups.write` | Duplicate a group together with its items. |
| `list_board_groups` | read | `groups.read` | List groups in a monday.com board. |
| `update_group` | write | `groups.write` | Change a group's title, color, or position. |

### Columns

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `create_column` | write | `columns.write` | Create a typed column; status and dropdown columns accept initial labels. |
| `describe_column_formats` | read | `columns.read` | Describe the friendly value formats accepted per column type by every write tool, and which types are read-only. |
| `list_board_columns` | read | `columns.read` | List columns in a monday.com board, including settings such as status labels. |
| `update_column` | write | `columns.write` | Rename a column and/or change its description. |

### Items — read and search

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `find_items_by_column_values` | read | `items.read` | Find items whose columns exactly match given display values (e.g. status=Done and environment=prod). |
| `get_item` | read | `items.read` | Get one item with its board, group, creator, column values, and subitems. |
| `get_items` | read | `items.read` | Get up to 100 items by ID in one request. |
| `list_group_items` | read | `items.read` | Read one cursor page of items in a single group. |
| `list_items` | read | `items.read` | Read one cursor page of board items with column values; pass the returned cursor to continue. Supports server-side filter rules. |
| `list_subitems` | read | `items.read` | List the subitems of an item. |
| `search_items` | read | `items.read` | Search a board for items whose name contains text, optionally combined with filter rules (status any_of, date within_the_next, etc.). |
| `validate_column_values` | read | `items.read` | Dry-run: validate friendly column values against a board schema and return the exact monday JSON that a write would send, or per-column issues. |

### Items — write

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `archive_item` | write · archive | `items.write` | Archive (never delete) a monday.com item; it can be restored from monday's archive. |
| `assign_item_people` | write | `items.write` | Assign users or teams to an item's people column (auto-detected); an empty list clears it. |
| `create_item` | write | `items.write` | Create an item; column values are validated by type against the board schema before monday is called. |
| `create_subitem` | write | `items.write` | Create a subitem under a parent item; values are validated against the subitems board. |
| `duplicate_item` | write | `items.write` | Duplicate an item, optionally with its updates. |
| `move_item` | write | `items.write` | Move a monday.com item to another group. |
| `move_item_to_board` | write | `items.write` | Move an item to a group on another board (both boards must be writable). |
| `rename_item` | write | `items.write` | Rename an item. |
| `set_item_date` | write | `items.write` | Set or clear a date on an item; the date column is auto-detected (prefers due/deadline). |
| `set_item_status` | write | `items.write` | Set a status label on an item; the status column is auto-detected and the label is validated. |
| `update_item_column_values` | write | `items.write` | Update several column values of an item in one mutation, after typed validation. |

### Bulk operations

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `bulk_archive_items` | write · archive | `items.write` | Archive (never delete) up to 50 items. dry_run defaults to true. |
| `bulk_move_items` | write | `items.write` | Move up to 50 items to a group. dry_run defaults to true. |
| `bulk_update_items` | write | `items.write` | Validate and apply column values to up to 50 items. dry_run defaults to true; nothing is written if any row is invalid. |

### Users and teams

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `get_user` | read | `people.read` | Get one user by ID. |
| `list_teams` | read | `people.read` | List teams with their members. |
| `list_users` | read | `people.read` | List account users (name, email, title, kind, status, teams) with pagination. |
| `search_users` | read | `people.read` | Find users by partial name or exact email — handy to resolve IDs before assign_item_people. |

### Updates and notifications

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `create_update` | write | `collaboration.write` | Post an update (comment) on an item. |
| `like_update` | write | `collaboration.write` | Like an update. |
| `list_board_updates` | read | `collaboration.read` | List the most recent updates across a board — an activity feed. |
| `list_item_updates` | read | `collaboration.read` | List an item's updates (comments) with replies and authors. |
| `notify_user` | write | `collaboration.write` | Send a monday bell notification to a user about an item or update. |
| `reply_to_update` | write | `collaboration.write` | Reply to an existing update. |

### Tags

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `create_or_get_tag` | write | `tags.write` | Return an existing tag by name or create it; use its ID in a tags column. |
| `list_tags` | read | `tags.read` | List account public tags. |

### Reports

| Tool | Mode | Capability | Description |
|---|---|---|---|
| `board_health_report` | read | `reports.read` | Score a board 0-100 (grade A-D) from overdue, blocked, stale, and unassigned work, with explained signals and Markdown. |
| `board_summary` | read | `reports.read` | Summarize a board: items per group and status, completion %, overdue, blocked, and unassigned open work. |
| `column_distribution` | read | `reports.read` | Count items by the display value of any column (priority, environment, owner, ...). |
| `daily_standup` | read | `reports.read` | Standup digest for the last N hours: completed, in motion, blocked, and overdue, plus ready-to-paste Markdown. |
| `export_board_csv` | read | `reports.read` | Export a board as RFC 4180 CSV (item id, name, group, updated_at, then columns). |
| `export_board_markdown` | read | `reports.read` | Export a board as Markdown tables grouped by group. |
| `overdue_items` | read | `reports.read` | List open items whose due date has passed, most late first, with days late. |
| `stale_items` | read | `reports.read` | List open items with no update for N days (default 14). |
| `workload_report` | read | `reports.read` | Open, done, overdue, and blocked items per assignee of the owner column. |
| `workspace_overview` | read | `reports.read` | Overview of a workspace: active boards (excluding subitem boards), item totals, and URLs. |

<!-- tools:end -->

## Prompts

| Prompt | Arguments | Purpose |
|---|---|---|
| `board_health_review` | `board_id` | Runs the health and workload reports and proposes up to five concrete, validated actions. |
| `standup_digest` | `board_id` | Builds a team standup from `daily_standup` and the board update feed. |

## Examples

Search open work in a group and move it:

```json
{"name": "search_items", "arguments": {"board_id": "1234567890", "text": "pipeline",
  "filter": {"rules": [{"column_id": "status", "compare_value": [2], "operator": "any_of"}]}}}
```

Validate before writing (no API mutation):

```json
{"name": "validate_column_values", "arguments": {"board_id": "1234567890",
  "column_values": {"status": "Done", "due_date": "2026-10-01", "owner": ["12345678"], "estimate": 5}}}
```

Plan a bulk update, then apply it:

```json
{"name": "bulk_update_items", "arguments": {"board_id": "1234567890",
  "updates": [{"item_id": "9876543210", "column_values": {"priority": "High"}}]}}
{"name": "bulk_update_items", "arguments": {"board_id": "1234567890", "dry_run": false,
  "updates": [{"item_id": "9876543210", "column_values": {"priority": "High"}}]}}
```

Provision a DevOps board with seed items:

```json
{"name": "provision_board_from_template", "arguments": {"template": "devops",
  "workspace_id": "555000111", "board_kind": "private",
  "seed_items": [{"group": "In Progress", "name": "CI pipeline", "values": {"status": "Working on it", "priority": "High"}}]}}
```

Regenerate the catalog above after adding a tool:

```bash
make docs-tools
```
