# syntax=docker/dockerfile:1

FROM golang:1.27.1-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/mcp-monday-projects ./cmd/mcp-server

FROM golang:1.27.1-alpine AS race-builder

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
# Profiles are mounted read-only at /profiles (see docs/CAPABILITIES.md#profiles).
ENV MCP_PROFILES_DIR=/profiles
USER nonroot:nonroot
ENTRYPOINT ["/mcp-monday-projects"]
