"""Fail when a Markdown file links to a repository file that does not exist.

Checks every relative link ``[text](path)`` / ``[text](path#anchor)`` in the
tracked Markdown files. External (``http``/``mailto``) links and pure anchors
are skipped: they need the network or a renderer and would make CI flaky.

Usage: python3 scripts/check_doc_links.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

LINK = re.compile(r"(?<!!)\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
SKIP = ("http://", "https://", "mailto:", "#")


EXCLUDED_DIRS = {".git", "wiki_out", "node_modules"}


def tracked_markdown(root: Path) -> list[Path]:
    """Every Markdown file in the repository, minus generated/vendored dirs."""
    return sorted(
        path
        for path in root.rglob("*.md")
        if not EXCLUDED_DIRS.intersection(path.relative_to(root).parts)
    )


def broken_links(root: Path) -> list[str]:
    problems: list[str] = []
    for md in tracked_markdown(root):
        in_fence = False
        for lineno, line in enumerate(md.read_text(encoding="utf-8").splitlines(), 1):
            if line.lstrip().startswith("```"):
                in_fence = not in_fence
                continue
            if in_fence:
                continue
            for target in LINK.findall(line):
                if target.startswith(SKIP):
                    continue
                path = target.split("#", 1)[0]
                resolved = (root / path.lstrip("/")) if path.startswith("/") else (md.parent / path)
                if not resolved.exists():
                    problems.append(f"{md.relative_to(root)}:{lineno}: {target}")
    return problems


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    problems = broken_links(root)
    for problem in problems:
        print(f"broken link: {problem}", file=sys.stderr)
    if problems:
        return 1
    print("All relative Markdown links resolve.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
