package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProfile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_PROFILES_DIR", dir)
	t.Setenv("MCP_PROFILE", name)
	return dir
}

func clearPolicyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"MCP_ACCESS_LEVEL", "MCP_READ_ONLY", "MONDAY_WORKSPACE_ID", "MONDAY_WRITE_BOARD_ALLOWLIST", "MONDAY_WRITE_WORKSPACE_ALLOWLIST", "MCP_REPORT_MAX_ITEMS"} {
		t.Setenv(name, "")
	}
}

func TestProfileSetsTargetAndWinsOverEnvironment(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	t.Setenv("MONDAY_TOKEN_DEVOPS", "devops-token")
	t.Setenv("MONDAY_WORKSPACE_ID", "999") // must not override the profile
	t.Setenv("MCP_REPORT_MAX_ITEMS", "250")
	writeProfile(t, "devops", `# DevOps sandbox
MONDAY_API_TOKEN_ENV=MONDAY_TOKEN_DEVOPS
MONDAY_WORKSPACE_ID=14216815
MCP_ACCESS_LEVEL="read"
export MONDAY_WRITE_BOARD_ALLOWLIST=1,2
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "devops" || cfg.APIToken != "devops-token" || cfg.WorkspaceID != "14216815" || cfg.AccessLevel != AccessRead || !cfg.ReadOnly {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.WriteBoardAllowlist) != 2 || cfg.ReportMaxItems != 250 {
		t.Fatalf("allowlist/report = %v / %d (unset keys must fall back to the environment)", cfg.WriteBoardAllowlist, cfg.ReportMaxItems)
	}
}

func TestProfileFallsBackToDefaultToken(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	writeProfile(t, "plain", "MCP_ACCESS_LEVEL=full\n")
	cfg, err := Load()
	if err != nil || cfg.APIToken != "default-token" || cfg.AccessLevel != AccessFull {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestProfileErrors(t *testing.T) {
	cases := map[string]struct{ name, body, want string }{
		"token in file":    {"leak", "MONDAY_API_TOKEN=abc\n", "not allowed"},
		"unknown key":      {"typo", "MONDAY_WORKSPACE=1\n", "unknown key"},
		"bad line":         {"bad", "just-text\n", "KEY=VALUE"},
		"empty token env":  {"empty", "MONDAY_API_TOKEN_ENV=MONDAY_TOKEN_MISSING\n", "is empty"},
		"bad token env":    {"lower", "MONDAY_API_TOKEN_ENV=lower\n", "must name"},
		"invalid level":    {"level", "MCP_ACCESS_LEVEL=admin\n", "MCP_ACCESS_LEVEL"},
		"invalid scope":    {"scope", "MONDAY_WORKSPACE_ID=DevOps\n", "numeric"},
		"nested selection": {"nest", "MCP_PROFILE=other\n", "not allowed"},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			clearPolicyEnv(t)
			t.Setenv("MONDAY_API_TOKEN", "default-token")
			t.Setenv("MONDAY_TOKEN_MISSING", "")
			writeProfile(t, tc.name, tc.body)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("err = %v, want mention of %q and the profile name", err, tc.want)
			}
		})
	}
}

func TestProfileNameAndMissingFile(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	t.Setenv("MCP_PROFILES_DIR", t.TempDir())
	for _, name := range []string{"../etc", "Dev Ops", "UPPER"} {
		t.Setenv("MCP_PROFILE", name)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "lowercase") {
			t.Fatalf("name %q: err = %v", name, err)
		}
	}
	t.Setenv("MCP_PROFILE", "absent")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "cannot open") {
		t.Fatalf("missing file err = %v", err)
	}
}

func TestNoProfileKeepsEnvironmentBehaviour(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MCP_PROFILE", "")
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	t.Setenv("MONDAY_WORKSPACE_ID", "7")
	cfg, err := Load()
	if err != nil || cfg.Profile != "" || cfg.WorkspaceID != "7" || cfg.APIToken != "default-token" {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestExampleProfileIsValid(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_TOKEN_EXAMPLE", "example-token")
	t.Setenv("MCP_PROFILES_DIR", filepath.Join("..", "..", "profiles"))
	t.Setenv("MCP_PROFILE", "example")
	if _, err := Load(); err != nil {
		t.Fatalf("profiles/example.env must load: %v", err)
	}
}
