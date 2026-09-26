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

## Workspace scope

`MONDAY_WORKSPACE_ID=<id>` confines the server to one workspace, for **reads
and writes**. It is enforced by a decorator on the application port
(`internal/application/scope.go`), so every tool goes through it.

It takes a numeric ID, not a name: monday does not guarantee unique workspace
names. Find the ID with `list_workspaces` on an unscoped server.

| Situation | Behaviour |
|---|---|
| `workspace_id` omitted | Defaults to the scope (`get_workspace`, `list_folders`, `create_folder`, `create_board`, `duplicate_board`, `provision_board_from_template`, `workspace_overview`). |
| `list_workspaces` | Returns only the scoped workspace. |
| `list_boards` | Filter forced to the scope; a different `workspace_ids` is refused. |
| Board-level tools | The board's workspace is resolved (and cached) first; foreign boards are refused. |
| Item-level tools | The item's board is resolved first; returned item pages are re-checked. |
| `create_workspace`, `notify` with `target_type=Post` | Refused: the target cannot be verified against the scope. |
| Users, teams, tags, `get_me`, `get_api_status` | Account-level; not scoped. |

Refusals are typed (`ScopeError`), name `MONDAY_WORKSPACE_ID`, and happen
before any mutation reaches monday. `server_info.workspace_scope` reports the
ID and resolved name, and the MCP server instructions tell the client which
workspace is in force.

Without a scope the server stays account-wide, and its instructions tell the
client to resolve the workspace from the request, or to call
`list_workspaces` and ask the user before writing.

The scope composes with the write policy below: `MCP_READ_ONLY` and the
allowlists still apply inside the scoped workspace.

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
| Single-team workspace | `MONDAY_WORKSPACE_ID=<team workspace>` |
| Team sandbox | `MONDAY_WORKSPACE_ID=<sandbox workspace>`, `MONDAY_WRITE_WORKSPACE_ALLOWLIST=<sandbox workspace>` and `MONDAY_WRITE_BOARD_ALLOWLIST=<sandbox boards>` |
| Trusted automation | no allowlist, dedicated monday user with minimal board access |
