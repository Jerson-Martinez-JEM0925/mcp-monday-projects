#!/usr/bin/env python3
"""Reproducible real-account smoke suite for mcp-monday-projects.

Drives the containerized MCP server over stdio exactly like an MCP client
would, and records a pass/fail line per tool call. It never deletes
anything: the only destructive verb it uses is archive, and only on items
the suite itself created inside the sandbox board.

Phases
  read       Always. Account, API budget, catalog, workspaces, boards,
             users, teams, tags, formats, templates. No writes.
  guard      Always. Proves read-only mode hides write tools and that the
             allowlist refuses a board outside it before calling monday.
  sandbox    With --board. Full write/read/report cycle on ONE sandbox
             board, with MONDAY_WRITE_BOARD_ALLOWLIST pinned to it.
  provision  With --provision. Creates a board from the "devops" template
             in --workspace (allowlisted) with validated seed items.

Usage
  python3 scripts/smoke.py [--board ID] [--workspace ID] [--provision]
                           [--notify-user ID] [--assign-user ID]
                           [--report PATH]

The token comes from .env via `docker run --env-file`; this script never
reads or prints it.
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from mcp_probe import Session  # noqa: E402


class Suite:
    def __init__(self) -> None:
        self.results: list[dict] = []

    def run(self, session: Session, phase: str, tool: str, args: dict, expect_error: bool = False, check=None):
        started = time.monotonic()
        response = session.call(tool, args)
        elapsed = int((time.monotonic() - started) * 1000)
        result = response.get("result") or {}
        is_error = bool(result.get("isError") or response.get("error"))
        payload = result.get("structuredContent")
        message = ""
        if is_error:
            content = result.get("content") or [{}]
            message = (content[0].get("text") or json.dumps(response.get("error")))[:300]
        ok = is_error == expect_error
        if ok and check is not None and not is_error:
            try:
                verdict = check(payload)
                if verdict is not True:
                    ok, message = False, f"check failed: {verdict}"
            except Exception as exc:  # noqa: BLE001 - report any check failure
                ok, message = False, f"check raised: {exc}"
        self.results.append({"phase": phase, "tool": tool, "ok": ok, "ms": elapsed, "expected_error": expect_error, "message": message})
        mark = "PASS" if ok else "FAIL"
        note = f" — {message}" if message else ""
        print(f"[{mark}] {phase:9} {tool:30} {elapsed:5}ms{note}", flush=True)
        return payload

    def summary(self) -> tuple[int, int]:
        passed = sum(1 for r in self.results if r["ok"])
        return passed, len(self.results)

    def markdown(self, meta: dict) -> str:
        passed, total = self.summary()
        lines = [
            "# Smoke report — mcp-monday-projects", "",
            f"- Generated: {dt.datetime.now(dt.timezone.utc).isoformat(timespec='seconds')}",
        ]
        lines += [f"- {key}: {value}" for key, value in meta.items()]
        lines += [f"- Result: **{passed}/{total} passed**", "", "| Phase | Tool | Result | ms | Note |", "|---|---|---|---:|---|"]
        for r in self.results:
            status = "✅" if r["ok"] else "❌"
            if r["expected_error"] and r["ok"]:
                status = "✅ (refused as expected)"
            note = r["message"].replace("|", "\\|").replace("\n", " ")[:160]
            lines.append(f"| {r['phase']} | `{r['tool']}` | {status} | {r['ms']} | {note} |")
        return "\n".join(lines) + "\n"


def read_phase(suite: Suite) -> dict:
    s = Session()
    ctx: dict = {}
    try:
        info = suite.run(s, "read", "server_info", {}, check=lambda p: p["tool_count"] >= 70 or p)
        ctx["tool_count"] = (info or {}).get("tool_count")
        me = suite.run(s, "read", "get_me", {}, check=lambda p: bool(p["me"]["id"]) or p)
        ctx["me"] = (me or {}).get("me", {})
        suite.run(s, "read", "get_api_status", {}, check=lambda p: p["complexity"]["before"] > 0 or p)
        suite.run(s, "read", "list_tool_catalog", {"category": "reports"}, check=lambda p: p["count"] >= 10 or p)
        suite.run(s, "read", "list_workspaces", {"limit": 5}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "read", "list_boards", {"limit": 5, "order_by": "used_at"}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "read", "list_folders", {"limit": 5})
        suite.run(s, "read", "list_users", {"limit": 5}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "read", "search_users", {"name": ctx["me"].get("name", "a").split()[0]}, check=lambda p: p["count"] >= 1 or p)
        if ctx["me"].get("id"):
            suite.run(s, "read", "get_user", {"user_id": ctx["me"]["id"]})
        suite.run(s, "read", "list_teams", {})
        suite.run(s, "read", "list_tags", {})
        suite.run(s, "read", "describe_column_formats", {}, check=lambda p: len(p["formats"]) > 20 or p)
        suite.run(s, "read", "list_board_templates", {}, check=lambda p: len(p["templates"]) == 4 or p)
        suite.run(s, "read", "list_items", {"board_id": "abc"}, expect_error=True)
    finally:
        s.close()
    return ctx


def guard_phase(suite: Suite, board: str | None) -> None:
    ro = Session(["MCP_READ_ONLY=true"])
    try:
        suite.run(ro, "guard", "server_info", {}, check=lambda p: (p["write_policy"]["read_only"] and p["hidden_write_tools"] > 0) or p)
        suite.run(ro, "guard", "create_item", {"board_id": "1", "name": "x"}, expect_error=True)
    finally:
        ro.close()
    if board:
        guarded = Session([f"MONDAY_WRITE_BOARD_ALLOWLIST={board}"])
        try:
            # Board "1" does not exist and is not allowlisted: refused locally.
            suite.run(guarded, "guard", "create_group", {"board_id": "1", "name": "never created"}, expect_error=True)
        finally:
            guarded.close()


def ensure_columns(suite: Suite, s: Session, board: str) -> dict:
    schema = suite.run(s, "sandbox", "get_board_schema", {"board_id": board}) or {}
    existing = {c["id"] for c in schema.get("schema", {}).get("columns", [])}
    wanted = [
        {"column_id": "status", "title": "Status", "column_type": "status", "labels": ["To Do", "Working on it", "Stuck", "Done"]},
        {"column_id": "priority", "title": "Priority", "column_type": "status", "labels": ["Critical", "High", "Medium", "Low"]},
        {"column_id": "environment", "title": "Environment", "column_type": "dropdown", "labels": ["dev", "staging", "prod"]},
        {"column_id": "owner", "title": "Owner", "column_type": "people"},
        {"column_id": "due_date", "title": "Due date", "column_type": "date"},
        {"column_id": "estimate", "title": "Estimate (pts)", "column_type": "numbers"},
        {"column_id": "pr_link", "title": "Pull request", "column_type": "link"},
        {"column_id": "notes", "title": "Notes", "column_type": "long_text"},
        {"column_id": "tags", "title": "Tags", "column_type": "tags"},
    ]
    for column in wanted:
        if column["column_id"] not in existing:
            suite.run(s, "sandbox", "create_column", {"board_id": board, **column})
    return schema


def sandbox_phase(suite: Suite, board: str, assign_user: str | None, notify_user: str | None) -> dict:
    s = Session([f"MONDAY_WRITE_BOARD_ALLOWLIST={board}"])
    ctx: dict = {}
    today = dt.date.today()
    stamp = dt.datetime.now().strftime("%Y%m%d-%H%M")
    try:
        ensure_columns(suite, s, board)
        suite.run(s, "sandbox", "update_column", {"board_id": board, "column_id": "estimate", "description": "Story points (smoke-managed)"})
        suite.run(s, "sandbox", "update_board", {"board_id": board, "attribute": "description", "value": f"Sandbox E2E de mcp-monday-projects. Última corrida smoke: {stamp} UTC. Nada aquí se elimina."})
        groups = suite.run(s, "sandbox", "list_board_groups", {"board_id": board}) or {"groups": []}
        titles = {g["title"]: g["id"] for g in groups["groups"]}
        for title in ["Backlog", "In Progress", "Done"]:
            if title not in titles:
                created = suite.run(s, "sandbox", "create_group", {"board_id": board, "name": title})
                if created:
                    titles[title] = created["group"]["id"]
        suite.run(s, "sandbox", "list_board_columns", {"board_id": board}, check=lambda p: p["count"] >= 9 or p)

        # Validation: one invalid plan (no write), one valid plan.
        suite.run(s, "sandbox", "validate_column_values", {"board_id": board, "column_values": {"status": "Nope", "estimate": "abc", "due_date": "2026-13-40", "ghost": 1}},
                  check=lambda p: (not p["plan"]["valid"] and len(p["plan"]["issues"]) == 4) or p)
        values = {
            "status": "Working on it", "priority": "High", "environment": ["staging", "prod"],
            "due_date": (today - dt.timedelta(days=3)).isoformat(), "estimate": 5,
            "pr_link": {"url": "https://github.com/jersonmartinez/mcp-monday-projects", "text": "Repo"},
            "notes": "Creado por la suite smoke; valores validados por tipo antes de escribir.",
        }
        if assign_user:
            values["owner"] = [assign_user]
        suite.run(s, "sandbox", "validate_column_values", {"board_id": board, "column_values": values}, check=lambda p: p["plan"]["valid"] or p)
        suite.run(s, "sandbox", "create_item", {"board_id": board, "name": "rejected item", "column_values": {"status": "Nope"}}, expect_error=True)

        item = suite.run(s, "sandbox", "create_item", {"board_id": board, "group_id": titles.get("In Progress", ""), "name": f"[smoke {stamp}] Pipeline de despliegue", "column_values": values},
                         check=lambda p: any(v["id"] == "status" and v["text"] == "Working on it" for v in p["item"]["column_values"]) or p)
        item_id = item["item"]["id"] if item else None
        ctx["item_id"] = item_id
        second = suite.run(s, "sandbox", "create_item", {"board_id": board, "group_id": titles.get("Backlog", ""), "name": f"[smoke {stamp}] Rotación de secretos",
                                                       "column_values": {"status": "Stuck", "priority": "Critical", "estimate": "3", "due_date": (today + dt.timedelta(days=7)).isoformat()}})
        second_id = second["item"]["id"] if second else None
        if not item_id:
            return ctx

        suite.run(s, "sandbox", "update_item_column_values", {"board_id": board, "item_id": item_id, "column_values": {"estimate": 8, "notes": {"text": "Estimación ajustada"}}})
        suite.run(s, "sandbox", "set_item_status", {"board_id": board, "item_id": item_id, "label": "working on it"}, check=lambda p: p["column_id"] == "status" or p)
        suite.run(s, "sandbox", "set_item_date", {"board_id": board, "item_id": item_id, "date": (today - dt.timedelta(days=2)).isoformat()}, check=lambda p: p["column_id"] == "due_date" or p)
        if assign_user:
            suite.run(s, "sandbox", "assign_item_people", {"board_id": board, "item_id": item_id, "user_ids": [assign_user]}, check=lambda p: p["column_id"] == "owner" or p)
        suite.run(s, "sandbox", "rename_item", {"board_id": board, "item_id": item_id, "name": f"[smoke {stamp}] Pipeline de despliegue CI/CD"})
        tag = suite.run(s, "sandbox", "create_or_get_tag", {"board_id": board, "name": "mcp-smoke"})
        if tag:
            suite.run(s, "sandbox", "update_item_column_values", {"board_id": board, "item_id": item_id, "column_values": {"tags": [tag["tag"]["id"]]}})

        suite.run(s, "sandbox", "create_subitem", {"parent_item_id": item_id, "name": "Configurar runner"})
        suite.run(s, "sandbox", "list_subitems", {"item_id": item_id}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "sandbox", "get_item", {"item_id": item_id}, check=lambda p: p["item"]["board_id"] == board or p)
        suite.run(s, "sandbox", "get_items", {"item_ids": [i for i in [item_id, second_id] if i]})

        dup = suite.run(s, "sandbox", "duplicate_item", {"board_id": board, "item_id": item_id})
        dup_id = dup["item"]["id"] if dup else None
        if dup_id and "Done" in titles:
            suite.run(s, "sandbox", "move_item", {"item_id": dup_id, "group_id": titles["Done"]})
            suite.run(s, "sandbox", "set_item_status", {"board_id": board, "item_id": dup_id, "label": "Done"})

        # Reads with pagination, search and filters.
        page = suite.run(s, "sandbox", "list_items", {"board_id": board, "limit": 1})
        if page and page.get("cursor"):
            suite.run(s, "sandbox", "list_items", {"board_id": board, "limit": 1, "cursor": page["cursor"]}, check=lambda p: p["count"] == 1 or p)
        if "In Progress" in titles:
            suite.run(s, "sandbox", "list_group_items", {"board_id": board, "group_id": titles["In Progress"]}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "sandbox", "search_items", {"board_id": board, "text": "Pipeline"}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "sandbox", "search_items", {"board_id": board, "filter": {"rules": [{"column_id": "priority", "compare_value": [4], "operator": "any_of"}]}})
        suite.run(s, "sandbox", "find_items_by_column_values", {"board_id": board, "columns": [{"column_id": "status", "column_values": ["Stuck"]}]}, check=lambda p: p["count"] >= 1 or p)

        # Collaboration.
        update = suite.run(s, "sandbox", "create_update", {"item_id": item_id, "body": "<p>Smoke: pipeline configurado. <b>Siguiente</b>: validar staging.</p>"})
        if update:
            update_id = update["update"]["id"]
            suite.run(s, "sandbox", "reply_to_update", {"item_id": item_id, "update_id": update_id, "body": "Respuesta automática de la suite smoke."})
            suite.run(s, "sandbox", "like_update", {"item_id": item_id, "update_id": update_id})
        suite.run(s, "sandbox", "list_item_updates", {"item_id": item_id}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "sandbox", "list_board_updates", {"board_id": board, "limit": 5})  # board feed is eventually consistent
        if notify_user:
            suite.run(s, "sandbox", "notify_user", {"user_id": notify_user, "target_id": item_id, "text": "mcp-monday-projects smoke: revisa el item de prueba en el sandbox DevOps."})

        # Bulk: dry-run first, then execute a safe update.
        rows = [{"item_id": i, "column_values": {"estimate": 13}} for i in [item_id, second_id] if i]
        suite.run(s, "sandbox", "bulk_update_items", {"board_id": board, "updates": rows}, check=lambda p: (p["result"]["dry_run"] and p["result"]["succeeded"] == 0) or p)
        suite.run(s, "sandbox", "bulk_update_items", {"board_id": board, "updates": [{"item_id": item_id, "column_values": {"status": "Nope"}}], "dry_run": False},
                  check=lambda p: p["result"]["failed"] == 1 and p["result"]["succeeded"] == 0 or p)
        suite.run(s, "sandbox", "bulk_update_items", {"board_id": board, "updates": rows, "dry_run": False}, check=lambda p: p["result"]["succeeded"] == len(rows) or p)
        if "Backlog" in titles and second_id:
            suite.run(s, "sandbox", "bulk_move_items", {"item_ids": [second_id], "group_id": titles["Backlog"]}, check=lambda p: p["result"]["dry_run"] or p)
        if dup_id:
            suite.run(s, "sandbox", "bulk_archive_items", {"item_ids": [dup_id]}, check=lambda p: p["result"]["dry_run"] and p["result"]["planned"] == 1 or p)
            # Archive (not delete) the suite's own duplicate to exercise archive semantics.
            suite.run(s, "sandbox", "archive_item", {"item_id": dup_id}, check=lambda p: p["item"]["state"] == "archived" or p)
        suite.run(s, "sandbox", "archive_board", {"board_id": board}, expect_error=True)  # refused without confirm

        # Reports.
        suite.run(s, "reports", "board_summary", {"board_id": board}, check=lambda p: p["summary"]["total_items"] >= 2 or p)
        suite.run(s, "reports", "column_distribution", {"board_id": board, "column_id": "priority"})
        suite.run(s, "reports", "workload_report", {"board_id": board})
        suite.run(s, "reports", "overdue_items", {"board_id": board}, check=lambda p: p["count"] >= 1 or p)
        suite.run(s, "reports", "stale_items", {"board_id": board, "days": 1})
        suite.run(s, "reports", "daily_standup", {"board_id": board}, check=lambda p: "Standup" in p["standup"]["markdown"] or p)
        health = suite.run(s, "reports", "board_health_report", {"board_id": board}, check=lambda p: 0 <= p["report"]["score"] <= 100 or p)
        ctx["health"] = (health or {}).get("report", {})
        suite.run(s, "reports", "export_board_markdown", {"board_id": board}, check=lambda p: p["content"].startswith("# ") or p)
        suite.run(s, "reports", "export_board_csv", {"board_id": board}, check=lambda p: p["content"].startswith("item_id,") or p)
        schema = suite.run(s, "reports", "get_board", {"board_id": board})
        if schema and schema["board"].get("workspace_id"):
            suite.run(s, "reports", "workspace_overview", {"workspace_id": schema["board"]["workspace_id"]}, check=lambda p: p["overview"]["board_count"] >= 1 or p)
        ctx["board_url"] = (schema or {}).get("board", {}).get("url")
    finally:
        s.close()
    return ctx


def provision_phase(suite: Suite, workspace: str, assign_user: str | None) -> dict:
    s = Session([f"MONDAY_WRITE_WORKSPACE_ALLOWLIST={workspace}", "MONDAY_WRITE_BOARD_ALLOWLIST=1"])
    today = dt.date.today()
    owner = {"owner": [assign_user]} if assign_user else {}
    seed = [
        {"group": "Backlog", "name": "Estandarizar Dockerfiles multi-stage", "values": {"status": "To Do", "priority": "Medium", "environment": ["dev"], "estimate": 3, **owner}},
        {"group": "In Progress", "name": "Pipeline CI con escaneo de secretos", "values": {"status": "Working on it", "priority": "High", "environment": ["staging"], "due_date": (today + dt.timedelta(days=5)).isoformat(), "estimate": 5, **owner}},
        {"group": "In Progress", "name": "Migrar Terraform state a backend remoto", "values": {"status": "Stuck", "priority": "Critical", "environment": ["prod"], "due_date": (today - dt.timedelta(days=1)).isoformat(), "estimate": 8}},
        {"group": "Code Review", "name": "Alertas de latencia p95", "values": {"status": "Working on it", "priority": "High", "environment": ["prod"], "pr_link": {"url": "https://github.com/jersonmartinez/mcp-monday-projects", "text": "PR"}}},
        {"group": "Done", "name": "Runbook de rollback", "values": {"status": "Done", "priority": "Low", "environment": ["prod"], "estimate": 2}},
    ]
    try:
        result = suite.run(s, "provision", "provision_board_from_template", {
            "template": "devops", "name": "DevOps Delivery · MCP template", "workspace_id": workspace, "board_kind": "private",
            "description": "Board generado con provision_board_from_template (mcp-monday-projects). Demuestra columnas tipadas, grupos y seeds validados.",
            "seed_items": seed,
        }, check=lambda p: (len(p["result"]["items"]) == len(seed) and len(p["result"]["columns"]) == 8) or p)
        board = (result or {}).get("result", {}).get("board", {})
        if board.get("id"):
            suite.run(s, "provision", "board_health_report", {"board_id": board["id"]})
            if assign_user:
                suite.run(s, "provision", "add_board_subscribers", {"board_id": board["id"], "user_ids": [assign_user], "kind": "owner"})
        return {"board": board, "warnings": (result or {}).get("result", {}).get("warnings")}
    finally:
        s.close()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--board", help="sandbox board ID for the write cycle")
    parser.add_argument("--workspace", help="workspace ID allowed for --provision")
    parser.add_argument("--provision", action="store_true", help="create a devops template board in --workspace")
    parser.add_argument("--assign-user", help="user ID to assign as owner in sandbox items")
    parser.add_argument("--notify-user", help="user ID to receive one test notification")
    parser.add_argument("--report", help="write a Markdown report to this path")
    args = parser.parse_args()

    suite = Suite()
    meta: dict = {}
    read_ctx = read_phase(suite)
    meta["Tools registered"] = read_ctx.get("tool_count")
    guard_phase(suite, args.board)
    if args.board:
        ctx = sandbox_phase(suite, args.board, args.assign_user, args.notify_user)
        meta["Sandbox board"] = ctx.get("board_url") or args.board
        if ctx.get("health"):
            meta["Sandbox health"] = f"{ctx['health'].get('score')}/100 (grade {ctx['health'].get('grade')})"
    if args.provision:
        if not args.workspace:
            parser.error("--provision requires --workspace")
        ctx = provision_phase(suite, args.workspace, args.assign_user)
        if ctx.get("board"):
            meta["Provisioned board"] = ctx["board"].get("url")
        if ctx.get("warnings"):
            meta["Provision warnings"] = "; ".join(ctx["warnings"])
    passed, total = suite.summary()
    print(f"\n{passed}/{total} passed")
    if args.report:
        Path(args.report).write_text(suite.markdown(meta), encoding="utf-8")
        print(f"report written to {args.report}")
    return 0 if passed == total else 1


if __name__ == "__main__":
    raise SystemExit(main())
