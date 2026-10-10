# Monday update formatting

The `create_update` and `reply_to_update` tools accept Markdown-like input from an MCP client, but Monday's GraphQL `create_update(body: ...)` field renders the body as HTML/plain text. It does not render GitHub Markdown. The server therefore converts the input through `internal/monday/update_format.go` before sending it to Monday.

## Supported input

- `#` through `######` headings → `h1`–`h6`
- Paragraphs and line breaks → `p` and `br`
- `**bold**`, `*italic*`, `~~strike~~`
- Inline backticks → `code`
- Fenced code blocks with optional language → `pre > code`
- Unordered and ordered lists → `ul`/`ol` and `li`
- Pipe tables with a separator row → `table`, `thead`, `tbody`, `th`, `td`
- HTTP(S) and `mailto:` links → `a`
- Plain placeholders such as `[VM_NAME]`, `${PROJECT}` and `<VM_NAME>`

## Safety and sanitization

Raw HTML is escaped instead of being passed through. This keeps technical text visible and prevents Monday's sanitizer from silently removing tags or attributes. Links are emitted only for `http`, `https` and `mailto` schemes; unsafe schemes remain visible as escaped text. Code blocks and inline code are escaped before being wrapped in HTML, so shell operators, variables, angle brackets and backticks are preserved.

The renderer emits only the small HTML subset needed for update readability. It is intentionally separate from the Markdown renderers used by reports and GitHub-facing content.

## Limits and verification

The application keeps the existing 20,000-character provider limit, applying it to the formatted HTML body. Unit tests cover headings, links, code, backticks, lists, tables, raw HTML, placeholders and unsafe links. To verify a real account without exposing credentials:

```bash
python3 scripts/mcp_probe.py create_update \
  '{"item_id":"<existing-item-id>","body":"## Probe\\n\\n```bash\\ngcloud projects describe [PROJECT]\\n```"}'
python3 scripts/mcp_probe.py list_item_updates \
  '{"item_id":"<existing-item-id>","limit":5}'
```

Use an existing item only. Do not create a task solely for a formatting probe, and do not include tokens, passwords, private URLs or database data.

## Editing existing updates

Use the `edit_update` MCP tool when an existing comment must be re-rendered. It validates that the `update_id` belongs to the supplied `item_id`, applies the same formatter, and calls Monday's official `edit_update` mutation. This preserves the original update, its history and its URL; do not create a second comment for a formatting-only migration.
