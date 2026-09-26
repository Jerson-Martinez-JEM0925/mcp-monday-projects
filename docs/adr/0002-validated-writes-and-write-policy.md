# ADR 0002: Validated writes, write policy, and archive-only mutations

- Status: Accepted
- Date: 2026-09-26
- Issues: #15, #16, #17, #18 (epic #14)

## Context

The operational MVP exposes ~70 tools against real monday.com accounts that
are shared by whole organizations. Three failure modes matter most:

1. **Malformed writes.** monday accepts column values as a JSON-encoded
   string whose shape depends on the column type. Invalid labels either fail
   late with opaque errors or, with `create_labels_if_missing`, silently add
   labels to shared boards.
2. **Blast radius.** An MCP client driven by a model can call any registered
   tool. A token is user-scoped and usually far more powerful than the task.
3. **Irreversibility.** Deleting items or boards cannot be undone.

## Decision

- **Validate in the domain layer.** `domain.PlanColumnValues` converts friendly
  input into monday JSON using the live board schema (column types, status and
  dropdown labels). Writes fail locally with per-column issues. The same plan is
  exposed read-only through `validate_column_values`.
- **Enforce a write policy in the application layer.** `WriteGuard` supports
  read-only mode (write tools are not registered at all) and board/workspace
  allowlists. Item writes resolve the owning board first. Boards created by the
  process are trusted for its lifetime so provisioning works under a strict list.
- **Archive, never delete.** No delete tool exists. Board and group archive
  require `confirm: true`; bulk tools default to `dry_run: true`, are capped at
  50 rows, and a bulk update writes nothing unless every row validates.
- **Keep the adapter honest.** The GraphQL adapter omits nil variables (monday
  rejects some explicit nulls), encodes JSON scalars as strings, types GraphQL
  errors (`RequestError.Code`), and retries only transient codes.
- **Reports are pure.** Reports run on a bounded snapshot and live in
  `internal/domain`, so they are unit-tested without HTTP and flag truncation.

## Consequences

- Unsupported or future column types require a raw JSON object; computed types
  are rejected explicitly.
- Guarded item writes cost one extra read to resolve the board when an
  allowlist is active.
- The tool catalog is data (`list_tool_catalog`), which lets tests assert that
  every tool is documented and that annotations match the policy.
