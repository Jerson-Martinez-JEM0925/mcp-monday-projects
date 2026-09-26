#!/usr/bin/env python3
"""Regenerate the tool tables in docs/TOOLS.md from the live catalog.

The server is the source of truth: this script calls `list_tool_catalog`
through the containerized MCP server and rewrites the block between the
`<!-- tools:begin -->` and `<!-- tools:end -->` markers. A unit test
(TestEveryToolIsDocumented) fails when a registered tool is missing.

Usage:
    python3 scripts/gen_tool_docs.py            # rewrite docs/TOOLS.md
    python3 scripts/gen_tool_docs.py --check    # exit 1 if it would change
"""
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from mcp_probe import call  # noqa: E402

DOC = Path(__file__).resolve().parent.parent / "docs" / "TOOLS.md"
BEGIN, END = "<!-- tools:begin -->", "<!-- tools:end -->"

TITLES = {
    "diagnostics": "Diagnostics",
    "workspaces": "Workspaces and folders",
    "boards": "Boards and templates",
    "groups": "Groups",
    "columns": "Columns",
    "items.read": "Items — read and search",
    "items.write": "Items — write",
    "bulk": "Bulk operations",
    "people": "Users and teams",
    "collaboration": "Updates and notifications",
    "tags": "Tags",
    "reports": "Reports",
}


def render() -> str:
    response = call("list_tool_catalog", {}) or {}
    catalog = (response.get("result") or {}).get("structuredContent") or {}
    tools = catalog.get("tools") or []
    if not tools:
        raise SystemExit("error: empty catalog; is the image built and .env present?")
    lines = [BEGIN, "", f"_{len(tools)} tools, generated from `list_tool_catalog`._"]
    current = None
    for tool in tools:
        if tool["category"] != current:
            current = tool["category"]
            lines += ["", f"### {TITLES.get(current, current)}", "", "| Tool | Mode | Capability | Description |", "|---|---|---|---|"]
        if tool["read_only"]:
            mode = "read"
        elif tool["destructive"]:
            mode = "write · archive"
        else:
            mode = "write"
        description = tool["description"].replace("|", "\\|")
        lines.append(f"| `{tool['name']}` | {mode} | `{tool['capability']}` | {description} |")
    lines += ["", END]
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    text = DOC.read_text(encoding="utf-8")
    if BEGIN not in text or END not in text:
        raise SystemExit(f"error: markers missing in {DOC}")
    head, rest = text.split(BEGIN, 1)
    _, tail = rest.split(END, 1)
    updated = head + render() + tail
    if "--check" in argv:
        return 0 if updated == text else 1
    DOC.write_text(updated, encoding="utf-8")
    print(f"updated {DOC}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
