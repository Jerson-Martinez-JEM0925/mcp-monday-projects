# Reports

Reports are pure functions over a **bounded board snapshot**: the board
schema plus up to `MCP_REPORT_MAX_ITEMS` items (default 500, max 5000) loaded
with cursor pagination. When the limit is reached the result carries
`truncated: true` and figures are a lower bound. Pass `max_items` to lower the
budget per call.

## Column detection

Reports need a status, a due-date, and an owner column. They are detected
automatically and returned in every response (`columns`), so you can see what
was used:

| Role | Column types | Title hints (first match wins) |
|---|---|---|
| Status | `status` | status, estado, state |
| Due date | `date` | due, deadline, vence, entrega, fecha límite |
| Owner | `people` | owner, responsable, assignee, asignado, person |

Override with `status_column_id`, `date_column_id`, `people_column_id`.
`get_board_schema` also returns the detected columns.

**Done** comes from the status column's `is_done` label flag; boards without
it fall back to common labels (Done, Listo, Hecho, Completado, Resolved…).
**Blocked** matches labels containing stuck, blocked, bloque, detenido, atascado.

## Catalog

| Tool | Output |
|---|---|
| `board_summary` | totals, per-group and per-status counts, completion %, overdue, blocked, unassigned open |
| `column_distribution` | counts per display value of any column |
| `workload_report` | per assignee: open, done, overdue, blocked (multi-owner items count for each owner) |
| `overdue_items` | open items past due, most late first, with `days_late` |
| `stale_items` | open items without updates for `days` (default 14) |
| `daily_standup` | completed / in motion / blocked / overdue in the last `hours` (default 24) + Markdown |
| `board_health_report` | 0–100 score, A–D grade, explained signals + Markdown |
| `export_board_markdown` | Markdown tables per group |
| `export_board_csv` | RFC 4180 CSV |
| `workspace_overview` | active boards (subitem boards excluded) and item totals |

## Health score

Starts at 100 and subtracts, relative to **open** work:

| Signal | Max penalty |
|---|---:|
| Overdue share | 40 |
| Blocked share | 25 |
| Stale share | 20 |
| Unassigned share | 15 |

Grades: A ≥ 85, B ≥ 70, C ≥ 50, D < 50. Every applied penalty is listed in
`signals`, so the score is always explainable.
