#!/usr/bin/env python3
"""Minimal stdio MCP probe for mcp-monday-projects.

Spawns the containerized MCP server over stdio, performs the MCP
handshake, calls one tool, prints the JSON result, and exits.

Usage:
    python3 scripts/mcp_probe.py <TOOL> [JSON_ARGS] [-e NAME=VALUE ...]

    -e NAME=VALUE   extra environment for the container (repeatable), e.g.
                    -e MCP_READ_ONLY=true or
                    -e MONDAY_WRITE_BOARD_ALLOWLIST=123
    --list          list registered tools instead of calling one

The Monday API token is read from the .env file by `docker run
--env-file`; this script never reads or prints the token.
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parent.parent
_ENV_FILE = _REPO_ROOT / ".env"
_IMAGE = os.environ.get("MCP_PROBE_IMAGE", "mcp-monday-projects:local")
_PROTOCOL_VERSION = "2025-06-18"


class Session:
    """One MCP stdio session against the containerized server."""

    def __init__(self, extra_env: list[str] | None = None) -> None:
        command = ["docker", "run", "--rm", "-i", "--env-file", str(_ENV_FILE)]
        for entry in extra_env or []:
            command += ["-e", entry]
        command.append(_IMAGE)
        self._proc = subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL, text=True,
        )
        self._next_id = 1
        self._request("initialize", {
            "protocolVersion": _PROTOCOL_VERSION,
            "capabilities": {},
            "clientInfo": {"name": "mcp_probe", "version": "2"},
        })
        self._send({"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}})

    def _send(self, obj: dict) -> None:
        assert self._proc.stdin is not None
        self._proc.stdin.write(json.dumps(obj) + "\n")
        self._proc.stdin.flush()

    def _request(self, method: str, params: dict) -> dict:
        request_id = self._next_id
        self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
        assert self._proc.stdout is not None
        while True:
            line = self._proc.stdout.readline()
            if not line:
                raise RuntimeError("server closed the stream")
            try:
                message = json.loads(line)
            except json.JSONDecodeError:
                continue
            if message.get("id") == request_id:
                return message

    def call(self, tool: str, arguments: dict) -> dict:
        return self._request("tools/call", {"name": tool, "arguments": arguments})

    def list_tools(self) -> dict:
        return self._request("tools/list", {})

    def close(self) -> None:
        if self._proc.stdin is not None:
            self._proc.stdin.close()
        try:
            self._proc.wait(timeout=30)
        except subprocess.TimeoutExpired:
            self._proc.kill()


def call(tool: str, arguments: dict, extra_env: list[str] | None = None) -> dict | None:
    session = Session(extra_env)
    try:
        return session.call(tool, arguments)
    finally:
        session.close()


def main(argv: list[str]) -> int:
    extra: list[str] = []
    positional: list[str] = []
    list_mode = False
    index = 0
    while index < len(argv):
        if argv[index] == "-e" and index + 1 < len(argv):
            extra.append(argv[index + 1])
            index += 2
            continue
        if argv[index] == "--list":
            list_mode = True
        else:
            positional.append(argv[index])
        index += 1
    if list_mode:
        session = Session(extra)
        try:
            tools = session.list_tools().get("result", {}).get("tools", [])
        finally:
            session.close()
        for tool in tools:
            hints = tool.get("annotations") or {}
            print(f"{tool['name']:32} {'RO' if hints.get('readOnlyHint') else 'RW'}  {tool.get('description', '')[:80]}")
        print(f"{len(tools)} tools")
        return 0
    if not positional:
        sys.stderr.write("usage: mcp_probe.py <TOOL> [JSON_ARGS] [-e NAME=VALUE ...] | --list\n")
        return 2
    tool = positional[0]
    arguments = json.loads(positional[1]) if len(positional) > 1 else {}
    response = call(tool, arguments, extra)
    if response is None:
        sys.stderr.write("error: no response from server\n")
        return 1
    print(json.dumps(response, indent=2, ensure_ascii=False))
    result = response.get("result") or {}
    return 1 if (result.get("isError") or response.get("error")) else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
