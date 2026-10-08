# Setup

## Requirements

Only Docker and Docker Compose are required on the host. Go commands run inside
Docker through the Makefile.

## Configuration

Copy the committed template:

```bash
cp .env.example .env
```

Set `MONDAY_API_TOKEN` to an API token created in monday.com's developer
settings. Keep the token only in `.env` or a secret manager; `.env` is ignored
by Git.

`MONDAY_API_VERSION` is sent as Monday's `API-Version` HTTP header. The default
`2026-07` is the current stable version documented by Monday at the time this
repository was initialized. Check the [Monday API versioning
documentation](https://developer.monday.com/api-reference/docs/api-versioning)
before upgrading it. Do not use a release candidate in production.

## Run

```bash
make build
make run
```

The MCP server uses stdio. Its stdout is reserved for MCP JSON-RPC traffic and
logs are written to stderr.

## HTTP transport and per-user credentials

The stateless Streamable HTTP transport uses the official Go SDK's
`StreamableHTTPOptions{Stateless: true, JSONResponse: true}` handler. It exposes
`GET /healthz` for liveness, `GET /readyz` for readiness after configuration
loading, and the configured `MCP_HTTP_PATH` (default `/mcp`) for MCP requests.

`MCP_AUTH_MODE` defaults to `env` for stdio and `request` for Streamable HTTP.
In `request` mode, the server reads `Authorization: Bearer <token>` from every
MCP request and keeps the credential only in that request's context. It never
falls back to `MONDAY_API_TOKEN` or `MONDAY_API_TOKEN_ENV`. Missing, malformed,
or disallowed-prefix tokens return `401` with `WWW-Authenticate: Bearer`.
`MCP_CLIENT_KEY`, when set, additionally requires the matching
`X-MCP-Client-Key` header. The optional `MCP_ALLOWED_TOKEN_PREFIXES` is a
comma-separated list; Monday has no prefix filter by default.

HTTP plus `MCP_AUTH_MODE=env` is rejected unless
`MCP_ALLOW_SHARED_TOKEN=true`. This mode is intended only for a trusted local
network because all requests use the process-wide `MONDAY_API_TOKEN`.

The default host is `127.0.0.1`; set `MCP_HTTP_HOST=0.0.0.0` explicitly when
the container must accept traffic on its network interface. `MCP_HTTP_PORT`
defaults to `8080`. The Compose example intentionally does not publish a host
port.

```yaml
mcpServers:
  monday-governance:
    type: streamable-http
    url: "http://mcp-monday:8080/mcp"
    startup: false
    requiresOAuth: false
    headers:
      Authorization: "Bearer {{MONDAY_API_TOKEN}}"
      X-MCP-Client-Key: "${MCP_MONDAY_CLIENT_KEY}"
    customUserVars:
      MONDAY_API_TOKEN:
        title: "Monday.com API token"
        sensitive: true
```

The `oauth` block for a future GitHub App or other provider flow is documented
in PR 4. Do not put real tokens in this repository or in client configuration
committed to source control.

## MCP client configuration

Use the published image (`ghcr.io/jersonmartinez/mcp-monday-projects:0.3.0`,
also tagged `0.3` and `latest`; see [RELEASING.md](RELEASING.md)) or build
your own with `make build`, which tags `mcp-monday-projects:local`. Register
the server in any MCP client that supports stdio. The client starts one
container per session:

```json
{
  "mcpServers": {
    "monday": {
      "command": "docker",
      "args": ["run", "--rm", "-i", "--env-file", "/path/to/mcp-monday-projects/.env", "ghcr.io/jersonmartinez/mcp-monday-projects:0.3.0"]
    }
  }
}
```

Add `-e MCP_ACCESS_LEVEL=read` (or `full`, a workspace scope, or an allowlist, see
[CAPABILITIES.md](CAPABILITIES.md)) to the `args` for a restricted profile.
Clients that honor MCP tool annotations can require confirmation for tools
with `destructiveHint: true`.

To pin a target per client, add `"-e", "MCP_PROFILE=devops", "-v",
"/path/to/mcp-monday-projects/profiles:/profiles:ro"` to the `args`; see
[profiles](CAPABILITIES.md#profiles).

## Verify the connection

```bash
make tools                 # list registered tools and their read/write mode
make probe TOOL=get_me     # authenticated user and account (never the token)
make probe TOOL=get_api_status
```

`scripts/mcp_probe.py` accepts `-e NAME=VALUE` to test a policy, for example
`python3 scripts/mcp_probe.py --list -e MCP_ACCESS_LEVEL=read`.

## Real-account smoke suite

See [SMOKE.md](SMOKE.md). Always point the write phase at a dedicated sandbox
board; the suite pins the write allowlist to it.

## Validation

```bash
make test
make fmt-check
make vet
make validate
```

All commands execute in Docker or Docker Compose; no Go installation is
required on the host.

GitHub Actions mirrors this Docker-first validation:

- **CI** builds the builder image and runs `gofmt`, `go mod verify`, the
  tests, the coverage gate (`scripts/coverage_gate.sh`, ≥ 80% on
  `internal/application` and `internal/monday`), race-enabled tests and
  `go vet`, then builds the runtime image.
- **PR Checks** validates the PR title (Conventional Commits) and branch
  name, and lints YAML, shell scripts, Markdown and relative doc links —
  the same checks as `make lint`.
- **Security** runs a gitleaks secret scan on pull requests, pushes to
  `main` and a weekly schedule.
- **Labels sync** applies `.github/labels.yaml` (never deletes a label) and
  **Docs Wiki sync** publishes `docs/` to the Wiki once the Wiki exists.

The GraphQL transport retries only transient network failures, HTTP 429,
HTTP 5xx responses, and monday complexity/rate-limit GraphQL errors (honoring
`retry_in_seconds`). Retries are bounded by `MCP_MAX_RETRIES` and honor a
bounded `Retry-After` header. GraphQL validation errors and other permanent
errors are returned without retrying as typed `RequestError`s carrying
monday's error code. Nil GraphQL variables are omitted rather than sent as
`null`, because some monday resolvers reject explicit nulls.
