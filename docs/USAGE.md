# Usage

Task-oriented guide for the monday.com MCP server. Every tool, its mode and
capability is listed in [TOOLS.md](TOOLS.md) (generated from the server);
this page shows how the tools fit together. Examples are the `arguments`
object an MCP client sends in `tools/call`; IDs are placeholders.

## Calling conventions

- **IDs are numeric strings**: `"board_id": "1234567890"`. Non-numeric IDs are
  rejected before any API call.
- **Column values are friendly and validated**: pass `"Done"`, `"2026-10-01"`
  or `["12345678"]` keyed by column ID; the server checks them against the live
  board schema and converts them to monday JSON. Formats per column type:
  [COLUMN_VALUES.md](COLUMN_VALUES.md).
- **Pagination**: list tools take `limit` and `page`; item tools return a
  `cursor` (pass it back to continue) and `has_more`.
- **Errors come back as tool results** with `isError: true` and one
  actionable sentence; see [TROUBLESHOOTING.md](TROUBLESHOOTING.md).
- **What you can call depends on configuration**: `MCP_ACCESS_LEVEL`,
  `MONDAY_WORKSPACE_ID`, the write allowlists and `MCP_PROFILE`
  ([CAPABILITIES.md](CAPABILITIES.md)). `server_info` reports the effective
  policy; a tool that is not registered answers `unknown tool`.

## 1. Check the connection and the policy

```json
{"name": "get_me", "arguments": {}}
{"name": "server_info", "arguments": {}}
{"name": "get_api_status", "arguments": {}}
```

`get_me` shows whose token is in use — every action is attributed to that
user in monday. `server_info` shows the access level, the workspace scope
(with its resolved name) and the allowlists. `get_api_status` returns the
remaining per-minute complexity budget.

## 2. Find the workspace, board and columns

```json
{"name": "list_workspaces", "arguments": {"limit": 25}}
{"name": "list_boards", "arguments": {"workspace_ids": ["14216815"], "limit": 25}}
{"name": "get_board_schema", "arguments": {"board_id": "1234567890"}}
```

With `MONDAY_WORKSPACE_ID` set, `list_workspaces` returns only that workspace
and `list_boards` only its boards. `get_board_schema` returns columns with
their status/dropdown labels, groups, owners and the status/date/people
columns the reports will use — read it before writing.

## 3. Read and search items

```json
{"name": "list_items", "arguments": {"board_id": "1234567890", "limit": 50}}
{"name": "search_items", "arguments": {"board_id": "1234567890", "text": "pipeline"}}
{"name": "find_items_by_column_values", "arguments": {"board_id": "1234567890",
  "columns": [{"column_id": "status", "column_values": ["Stuck"]}]}}
{"name": "get_item", "arguments": {"item_id": "9876543210"}}
```

`search_items` also takes a monday `filter` (rules with `column_id`,
`compare_value`, `operator`), see the example in [TOOLS.md](TOOLS.md#examples).
`find_items_by_column_values` matches values exactly.

## 4. Write safely

Validate first — this never mutates:

```json
{"name": "validate_column_values", "arguments": {"board_id": "1234567890",
  "column_values": {"status": "Done", "due_date": "2026-10-01", "owner": ["12345678"]}}}
```

Then write:

```json
{"name": "create_item", "arguments": {"board_id": "1234567890", "group_id": "topics",
  "name": "Rotate API keys", "column_values": {"status": "Working on it", "priority": "High"}}}
{"name": "set_item_status", "arguments": {"board_id": "1234567890", "item_id": "9876543210", "label": "Done"}}
{"name": "set_item_date", "arguments": {"board_id": "1234567890", "item_id": "9876543210", "date": "2026-10-15"}}
{"name": "assign_item_people", "arguments": {"board_id": "1234567890", "item_id": "9876543210", "user_ids": ["12345678"]}}
{"name": "move_item", "arguments": {"item_id": "9876543210", "group_id": "done"}}
```

A label that does not exist is rejected with the list of valid ones. When
`column_id` is omitted, the single-purpose setters auto-detect the column the
same way the reports do (for dates, a due/deadline column is preferred); a
board without such a column asks for `column_id`. `set_item_date` with an
empty `date` clears it, and `assign_item_people` with an empty list clears
the column.

## 5. Bulk changes: plan, then apply

Bulk tools accept at most 50 rows, **default to `dry_run: true`** and write
nothing if any row is invalid:

```json
{"name": "bulk_update_items", "arguments": {"board_id": "1234567890",
  "updates": [{"item_id": "9876543210", "column_values": {"priority": "High"}}]}}
{"name": "bulk_update_items", "arguments": {"board_id": "1234567890", "dry_run": false,
  "updates": [{"item_id": "9876543210", "column_values": {"priority": "High"}}]}}
{"name": "bulk_move_items", "arguments": {"item_ids": ["9876543210", "9876543211"], "group_id": "done", "dry_run": false}}
```

## 6. Archive and delete

Archiving is the default destructive verb and is restorable from monday's
archive:

```json
{"name": "archive_item", "arguments": {"item_id": "9876543210"}}
{"name": "archive_board", "arguments": {"board_id": "1234567890", "confirm": true}}
```

Permanent `delete_*` tools exist only at `MCP_ACCESS_LEVEL=full`, require
`confirm: true`, and still respect the workspace scope and allowlists:

```json
{"name": "delete_item", "arguments": {"item_id": "9876543210", "confirm": true}}
```

## 7. Collaborate

```json
{"name": "create_update", "arguments": {"item_id": "9876543210", "body": "Deployed to staging."}}
{"name": "notify_user", "arguments": {"user_id": "12345678", "target_id": "9876543210",
  "target_type": "Project", "text": "Ready for review"}}
```

With `MONDAY_WORKSPACE_ID` set, notify on the item (`target_type: "Project"`);
`Post` targets are refused because an update's workspace cannot be verified.

## 8. Reports

Reports detect the status, date and people columns automatically; override
with `status_column_id`, `date_column_id`, `people_column_id`. They load at
most `MCP_REPORT_MAX_ITEMS` items and say so (`truncated`) when the board is
larger. Semantics and the health score: [REPORTS.md](REPORTS.md).

```json
{"name": "board_summary", "arguments": {"board_id": "1234567890"}}
{"name": "workload_report", "arguments": {"board_id": "1234567890"}}
{"name": "overdue_items", "arguments": {"board_id": "1234567890"}}
{"name": "stale_items", "arguments": {"board_id": "1234567890", "days": 14}}
{"name": "daily_standup", "arguments": {"board_id": "1234567890", "hours": 24}}
{"name": "board_health_report", "arguments": {"board_id": "1234567890"}}
{"name": "export_board_markdown", "arguments": {"board_id": "1234567890"}}
```

The prompts `board_health_review` and `standup_digest` chain these reports for
an MCP client ([TOOLS.md](TOOLS.md#prompts)).

## 9. Boards from templates

```json
{"name": "list_board_templates", "arguments": {}}
{"name": "provision_board_from_template", "arguments": {"template": "devops",
  "workspace_id": "14216815", "board_kind": "private",
  "seed_items": [{"group": "In Progress", "name": "CI pipeline", "values": {"status": "Working on it"}}]}}
```

Templates: `devops`, `incident`, `release`, `project`. Provisioning is
additive (nothing is deleted) and validates seed values against the new
board. A board created in the session stays writable for that session even
under a board allowlist.

## 10. Try it from the command line

`scripts/mcp_probe.py` runs the image with your `.env` and calls one tool:

```bash
make probe TOOL=get_me
make probe TOOL=list_boards ARGS='{"limit": 5}'
python3 scripts/mcp_probe.py server_info '{}' -e MCP_ACCESS_LEVEL=read
python3 scripts/mcp_probe.py --list -e MCP_ACCESS_LEVEL=full
```
