# Troubleshooting

Symptoms are the exact messages the server prints (startup, on stderr) or
returns (tool results with `isError: true`). Start with `make probe
TOOL=server_info` to see the effective configuration.

## The server does not start

The process exits at startup with `ERROR invalid configuration error="…"`:

| Message | Fix |
|---|---|
| `MONDAY_API_TOKEN is required` | Set `MONDAY_API_TOKEN` in `.env` (or pass `--env-file .env`). With a profile, set the variable named by `MONDAY_API_TOKEN_ENV`. |
| `MONDAY_API_TOKEN_ENV=X but that environment variable is empty` | The profile points to a token variable that is not set in the container environment. |
| `MONDAY_WORKSPACE_ID must be a single numeric workspace ID (names are not unique in monday)` | Use the ID, not the name: `list_workspaces` shows it (e.g. `14216815`). |
| `MCP_ACCESS_LEVEL must be read, write, or full` | Fix the value; the default is `write`. |
| `MCP_READ_ONLY=true contradicts MCP_ACCESS_LEVEL=…` | `MCP_READ_ONLY` is a deprecated alias; remove it and keep `MCP_ACCESS_LEVEL`. |
| `MCP_PROFILE must be a lowercase name …` / unknown key in a profile | Profile names are `a-z0-9-_`; profile files accept only the keys listed in [CAPABILITIES.md](CAPABILITIES.md#profiles) and never the token itself. |
| `MONDAY_API_URL must be a valid HTTPS URL` | Only HTTPS endpoints are accepted. |
| `MCP_MAX_RETRIES must be between 0 and 5`, `MCP_REPORT_MAX_ITEMS must be between 1 and 5000`, … | Bring the value into the stated range. |

## The MCP client shows no tools or disconnects

- stdout carries the MCP protocol; anything else printed there breaks the
  session. The server logs only to stderr — do not wrap the container in a
  script that echoes to stdout.
- The client must keep stdin open (`docker run -i`). Without `-i` the server
  reads EOF and exits immediately.
- Rebuild after pulling changes: `make build` (the client runs
  `mcp-monday-projects:local`, which does not update itself).

## Authentication and attribution

| Symptom | Cause and fix |
|---|---|
| `get me: monday API returned HTTP 401: Unauthorized` | The token is invalid or revoked. Create a new one in monday (avatar → Developers → My access tokens) and update `.env`. |
| Actions in monday appear as another person | A personal token always acts as its owner. `get_me` shows who that is; use your own token for your own attribution. Already-created objects keep their original author. |
| `USER_UNAUTHORIZED` / permission errors on some boards | The token's user lacks access to that board or workspace (member vs. admin, private boards). Share the board with the user or use a token that has access. |

## Refused by the server's own policy

These are deliberate and happen **before** any call to monday:

| Message | Meaning |
|---|---|
| JSON-RPC error `unknown tool "delete_item"` (or any `delete_*`) | The tool is not registered at the current access level. Deletes exist only at `MCP_ACCESS_LEVEL=full`; mutations do not exist at `read`. |
| `server is running at MCP_ACCESS_LEVEL=read; mutations are disabled` | Same, reported by a write path. |
| `board 123 belongs to workspace 456, outside the configured scope MONDAY_WORKSPACE_ID=789` | `MONDAY_WORKSPACE_ID` confines reads and writes to one workspace. Unset it (or change it) to reach other workspaces. |
| `workspace 456 is outside the configured scope MONDAY_WORKSPACE_ID=789` | Same, for workspace-level tools. |
| `board 123 is not in the write allowlist; add it to MONDAY_WRITE_BOARD_ALLOWLIST to allow mutations` | A board allowlist is set and this board is not in it. Reads still work. |
| `creating workspaces is disabled while a write allowlist is configured` | Workspace creation/deletion is refused whenever any allowlist is set. |
| `archiving a board requires confirm=true; …` / `archiving a group archives all of its items; pass confirm=true` | Pass `confirm: true` after checking the target. |
| `deleting item 123 is permanent and cannot be undone; pass confirm=true …` | Permanent delete guard; the message suggests the matching `archive_*` tool. |
| `notify_user` with `target_type: "Post"` refused under a workspace scope | An update's workspace cannot be verified; notify on the item (`target_type: "Project"`). |

## Column values are rejected

The server validates values against the live board schema and names the
problem:

| Message | Fix |
|---|---|
| `unknown status label; valid labels: …` | Use one of the listed labels (exact text) or its index. `get_board_schema` lists them. |
| `status label "X" is deactivated` | The label exists but is deactivated on the board; pick an active one. |
| `unknown dropdown label "X"; valid labels: …` / `mix of dropdown labels and IDs is not allowed` | Use existing labels, and either names or IDs, not both. |
| `invalid user ID … (use numeric IDs …)` | People columns take numeric user IDs; find them with `search_users`. |
| `invalid email address`, `invalid phone number`, `invalid http(s) URL`, `countryCode must be ISO-3166 alpha-2 …` | Format per [COLUMN_VALUES.md](COLUMN_VALUES.md). |
| `column type "X" requires a raw JSON object value` | Types without a friendly format take monday's own JSON object. |
| `board has no status column; pass column_id explicitly` | Auto-detection found no column of that type; pass `column_id`. |

Bulk tools validate every row first; one invalid row means nothing is written
and the response lists each row's `status` and `issues` (for example
`"unknown status label; valid labels: \"To Do\", \"Working on it\", …"`).

## Rate limits and large boards

| Symptom | Cause and fix |
|---|---|
| `monday graphql request failed [ComplexityException]` / `[COMPLEXITY_BUDGET_EXHAUSTED]` after retries | monday's per-minute complexity budget is spent. The server already retried (bounded by `MCP_MAX_RETRIES`, honoring `retry_in_seconds`). Wait for the reset shown by `get_api_status`, or request smaller pages (`limit`). |
| `monday response exceeded configured limit` | The response was larger than `MCP_MAX_RESPONSE_BYTES` (4 MiB). Use smaller `limit`s or narrower queries; raise the limit only if you must. |
| A report says `truncated: true` | The board has more items than `MCP_REPORT_MAX_ITEMS` (default 500); raise it (≤ 5000) or pass `max_items`. |
| Timeouts | Raise `MCP_HTTP_TIMEOUT` (default `15s`) for slow networks. |

## Development and validation

| Symptom | Fix |
|---|---|
| `make validate` fails on coverage | `scripts/coverage_gate.sh` requires ≥ 80% statement coverage on `internal/application` and `internal/monday`; add tests for the new code. |
| `TestEveryToolIsDocumented` fails | A new or renamed tool is missing from `docs/TOOLS.md`: run `make docs-tools` and commit the result. |
| PR Checks fail on the title or branch | Titles follow Conventional Commits (`feat: …`, `fix(scope): …`); see [CONTRIBUTING.md](../CONTRIBUTING.md). |
| `make lint` reports a broken link | A relative Markdown link points to a missing file or anchor. |
| The Wiki sync job passes but publishes nothing | GitHub creates the wiki repository only after the first page is saved from the Wiki tab; create any page once. |
| Bind mounts (`-v`) fail on some filesystems (e.g. 9p shares) | `make lint` and profile mounts need a working bind mount. Where that fails, copy the files into a throwaway image or container (`docker cp`) instead. |

### Safe item creation

`create_item` performs all local checks before the mutation: the board write policy, the optional active `group_id`, and every supplied column value. If Monday returns a payload without an item ID, the MCP reports an incomplete response instead of returning a zero-value item. Paginated item reads apply the same rule: a missing `items_page` payload is an error, while a present page with zero items is a valid empty page. For a large import, call `get_board_schema` and `validate_column_values` first, follow every returned cursor until `has_more` is false, then process bounded batches and persist each returned item ID so the client can resume without guessing what was created.
