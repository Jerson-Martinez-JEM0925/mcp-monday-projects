# Column values

Write tools (`create_item`, `create_subitem`, `update_item_column_values`,
`set_item_*`, `assign_item_people`, `bulk_update_items`,
`provision_board_from_template` seeds) accept **friendly values** and convert
them to monday's JSON before calling the API. Validation runs against the
target board's live schema, so a bad label, an unknown column, or a malformed
date fails locally with a per-column explanation — nothing is written.

Use `validate_column_values` to see the exact plan (normalized JSON and
issues) without writing, and `describe_column_formats` for this table as data.

## Accepted inputs

| Type | Accepts | Example | Sent to monday |
|---|---|---|---|
| `name` | non-empty string | `"Deploy v2"` | `"Deploy v2"` |
| `text` | string | `"notes"` | `"notes"` |
| `long_text` | string or `{text}` | `"multi-line"` | `{"text":"multi-line"}` |
| `numbers` | number or numeric string | `42.5` | `"42.5"` |
| `status` | label (case-insensitive), label ID, `{label}`, `{index}` | `"done"` | `{"label":"Done"}` |
| `dropdown` | label, list of labels, or list of label IDs | `["npm","pip"]` | `{"labels":["npm","pip"]}` |
| `date` | `YYYY-MM-DD`, `YYYY-MM-DD HH:MM`, `{date,time}` | `"2026-10-01 09:30"` | `{"date":"2026-10-01","time":"09:30:00"}` |
| `timeline` | `{from,to}` ordered dates | `{"from":"2026-10-01","to":"2026-10-15"}` | same |
| `people` | user ID, list of IDs, `"team:<id>"`, `{personsAndTeams}` | `["12345678","team:7"]` | `{"personsAndTeams":[{"id":12345678,"kind":"person"},{"id":7,"kind":"team"}]}` |
| `checkbox` | boolean | `true` | `{"checked":"true"}` (false clears) |
| `email` | address or `{email,text}` | `"ops@example.com"` | `{"email":…,"text":…}` |
| `link` | http(s) URL or `{url,text}` | `{"url":"https://…","text":"Runbook"}` | same |
| `phone` | `"CC:number"` or `{phone,countryShortName}` | `"NI:+50588887777"` | `{"phone":"+50588887777","countryShortName":"NI"}` |
| `rating` | integer 0–5 | `4` | `{"rating":4}` |
| `hour` | `HH:MM` or `{hour,minute}` | `"09:30"` | `{"hour":9,"minute":30}` |
| `week` | any date in the week, or `{startDate,endDate}` | `"2026-10-01"` | Monday–Sunday week |
| `country` | `{countryCode,countryName}` | `{"countryCode":"NI","countryName":"Nicaragua"}` | same (code upper-cased) |
| `location` | `{lat,lng,address}` | `{"lat":12.13,"lng":-86.25,"address":"Managua"}` | same |
| `world_clock` | IANA time zone | `"America/Managua"` | `{"timezone":"America/Managua"}` |
| `tags` | list of tag IDs (see `create_or_get_tag`) | `[123]` | `{"tag_ids":[123]}` |
| `board_relation`, `dependency` | list of item IDs | `[1234567890]` | `{"item_ids":[…]}` |
| `color_picker` | hex color | `"#1F76C2"` | `{"color":{"hex":"#1F76C2"}}` |

`null` clears a column (`""` for text/numbers, `{}` otherwise).

## Rejected on purpose

- **Unknown column IDs** and **archived columns**.
- **Computed or read-only types**: `auto_number`, `button`, `creation_log`,
  `formula`, `item_id`, `last_updated`, `mirror`, `progress`, `subtasks`,
  `time_tracking`, `vote`, `file`, `doc`, `direct_doc`, `integration`,
  `item_assignees`, `group`, `unsupported`.
- **Status/dropdown labels that do not exist** or are deactivated. The error
  lists the valid labels. The server never sends `create_labels_if_missing`,
  so a typo cannot silently add a label to a shared board.
- **Impossible dates** (`2026-02-30`), inverted timelines, invalid emails,
  non-http(s) links, out-of-range ratings/hours/coordinates, unknown time zones.

## Transport detail

monday's `JSON` scalar must be sent as a **JSON-encoded string**; sending an
object is rejected with `Invalid type, expected a JSON string`. The adapter
encodes column values and column defaults accordingly
(`internal/monday/items.go`), and contract tests pin that behavior.
