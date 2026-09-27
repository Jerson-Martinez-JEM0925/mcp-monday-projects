package config

import (
	"os"
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
	t.Setenv("MCP_READ_ONLY", "true")
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
		"MCP_READ_ONLY":                    "maybe",
		"MONDAY_WRITE_BOARD_ALLOWLIST":     "12,abc",
		"MONDAY_WRITE_WORKSPACE_ALLOWLIST": "-1",
		"MCP_REPORT_MAX_ITEMS":             "0",
		"MONDAY_WORKSPACE_ID":              "DevOps",
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
		level, readOnly, want string
		readOnlyWant          bool
	}{
		{"", "", AccessWrite, false},
		{"read", "", AccessRead, true},
		{"WRITE", "", AccessWrite, false},
		{"full", "", AccessFull, false},
		{"", "true", AccessRead, true},
		{"read", "true", AccessRead, true},
	}
	for _, tc := range cases {
		t.Setenv("MONDAY_API_TOKEN", "test-token")
		t.Setenv("MCP_ACCESS_LEVEL", tc.level)
		t.Setenv("MCP_READ_ONLY", tc.readOnly)
		cfg, err := Load()
		if err != nil || cfg.AccessLevel != tc.want || cfg.ReadOnly != tc.readOnlyWant {
			t.Fatalf("level=%q read_only=%q: got %q/%v, err %v", tc.level, tc.readOnly, cfg.AccessLevel, cfg.ReadOnly, err)
		}
	}
	for _, bad := range [][2]string{{"admin", ""}, {"full", "true"}, {"write", "true"}} {
		t.Setenv("MCP_ACCESS_LEVEL", bad[0])
		t.Setenv("MCP_READ_ONLY", bad[1])
		if _, err := Load(); err == nil {
			t.Fatalf("Load() accepted MCP_ACCESS_LEVEL=%s MCP_READ_ONLY=%s", bad[0], bad[1])
		}
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
