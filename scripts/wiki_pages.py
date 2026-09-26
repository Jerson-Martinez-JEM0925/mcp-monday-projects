"""Build GitHub Wiki pages from docs/.

Each ``docs/**/*.md`` file becomes one flat Wiki page named after its path
(``docs/adr/0001-go-modular-mcp.md`` -> ``adr-0001-go-modular-mcp``;
``docs/**/README.md`` -> the folder name, and ``docs/README.md`` -> ``Home``).
Every relative link is resolved against the source file and rewritten:

* to another page under docs/  -> that page's Wiki name (anchor kept);
* to any other repository file -> its ``https://github.com/<repo>/blob/main/...`` URL.

A ``Home`` page listing every page is generated when docs/ has no README.

Usage: python3 scripts/wiki_pages.py <repo owner/name> <out_dir>
"""

from __future__ import annotations

import re
import shutil
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"
LINK = re.compile(r"(\]\()([^)\s]+)(\))")
SKIP = ("http://", "https://", "mailto:", "#")


def page_name(md: Path) -> str:
    rel = md.relative_to(DOCS).with_suffix("")
    parts = list(rel.parts)
    if parts[-1] == "README":
        parts = parts[:-1] or ["Home"]
    return "-".join(parts)


def rewrite(md: Path, text: str, repo: str) -> str:
    def repl(match: re.Match[str]) -> str:
        target = match.group(2)
        if target.startswith(SKIP):
            return match.group(0)
        path, _, anchor = target.partition("#")
        resolved = (md.parent / path).resolve()
        suffix = f"#{anchor}" if anchor else ""
        if resolved.suffix == ".md" and DOCS in resolved.parents and resolved.exists():
            new = page_name(resolved) + suffix
        else:
            new = f"https://github.com/{repo}/blob/main/{resolved.relative_to(ROOT).as_posix()}{suffix}"
        return f"{match.group(1)}{new}{match.group(3)}"

    return LINK.sub(repl, text)


def build(repo: str, out: Path) -> int:
    if out.exists():
        shutil.rmtree(out)
    out.mkdir(parents=True)
    names: list[str] = []
    for md in sorted(DOCS.rglob("*.md")):
        name = page_name(md)
        names.append(name)
        (out / f"{name}.md").write_text(rewrite(md, md.read_text(encoding="utf-8"), repo), encoding="utf-8")
    if "Home" not in names:
        lines = [
            f"# {repo.split('/')[-1]}",
            "",
            "This Wiki is generated from the repository's `docs/` folder on every push to",
            "`main`. Edit the files in the repository: manual Wiki edits are overwritten.",
            "",
            *[f"- [{name}]({name})" for name in names],
            "",
        ]
        (out / "Home.md").write_text("\n".join(lines), encoding="utf-8")
        names.append("Home")
    return len(names)


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    count = build(argv[1], Path(argv[2]))
    print(f"Built {count} Wiki pages in {argv[2]}/.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
