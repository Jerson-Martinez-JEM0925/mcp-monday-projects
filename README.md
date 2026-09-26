# mcp-monday-projects

[![CI](https://github.com/jersonmartinez/mcp-monday-projects/actions/workflows/ci.yaml/badge.svg)](https://github.com/jersonmartinez/mcp-monday-projects/actions/workflows/ci.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker--first-2496ED.svg?logo=docker&logoColor=white)](Dockerfile)

A high-performance, Docker-first [Model Context Protocol](https://modelcontextprotocol.io/) server for monday.com, written in Go. It works with any MCP client that supports the stdio transport.

## Features

- **72 typed MCP tools** across workspaces, boards, groups, columns, items, bulk operations, users and teams, updates, tags, and reports — plus 2 reusable prompts.
- **Validated writes** — friendly column values (`"Done"`, `"2026-10-01"`, `[user_id]`) are checked against the live board schema and converted to monday JSON before any mutation. See [docs/COLUMN_VALUES.md](docs/COLUMN_VALUES.md).
- **Safe by construction** — no delete tools, archive-only destructive verbs, `confirm` gates, bulk operations capped at 50 rows with `dry_run: true` by default and all-or-nothing validation.
- **Write policy** — `MCP_READ_ONLY` hides every write tool; board and workspace allowlists fence mutations. See [docs/CAPABILITIES.md](docs/CAPABILITIES.md).
- **Operational reports** — summary, workload, overdue, stale, standup, 0–100 health score, Markdown/CSV export, workspace overview. See [docs/REPORTS.md](docs/REPORTS.md).
- **Board templates** — `provision_board_from_template` builds DevOps, incident, release, or project boards with typed columns, ordered groups, and validated seed items in one call.
- **Self-describing** — `list_tool_catalog` returns every tool with category, capability, and read-only/destructive hints; MCP annotations are set on every tool.
- **Hardened transport** — bounded timeouts, response limits, retries only for transient HTTP and complexity errors, typed GraphQL errors, complexity-budget diagnostics.
- **Reproducible real-account smoke suite** — see [docs/SMOKE.md](docs/SMOKE.md).

## Quick start

```bash
cp .env.example .env
# Set MONDAY_API_TOKEN in .env
make validate
make tools          # list the registered tools
make probe TOOL=get_me
make run            # stdio server for an MCP client
```

Only Docker and Docker Compose are required on the host. Go, tests, formatting, and runtime execution are containerized through the Makefile. MCP client configuration is in [docs/SETUP.md](docs/SETUP.md#mcp-client-configuration).

## Tool catalog

| Category | Tools |
|---|---|
| Diagnostics | `server_info`, `list_tool_catalog`, `get_me`, `get_api_status` |
| Workspaces | `list_workspaces`, `get_workspace`, `create_workspace`, `list_folders`, `create_folder` |
| Boards | `list_boards`, `get_board`, `get_board_schema`, `create_board`, `update_board`, `archive_board`, `duplicate_board`, `add_board_subscribers`, `list_board_templates`, `provision_board_from_template` |
| Groups | `list_board_groups`, `create_group`, `update_group`, `duplicate_group`, `archive_group` |
| Columns | `list_board_columns`, `create_column`, `update_column`, `describe_column_formats` |
| Items — read | `list_items`, `list_group_items`, `search_items`, `find_items_by_column_values`, `get_item`, `get_items`, `list_subitems`, `validate_column_values` |
| Items — write | `create_item`, `create_subitem`, `update_item_column_values`, `set_item_status`, `set_item_date`, `assign_item_people`, `rename_item`, `move_item`, `move_item_to_board`, `duplicate_item`, `archive_item` |
| Bulk | `bulk_update_items`, `bulk_move_items`, `bulk_archive_items` |
| People | `list_users`, `search_users`, `get_user`, `list_teams` |
| Collaboration | `list_item_updates`, `list_board_updates`, `create_update`, `reply_to_update`, `like_update`, `notify_user` |
| Tags | `list_tags`, `create_or_get_tag` |
| Reports | `board_summary`, `column_distribution`, `workload_report`, `overdue_items`, `stale_items`, `daily_standup`, `board_health_report`, `export_board_markdown`, `export_board_csv`, `workspace_overview` |

Full reference with modes and capabilities: [docs/TOOLS.md](docs/TOOLS.md).

## Configuration

| Variable | Required | Default | Purpose |
|---|---:|---|---|
| `MONDAY_API_TOKEN` | yes | — | monday.com API token |
| `MONDAY_API_VERSION` | no | `2026-07` | Value sent in the `API-Version` header |
| `MONDAY_API_URL` | no | `https://api.monday.com/v2` | Monday GraphQL endpoint (HTTPS only) |
| `MCP_HTTP_TIMEOUT` | no | `15s` | Outbound request timeout |
| `MCP_MAX_RESPONSE_BYTES` | no | `4194304` | Response size limit |
| `MCP_MAX_RETRIES` | no | `2` | Bounded retries for transient HTTP/complexity failures |
| `MCP_READ_ONLY` | no | `false` | Hide every write tool |
| `MONDAY_WRITE_BOARD_ALLOWLIST` | no | — | Comma-separated board IDs allowed for mutations |
| `MONDAY_WRITE_WORKSPACE_ALLOWLIST` | no | — | Comma-separated workspace IDs allowed for board/folder creation |
| `MCP_REPORT_MAX_ITEMS` | no | `500` | Items loaded per report (1–5000) |

See [docs/SETUP.md](docs/SETUP.md) for token handling and API versioning. Confirm the stable version against Monday's [versioning documentation](https://developer.monday.com/api-reference/docs/api-versioning) before upgrades.

## Architecture

```text
MCP tools → application services → domain models → monday port → GraphQL transport
```

| Package | Responsibility |
|---|---|
| `cmd/mcp-server` | Entrypoint: configuration, logging to stderr, stdio transport |
| `internal/mcpserver` | Tool registry, typed inputs/outputs, annotations, prompts |
| `internal/application` | Use cases: input validation, write policy, dry-run planning, bulk, provisioning, report snapshots |
| `internal/domain` | Provider-neutral types, column-value validation, templates, pure report calculations |
| `internal/monday` | GraphQL adapter: queries, JSON encoding, typed errors, retries, limits |
| `internal/config` | Validated environment configuration |

The protocol layer never builds GraphQL. Decisions are recorded in [docs/adr](docs/adr/README.md).

## Make targets

```text
make help             Show targets
make build            Build the runtime image
make run              Run the stdio MCP server
make test             Run Go tests in Docker
make race             Run race-enabled tests in Docker
make fmt / fmt-check  Format / verify gofmt in Docker
make vet              Run go vet in Docker
make validate         Full local validation (build, test, race, fmt, vet)
make tools            List registered tools
make probe            Call one tool: make probe TOOL=get_me ARGS='{}'
make smoke            Real-account smoke suite on a sandbox board
make docs-tools       Regenerate the catalog in docs/TOOLS.md
make down             Stop Compose resources
```

## Documentation

| Document | Content |
|---|---|
| [docs/SETUP.md](docs/SETUP.md) | Token, versioning, MCP client configuration, validation |
| [docs/TOOLS.md](docs/TOOLS.md) | Tool reference, conventions, examples, prompts |
| [docs/COLUMN_VALUES.md](docs/COLUMN_VALUES.md) | Accepted value formats and rejection rules |
| [docs/CAPABILITIES.md](docs/CAPABILITIES.md) | Capability matrix and write policy |
| [docs/REPORTS.md](docs/REPORTS.md) | Report semantics and health score |
| [docs/SMOKE.md](docs/SMOKE.md) | Real-account smoke suite |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## Collaboration

- Start work from an issue in [Project #12](https://github.com/users/jersonmartinez/projects/12).
- Create an epic when work spans more than one issue.
- Every new tool requires tests and documentation in the same PR; `TestEveryToolIsDocumented` enforces the catalog.
- Use Conventional Commits and open PRs ready for review.
- `AGENTS.md` is the canonical instruction file. This repository intentionally does not use `CLAUDE.md`.

See [CONTRIBUTING.md](CONTRIBUTING.md), [GOVERNANCE.md](GOVERNANCE.md), and [SECURITY.md](SECURITY.md) for project policies.

## License

MIT. See [LICENSE](LICENSE).
