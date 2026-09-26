# mcp-monday-projects — Agent Guide

## Purpose

This repository contains a public, Docker-first MCP server for monday.com,
implemented in Go. The server must remain MCP-client agnostic and must not
contain IDE-specific assumptions.

## Source of truth

`AGENTS.md` is the canonical agent instruction file. Do not create or maintain
`CLAUDE.md`. Provider-specific guidance belongs in the relevant client-facing
configuration or documentation only when it adds information not already
covered here.

## Architecture

Use dependency-inward boundaries:

```text
MCP tools → application services → domain models → monday port → GraphQL transport
```

- `cmd/mcp-server/`: executable entrypoints only.
- `internal/mcpserver/`: MCP registration and protocol adapters.
- `internal/application/`: use cases and orchestration.
- `internal/domain/`: provider-independent business types and validation.
- `internal/monday/`: monday GraphQL adapter.
- `internal/config/`: validated environment configuration.

Do not put GraphQL queries directly in MCP tool handlers.

## Runtime rules

- Go is the only application language.
- Docker and Docker Compose are the only documented runtime paths.
- Never commit `.env`, tokens, personal data, or production responses.
- Logs go to stderr; MCP stdout is reserved for the protocol.
- All external calls use context, timeouts, bounded responses, and typed errors.
- Archive is the default destructive verb. Permanent deletes exist only at
  `MCP_ACCESS_LEVEL=full`, require `confirm: true`, and must pass the same
  workspace scope and write allowlists as every other mutation.

## Validation

Run `make validate` before opening a PR. It executes build, tests, formatting,
and `go vet` inside Docker. New tools require unit tests, mocked GraphQL
contract tests, and documentation in the same PR.

## GitHub workflow

- Every feature starts with an issue in the repository Project.
- If work spans more than one issue, create an epic and link child issues.
- PRs are ready for review, use Conventional Commits, and link `Closes #N`.
- Before presenting a PR as ready, verify that every required GitHub Actions check has completed successfully; pending, failed, or missing checks block delivery.
- If GitHub Actions fails, fix the failure and re-run the checks before presenting the PR again.
- Never merge a PR unless the user explicitly requests it.
