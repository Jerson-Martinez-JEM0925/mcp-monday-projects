# Capabilities and write policy

## Capability matrix

Each tool declares a capability (visible in `list_tool_catalog` and
[TOOLS.md](TOOLS.md)). monday API tokens are user-scoped: the token can do
whatever its user can do in the UI. Use a dedicated user and the server-side
write policy below to narrow what an MCP client can change.

| Capability | Covers | monday OAuth scope (apps) |
|---|---|---|
| `account.read` | `get_me`, `get_api_status`, `server_info`, catalog | `me:read`, `account:read` |
| `workspaces.read` / `.write` | workspaces and folders | `workspaces:read` / `workspaces:write` |
| `boards.read` / `.write` | boards, templates, subscribers | `boards:read` / `boards:write` |
| `groups.read` / `.write` | groups | `boards:read` / `boards:write` |
| `columns.read` / `.write` | columns, formats | `boards:read` / `boards:write` |
| `items.read` / `.write` | items, subitems, validation, bulk | `boards:read` / `boards:write` |
| `people.read` | users and teams | `users:read`, `teams:read` |
| `collaboration.read` / `.write` | updates, replies, likes, notifications | `updates:read` / `updates:write`, `notifications:write` |
| `tags.read` / `.write` | tags | `tags:read` / `boards:write` |
| `reports.read` | every report | `boards:read` |

## Write policy

The policy is enforced in the application layer (`internal/application/guard.go`)
before any GraphQL mutation is built.

| Variable | Effect |
|---|---|
| `MCP_READ_ONLY=true` | Write tools are **not registered** — clients cannot even see them. `server_info.hidden_write_tools` reports how many were hidden. |
| `MONDAY_WRITE_BOARD_ALLOWLIST=1,2` | Mutations are allowed only on these boards. Item-level writes resolve the item's board first. |
| `MONDAY_WRITE_WORKSPACE_ALLOWLIST=7` | Board/folder creation, duplication, and template provisioning are allowed only in these workspaces. |

When any allowlist is set:

- `create_workspace` is refused.
- Boards created by this server process (`create_board`, `duplicate_board`,
  `provision_board_from_template`) become writable for the rest of the process,
  so a freshly provisioned board can be populated without widening the list.
- Refusals are typed (`GuardError`) and name the variable to change.

`server_info.write_policy` shows the effective policy at runtime.

## Safety properties

- No tool deletes anything. `archive_*` tools archive (restorable in monday
  for 30 days); `archive_board` and `archive_group` require `confirm: true`.
- Bulk tools are capped at 50 rows and default to `dry_run: true`.
- `create_labels_if_missing` is always `false`.
- Every tool advertises MCP annotations (`readOnlyHint`, `destructiveHint`)
  so clients can require confirmation for destructive calls.

## Recommended profiles

| Profile | Settings |
|---|---|
| Analyst / reporting | `MCP_READ_ONLY=true` |
| Team sandbox | `MONDAY_WRITE_WORKSPACE_ALLOWLIST=<sandbox workspace>` and `MONDAY_WRITE_BOARD_ALLOWLIST=<sandbox boards>` |
| Trusted automation | no allowlist, dedicated monday user with minimal board access |
