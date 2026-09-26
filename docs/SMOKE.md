# Real-account smoke suite

`scripts/smoke.py` drives the containerized server over stdio exactly like an
MCP client and prints one PASS/FAIL line per tool call. It is the release gate
for behavior that unit and contract tests cannot prove: monday's real schema,
permissions, and eventual consistency.

## Safety model

- The token is passed with `docker run --env-file .env`; the script never
  reads or prints it.
- **Nothing is deleted.** The only destructive verb used is archive, and only
  on an item the suite itself duplicated moments before.
- The sandbox phase runs with `MONDAY_WRITE_BOARD_ALLOWLIST=<sandbox board>`,
  so a bug cannot write outside that board.
- The provision phase runs with `MONDAY_WRITE_WORKSPACE_ALLOWLIST=<workspace>`
  and a board allowlist that matches nothing, so it can only populate the
  board it just created.
- Guard checks prove read-only mode hides write tools and that a
  non-allowlisted board is refused before monday is called.

## Phases

| Phase | Trigger | What it proves |
|---|---|---|
| `read` | always | account, API budget, catalog, workspaces, boards, folders, users, teams, tags, formats, templates, input validation |
| `guard` | always | read-only mode and allowlist refusals |
| `sandbox` | `--board ID` | columns, groups, validation (valid + rejected), item CRUD, typed setters, tags, subitems, duplicate/move/archive, cursor pagination, search, filters, exact-match lookup, updates/replies/likes, notifications, bulk dry-run/all-or-nothing/execute, every report and export |
| `provision` | `--provision --workspace ID` | `provision_board_from_template` with validated seed items, health report, subscribers |

## Run

```bash
make build
make smoke SMOKE_BOARD=<sandbox board id> SMOKE_USER=<your user id>
# optional one-off: create a template board in a sandbox workspace
make smoke-provision SMOKE_WORKSPACE=<workspace id> SMOKE_USER=<your user id>
```

Or directly:

```bash
python3 scripts/smoke.py --board 1234567890 --assign-user 12345678 \
  --report smoke-report.md
```

The suite is idempotent on the sandbox board: it creates missing columns and
groups once, then adds timestamped items on each run. A Markdown report
(`--report`) is suitable for attaching to a PR.

## Reference run

The suite was executed against a real enterprise account before this
release. Sandbox board and template board live in the account's DevOps
workspace; both were created by this project and no pre-existing board was
modified.
