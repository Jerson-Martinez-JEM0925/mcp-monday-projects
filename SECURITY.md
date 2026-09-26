# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub Security Advisories](https://github.com/jersonmartinez/mcp-monday-projects/security/advisories/new).
Do not open public issues for security problems. Expect an acknowledgement
within five business days.

## Handling secrets

- `MONDAY_API_TOKEN` lives only in `.env` (git-ignored and docker-ignored) or
  a secret manager. The server never logs, returns, or echoes it; `get_me`
  and `server_info` expose identity and policy only.
- The smoke and probe scripts pass the token with `docker run --env-file`
  and never read it themselves.
- CI runs a repository secret scan on every pull request.

## Runtime hardening

- HTTPS-only API URL, bounded timeouts, bounded response size, bounded retries.
- Logs go to stderr; stdout carries only MCP JSON-RPC.
- The runtime image is distroless and runs as `nonroot`.
- No delete operations; destructive verbs archive and require confirmation.
- Optional read-only mode and board/workspace write allowlists
  ([docs/CAPABILITIES.md](docs/CAPABILITIES.md)).

## Supported versions

Only the latest release on `main` receives security fixes.
