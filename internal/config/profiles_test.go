package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfiles writes a profiles.yaml into a temp dir, points MCP_PROFILES_FILE
// at it and selects MCP_PROFILE=name.
func writeProfiles(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_PROFILES_FILE", path)
	t.Setenv("MCP_PROFILE", name)
	return path
}

// clearPolicyEnv removes the environment variables a profile pins so a test
// starts from a clean slate.
func clearPolicyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"MCP_ACCESS_LEVEL", "MCP_READ_ONLY", "MONDAY_WORKSPACE_ID",
		"MONDAY_WRITE_BOARD_ALLOWLIST", "MONDAY_WRITE_WORKSPACE_ALLOWLIST",
		"MCP_REPORT_MAX_ITEMS", "MONDAY_API_VERSION", "MONDAY_API_URL",
	} {
		t.Setenv(name, "")
	}
}

func TestProfileSetsTargetFromYAML(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "")
	t.Setenv("MONDAY_TOKEN_DEVOPS", "devops-token")
	writeProfiles(t, "devops", `
profiles:
  devops:
    token_env: MONDAY_TOKEN_DEVOPS
    workspace_id: 14216815
    access_level: read
    report_max_items: 250
    board_allowlist: [1, 2]
    api_version: "2026-01"
  other:
    token_env: MONDAY_TOKEN_OTHER
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "devops" || cfg.APIToken != "devops-token" || cfg.WorkspaceID != "14216815" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.AccessLevel != AccessRead || !cfg.ReadOnly {
		t.Fatalf("access level = %q readonly = %v", cfg.AccessLevel, cfg.ReadOnly)
	}
	if len(cfg.WriteBoardAllowlist) != 2 || cfg.ReportMaxItems != 250 || cfg.APIVersion != "2026-01" {
		t.Fatalf("allowlist/report/version = %v / %d / %q", cfg.WriteBoardAllowlist, cfg.ReportMaxItems, cfg.APIVersion)
	}
}

// TestExplicitEnvVarWinsOverProfile locks in the spec precedence:
// explicit env var > profile > default.
func TestExplicitEnvVarWinsOverProfile(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "")
	t.Setenv("MONDAY_TOKEN_DEVOPS", "devops-token")
	t.Setenv("MONDAY_WORKSPACE_ID", "999") // explicit env overrides the profile
	t.Setenv("MCP_REPORT_MAX_ITEMS", "17") // explicit env overrides the profile
	writeProfiles(t, "devops", `
profiles:
  devops:
    token_env: MONDAY_TOKEN_DEVOPS
    workspace_id: 14216815
    report_max_items: 250
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceID != "999" || cfg.ReportMaxItems != 17 {
		t.Fatalf("explicit env must win: workspace=%q report=%d", cfg.WorkspaceID, cfg.ReportMaxItems)
	}
}

// TestExplicitTokenWinsOverTokenEnv: MONDAY_API_TOKEN wins over token_env.
func TestExplicitTokenWinsOverTokenEnv(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "explicit-token")
	t.Setenv("MONDAY_TOKEN_DEVOPS", "devops-token")
	writeProfiles(t, "devops", `
profiles:
  devops:
    token_env: MONDAY_TOKEN_DEVOPS
`)
	cfg, err := Load()
	if err != nil || cfg.APIToken != "explicit-token" {
		t.Fatalf("cfg = %+v err = %v", cfg, err)
	}
}

func TestProfileFallsBackToDefaultToken(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	writeProfiles(t, "plain", `
profiles:
  plain:
    access_level: full
`)
	cfg, err := Load()
	if err != nil || cfg.APIToken != "default-token" || cfg.AccessLevel != AccessFull {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestProfileErrors(t *testing.T) {
	cases := map[string]struct {
		name, body, want string
		// clearToken exercises the token_env path by removing the default token
		// (an explicit MONDAY_API_TOKEN would legitimately win over token_env).
		clearToken bool
	}{
		"unknown profile":    {name: "missing", body: "profiles:\n  other:\n    access_level: read\n", want: "not found"},
		"unknown field":      {name: "typo", body: "profiles:\n  typo:\n    workspace: 1\n", want: "field workspace not found"},
		"empty token env":    {name: "empty", body: "profiles:\n  empty:\n    token_env: MONDAY_TOKEN_MISSING\n", want: "is empty", clearToken: true},
		"bad token env":      {name: "lower", body: "profiles:\n  lower:\n    token_env: lower\n", want: "must name", clearToken: true},
		"invalid level":      {name: "level", body: "profiles:\n  level:\n    access_level: admin\n", want: "MCP_ACCESS_LEVEL"},
		"bad workspace type": {name: "scope", body: "profiles:\n  scope:\n    workspace_id: DevOps\n", want: "cannot unmarshal"},
		"bad allowlist id":   {name: "neg", body: "profiles:\n  neg:\n    board_allowlist: [0]\n", want: "must be positive"},
		"no profiles map":    {name: "none", body: "boards:\n  - 1\n", want: "field boards not found"},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			clearPolicyEnv(t)
			if tc.clearToken {
				t.Setenv("MONDAY_API_TOKEN", "")
			} else {
				t.Setenv("MONDAY_API_TOKEN", "default-token")
			}
			t.Setenv("MONDAY_TOKEN_MISSING", "")
			writeProfiles(t, tc.name, tc.body)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want mention of %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("err = %v must name the profile %q", err, tc.name)
			}
		})
	}
}

func TestProfileNameValidation(t *testing.T) {
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	dir := t.TempDir()
	t.Setenv("MCP_PROFILES_FILE", filepath.Join(dir, "profiles.yaml"))
	for _, name := range []string{"../etc", "Dev Ops", "UPPER"} {
		t.Setenv("MCP_PROFILE", name)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "lowercase") {
			t.Fatalf("name %q: err = %v", name, err)
		}
	}
}

func TestMissingProfilesFile(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_API_TOKEN", "default-token")
	t.Setenv("MCP_PROFILES_FILE", filepath.Join(t.TempDir(), "absent.yaml"))
	t.Setenv("MCP_PROFILE", "devops")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "cannot open") || !strings.Contains(err.Error(), "devops") {
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

// TestExampleProfilesFileIsValid loads the committed profiles.example.yaml so a
// broken example fails CI.
func TestExampleProfilesFileIsValid(t *testing.T) {
	clearPolicyEnv(t)
	t.Setenv("MONDAY_TOKEN_DEVOPS", "example-token")
	t.Setenv("MCP_PROFILES_FILE", filepath.Join("..", "..", "profiles.example.yaml"))
	t.Setenv("MCP_PROFILE", "devops")
	if _, err := Load(); err != nil {
		t.Fatalf("profiles.example.yaml profile 'devops' must load: %v", err)
	}
}
