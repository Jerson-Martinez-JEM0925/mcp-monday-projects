package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadUsesMondayDefaults(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	t.Setenv("MONDAY_API_URL", "")
	t.Setenv("MONDAY_API_VERSION", "")
	t.Setenv("MCP_HTTP_TIMEOUT", "")
	t.Setenv("MCP_MAX_RESPONSE_BYTES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIVersion != defaultAPIVersion {
		t.Fatalf("APIVersion = %q, want %q", cfg.APIVersion, defaultAPIVersion)
	}
	if cfg.APIURL != defaultAPIURL {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, defaultAPIURL)
	}
	if cfg.HTTPTimeout != 15*time.Second {
		t.Fatalf("HTTPTimeout = %s, want 15s", cfg.HTTPTimeout)
	}
}

func TestLoadRequiresToken(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing-token error")
	}
}

func TestLoadRejectsNonHTTPSURL(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	t.Setenv("MONDAY_API_URL", "http://localhost:8080")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want URL validation error")
	}
	_ = os.Unsetenv("MONDAY_API_URL")
}

func TestLoadWritePolicy(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	t.Setenv("MCP_ACCESS_LEVEL", "read")
	t.Setenv("MONDAY_WRITE_BOARD_ALLOWLIST", " 123, 456 ,")
	t.Setenv("MONDAY_WRITE_WORKSPACE_ALLOWLIST", "789")
	t.Setenv("MCP_REPORT_MAX_ITEMS", "250")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.ReadOnly || len(cfg.WriteBoardAllowlist) != 2 || cfg.WriteBoardAllowlist[1] != "456" || cfg.WriteWorkspaceAllowlist[0] != "789" || cfg.ReportMaxItems != 250 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadRejectsInvalidWritePolicy(t *testing.T) {
	cases := map[string]string{
		"MONDAY_WRITE_BOARD_ALLOWLIST":     "12,abc",
		"MONDAY_WRITE_WORKSPACE_ALLOWLIST": "-1",
		"MCP_REPORT_MAX_ITEMS":             "0",
		"MONDAY_WORKSPACE_ID":              "DevOps",
		"MCP_READ_ONLY":                    "true",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("MONDAY_API_TOKEN", "test-token")
			t.Setenv(name, value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%s", name, value)
			}
		})
	}
}

func TestLoadServerInstructionsAndWriteToolAllowlist(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	t.Setenv("MCP_SERVER_INSTRUCTIONS", " custom instructions ")
	t.Setenv("MCP_WRITE_TOOL_ALLOWLIST", " create_item, archive_item, create_item ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerInstructions != "custom instructions" {
		t.Fatalf("ServerInstructions = %q", cfg.ServerInstructions)
	}
	if len(cfg.WriteToolAllowlist) != 2 || cfg.WriteToolAllowlist[0] != "create_item" || cfg.WriteToolAllowlist[1] != "archive_item" {
		t.Fatalf("WriteToolAllowlist = %v", cfg.WriteToolAllowlist)
	}
}

func TestLoadRejectsOversizedServerInstructions(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	t.Setenv("MCP_SERVER_INSTRUCTIONS", strings.Repeat("x", maxServerInstructions+1))
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MCP_SERVER_INSTRUCTIONS") {
		t.Fatalf("Load() error = %v, want instruction length error", err)
	}
}

func TestLoadWorkspaceScope(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	t.Setenv("MONDAY_WORKSPACE_ID", " 14216815 ")
	cfg, err := Load()
	if err != nil || cfg.WorkspaceID != "14216815" {
		t.Fatalf("WorkspaceID = %q, err = %v", cfg.WorkspaceID, err)
	}
	t.Setenv("MONDAY_WORKSPACE_ID", "")
	if cfg, err := Load(); err != nil || cfg.WorkspaceID != "" {
		t.Fatalf("unset scope = %q, err = %v", cfg.WorkspaceID, err)
	}
	t.Setenv("MONDAY_WORKSPACE_ID", "1,2")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a list for MONDAY_WORKSPACE_ID")
	}
}

func TestLoadAccessLevel(t *testing.T) {
	cases := []struct {
		level, want  string
		readOnlyWant bool
	}{
		{"", AccessWrite, false},
		{"read", AccessRead, true},
		{"WRITE", AccessWrite, false},
		{"full", AccessFull, false},
	}
	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			t.Setenv("MONDAY_API_TOKEN", "test-token")
			t.Setenv("MCP_ACCESS_LEVEL", tc.level)
			t.Setenv("MCP_READ_ONLY", "")
			cfg, err := Load()
			if err != nil || cfg.AccessLevel != tc.want || cfg.ReadOnly != tc.readOnlyWant {
				t.Fatalf("level=%q: got %q/%v, err %v", tc.level, cfg.AccessLevel, cfg.ReadOnly, err)
			}
		})
	}
	for _, bad := range []string{"admin", "read-only"} {
		t.Setenv("MCP_ACCESS_LEVEL", bad)
		t.Setenv("MCP_READ_ONLY", "")
		if _, err := Load(); err == nil {
			t.Fatalf("Load() accepted MCP_ACCESS_LEVEL=%s", bad)
		}
	}
	t.Setenv("MCP_ACCESS_LEVEL", "write")
	t.Setenv("MCP_READ_ONLY", "true")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted removed MCP_READ_ONLY")
	}
}

// MCP_LOG_LEVEL used to be read but ignored (logs were always INFO); it is
// now validated and normalized so cmd/mcp-server can apply it.
func TestLoadLogLevel(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	for _, tc := range []struct{ in, want string }{
		{"", "info"}, {"DEBUG", "debug"}, {" warn ", "warn"}, {"error", "error"},
	} {
		t.Setenv("MCP_LOG_LEVEL", tc.in)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("MCP_LOG_LEVEL=%q: Load() error = %v", tc.in, err)
		}
		if cfg.LogLevel != tc.want {
			t.Fatalf("MCP_LOG_LEVEL=%q: LogLevel = %q, want %q", tc.in, cfg.LogLevel, tc.want)
		}
	}
	t.Setenv("MCP_LOG_LEVEL", "verbose")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want MCP_LOG_LEVEL validation error")
	}
}

func TestLoadTransportDefaults(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "test-token")
	for name := range map[string]bool{"MCP_TRANSPORT": true, "MCP_HTTP_HOST": true, "MCP_HTTP_PORT": true, "MCP_HTTP_PATH": true} {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Transport != "stdio" || cfg.HTTPHost != "127.0.0.1" || cfg.HTTPPort != 8080 || cfg.HTTPPath != "/mcp" {
		t.Fatalf("HTTP config = %+v", cfg)
	}
}

func TestLoadRejectsInvalidTransportConfig(t *testing.T) {
	cases := map[string]string{
		"MCP_TRANSPORT":   "http",
		"MCP_HTTP_PORT":   "0",
		"MCP_HTTP_PATH":   "mcp",
		"MCP_HTTP_PATH_Q": "/mcp?x=1",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("MONDAY_API_TOKEN", "test-token")
			if name == "MCP_HTTP_PATH_Q" {
				t.Setenv("MCP_HTTP_PATH", value)
			} else {
				t.Setenv(name, value)
			}
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%s", name, value)
			}
		})
	}
}

func TestLoadHTTPRequestAuthDoesNotRequireEnvironmentToken(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "")
	t.Setenv("MCP_TRANSPORT", "streamable-http")
	t.Setenv("MCP_AUTH_MODE", "request")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIToken != "" || cfg.AuthMode != "request" {
		t.Fatalf("request auth config = %+v", cfg)
	}
}

func TestLoadHTTPEnvAuthRequiresExplicitSharedToken(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "shared")
	t.Setenv("MCP_TRANSPORT", "streamable-http")
	t.Setenv("MCP_AUTH_MODE", "env")
	t.Setenv("MCP_ALLOW_SHARED_TOKEN", "false")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted shared HTTP auth without explicit opt-in")
	}
	t.Setenv("MCP_ALLOW_SHARED_TOKEN", "true")
	cfg, err := Load()
	if err != nil || cfg.AuthMode != "env" || !cfg.AllowSharedToken {
		t.Fatalf("shared auth config = %+v, err=%v", cfg, err)
	}
}

func TestLoadAuthOptions(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "shared")
	t.Setenv("MCP_TRANSPORT", "streamable-http")
	t.Setenv("MCP_AUTH_MODE", "request")
	t.Setenv("MCP_CLIENT_KEY", "client-secret")
	t.Setenv("MCP_ALLOWED_TOKEN_PREFIXES", " gh_, monday_ ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ClientKey != "client-secret" || len(cfg.AllowedTokenPrefixes) != 2 || cfg.AllowedTokenPrefixes[1] != "monday_" {
		t.Fatalf("auth options = %+v", cfg)
	}
}
