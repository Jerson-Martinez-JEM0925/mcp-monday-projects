# Contributing

Thanks for contributing to `mcp-monday-projects`.

## Workflow

1. Open or select an issue in [Project #12](https://github.com/users/jersonmartinez/projects/12).
2. Create an epic when a change spans more than one issue.
3. Create a branch using `feat/`, `fix/`, `docs/`, `chore/`, or `test/`.
4. Implement the smallest coherent change.
5. Add tests and documentation in the same PR.
6. Run `make validate`.
7. Wait for every required GitHub Actions check to complete successfully; pending, failed, or missing checks block delivery.
8. Open a PR ready for review and include `Closes #N`.

CI and security workflows run on pull requests targeting `main` or an intermediate feature branch, so stacked PRs receive the same required validation.

## Development rules

- Go and Docker Compose are the supported development stack.
- Do not run Go commands directly on the host; use Makefile targets.
- Never commit `.env`, API tokens, or real monday.com data.
- Keep MCP stdout protocol-clean; write diagnostics to stderr.
- Use contexts, bounded I/O, typed errors, and explicit validation.
- Keep statement coverage of `internal/application` and `internal/monday` at
  80% or more. `make coverage` (part of `make validate`) and the CI
  "Coverage gate" step run `scripts/coverage_gate.sh` and fail below the
  threshold. New adapter methods need a contract test in
  `internal/monday/adapter_contract_test.go`; new use cases need a case in
  `internal/application/usecases_test.go`.
- Do not create `CLAUDE.md`; `AGENTS.md` is canonical.

## Commit style

Use Conventional Commits, for example `feat: add board discovery tool`.

## Review checklist

Reviewers verify behavior, tests, documentation, security, Docker reproducibility,
and that the change remains provider-aware rather than leaking GraphQL details
into MCP handlers.
