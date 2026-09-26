# syntax=docker/dockerfile:1

FROM golang:1.25.0-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/mcp-monday-projects ./cmd/mcp-server

FROM golang:1.25.0-alpine AS race-builder

WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="Monday.com MCP Server" \
      org.opencontainers.image.description="Go MCP server for monday.com" \
      org.opencontainers.image.source="https://github.com/jersonmartinez/mcp-monday-projects"

COPY --from=builder /out/mcp-monday-projects /mcp-monday-projects
# Profiles: mount a profiles.yaml into the container and select it with
# MCP_PROFILE, e.g. -v "$PWD/profiles.yaml:/profiles.yaml:ro" -e
# MCP_PROFILES_FILE=/profiles.yaml (see docs/CAPABILITIES.md#profiles).
USER nonroot:nonroot
ENTRYPOINT ["/mcp-monday-projects"]
