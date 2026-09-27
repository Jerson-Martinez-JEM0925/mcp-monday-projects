package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
	mcpserver "github.com/jersonmartinez/mcp-monday-projects/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// MCP_LOG_LEVEL is validated by config.Load (debug|info|warn|error), so
	// UnmarshalText cannot fail here; logs always go to stderr.
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	logger.Info("starting mcp-monday-projects", "api_version", cfg.APIVersion, "profile", cfg.Profile, "access_level", cfg.AccessLevel)
	server := mcpserver.New(cfg)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Error("MCP server stopped", "error", err)
		os.Exit(1)
	}
}
