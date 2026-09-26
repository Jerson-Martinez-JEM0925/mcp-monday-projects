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

## MCP client configuration

Build the image once (`make build`), then register the server in any MCP
client that supports stdio. The client starts one container per session:

```json
{
  "mcpServers": {
    "monday": {
      "command": "docker",
      "args": ["run", "--rm", "-i", "--env-file", "/path/to/mcp-monday-projects/.env", "mcp-monday-projects:local"]
    }
  }
}
```

Add `-e MCP_READ_ONLY=true` (or an allowlist, see
[CAPABILITIES.md](CAPABILITIES.md)) to the `args` for a restricted profile.
Clients that honor MCP tool annotations can require confirmation for tools
with `destructiveHint: true`.

## Verify the connection

```bash
make tools                 # list registered tools and their read/write mode
make probe TOOL=get_me     # authenticated user and account (never the token)
make probe TOOL=get_api_status
```

`scripts/mcp_probe.py` accepts `-e NAME=VALUE` to test a policy, for example
`python3 scripts/mcp_probe.py --list -e MCP_READ_ONLY=true`.

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

GitHub Actions mirrors this Docker-first validation: the Go checks build the
builder image and run formatting, module-integrity verification, tests, and
`go vet` inside it. The security workflow runs a repository secret scan on
pull requests, pushes to `main`, and the weekly schedule.

The GraphQL transport retries only transient network failures, HTTP 429,
HTTP 5xx responses, and monday complexity/rate-limit GraphQL errors (honoring
`retry_in_seconds`). Retries are bounded by `MCP_MAX_RETRIES` and honor a
bounded `Retry-After` header. GraphQL validation errors and other permanent
errors are returned without retrying as typed `RequestError`s carrying
monday's error code. Nil GraphQL variables are omitted rather than sent as
`null`, because some monday resolvers reject explicit nulls.
