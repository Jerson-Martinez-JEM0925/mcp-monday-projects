package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profiles mirror mcp-github-projects (GH_PROJECT_PROFILE): a profile pins one
// monday target — account token reference, workspace, access level, allowlists,
// report budget and API version. Profiles live in one optional YAML file
// (MCP_PROFILES_FILE, default profiles.yaml next to the binary's working dir)
// and one is selected with MCP_PROFILE=<name>.
//
// Rules:
//   - Precedence is: explicit environment variable > profile value > default.
//     A key the profile sets wins over the process default, but an explicitly
//     exported environment variable of the same name still wins over both, so
//     an operator can override a single field without editing the file.
//   - Tokens never live in the file. `token_env` names the environment
//     variable that holds this account's token (e.g. MONDAY_TOKEN_DEVOPS);
//     the value itself stays in the environment.
//   - Unknown profile name, unknown field, or an invalid value is a startup
//     error that names the profile.
//
// Example profiles.yaml:
//
//	profiles:
//	  devops:
//	    token_env: MONDAY_TOKEN_DEVOPS
//	    workspace_id: 14216815
//	    access_level: read
//	    report_max_items: 250
//	    board_allowlist: [123, 456]
//	    workspace_allowlist: [14216815]
//	    api_version: 2026-07
const defaultProfilesFile = "profiles.yaml"

var profileName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// profile is one named target parsed from the YAML file. Every field is
// optional; an omitted field falls back to the environment and then the
// default. `yaml:",flow"` is not used so that unknown keys can be rejected via
// KnownFields(true) on the decoder.
type profile struct {
	TokenEnv           string  `yaml:"token_env"`
	WorkspaceID        *uint64 `yaml:"workspace_id"`
	AccessLevel        string  `yaml:"access_level"`
	BoardAllowlist     []int64 `yaml:"board_allowlist"`
	WorkspaceAllowlist []int64 `yaml:"workspace_allowlist"`
	ReportMaxItems     *int    `yaml:"report_max_items"`
	APIVersion         string  `yaml:"api_version"`
	APIURL             string  `yaml:"api_url"`
}

// profilesFile is the top-level shape of MCP_PROFILES_FILE.
type profilesFile struct {
	Profiles map[string]profile `yaml:"profiles"`
}

// source resolves a variable: an explicit environment variable wins, then the
// value the active profile pinned, then the caller's default.
type source struct {
	profile map[string]string
	lookup  func(string) string
}

func (env source) get(name string) string {
	if value := env.lookup(name); strings.TrimSpace(value) != "" {
		return value
	}
	if value, ok := env.profile[name]; ok {
		return value
	}
	return ""
}

// token resolves the API token. MONDAY_API_TOKEN wins; otherwise the profile's
// token_env (surfaced as MONDAY_API_TOKEN_ENV) names the variable that holds it.
func (env source) token() (string, error) {
	if token := strings.TrimSpace(env.lookup("MONDAY_API_TOKEN")); token != "" {
		return token, nil
	}
	if ref := strings.TrimSpace(env.profile["MONDAY_API_TOKEN_ENV"]); ref != "" {
		if !envName.MatchString(ref) {
			return "", fmt.Errorf("token_env must name an environment variable (uppercase, e.g. MONDAY_TOKEN_DEVOPS), got %q", ref)
		}
		token := strings.TrimSpace(env.lookup(ref))
		if token == "" {
			return "", fmt.Errorf("token_env=%s but that environment variable is empty", ref)
		}
		return token, nil
	}
	return "", fmt.Errorf("MONDAY_API_TOKEN is required (set it, or point the profile's token_env at a variable that holds it)")
}

// loadSource reads MCP_PROFILE and, when set, the profile from MCP_PROFILES_FILE.
// It returns the resolution source, the active profile name, and any error.
func loadSource() (source, string, error) {
	env := source{lookup: os.Getenv}
	name := strings.TrimSpace(os.Getenv("MCP_PROFILE"))
	if name == "" {
		return env, "", nil
	}
	if !profileName.MatchString(name) {
		return source{}, "", fmt.Errorf("MCP_PROFILE must be a lowercase name (a-z, 0-9, - or _), got %q", name)
	}
	path := strings.TrimSpace(os.Getenv("MCP_PROFILES_FILE"))
	if path == "" {
		path = defaultProfilesFile
	}
	values, err := readProfile(path, name)
	if err != nil {
		return source{}, "", err
	}
	env.profile = values
	return env, name, nil
}

// readProfile parses the YAML file, selects the named profile, rejects unknown
// fields and invalid values, and flattens the profile into the environment-key
// namespace the rest of config.go already understands.
func readProfile(path, name string) (map[string]string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("profile %q: cannot open profiles file %s (set MCP_PROFILES_FILE or copy profiles.example.yaml to %s): %w", name, path, defaultProfilesFile, err)
	}
	defer file.Close()

	var parsed profilesFile
	dec := yaml.NewDecoder(file)
	dec.KnownFields(true) // reject unknown fields so a typo fails at startup
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("profile %q: %s is not valid YAML: %w", name, path, err)
	}
	if len(parsed.Profiles) == 0 {
		return nil, fmt.Errorf("profile %q: %s defines no profiles (expected a top-level `profiles:` map)", name, path)
	}
	p, ok := parsed.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q: not found in %s (known profiles: %s)", name, path, strings.Join(profileNames(parsed.Profiles), ", "))
	}

	values := map[string]string{}
	if v := strings.TrimSpace(p.TokenEnv); v != "" {
		if !envName.MatchString(v) {
			return nil, fmt.Errorf("profile %q: token_env must name an environment variable (uppercase, e.g. MONDAY_TOKEN_DEVOPS), got %q", name, v)
		}
		values["MONDAY_API_TOKEN_ENV"] = v
	}
	if p.WorkspaceID != nil {
		values["MONDAY_WORKSPACE_ID"] = strconv.FormatUint(*p.WorkspaceID, 10)
	}
	if v := strings.TrimSpace(p.AccessLevel); v != "" {
		values["MCP_ACCESS_LEVEL"] = v
	}
	if list, err := idList(name, "board_allowlist", p.BoardAllowlist); err != nil {
		return nil, err
	} else if list != "" {
		values["MONDAY_WRITE_BOARD_ALLOWLIST"] = list
	}
	if list, err := idList(name, "workspace_allowlist", p.WorkspaceAllowlist); err != nil {
		return nil, err
	} else if list != "" {
		values["MONDAY_WRITE_WORKSPACE_ALLOWLIST"] = list
	}
	if p.ReportMaxItems != nil {
		values["MCP_REPORT_MAX_ITEMS"] = strconv.Itoa(*p.ReportMaxItems)
	}
	if v := strings.TrimSpace(p.APIVersion); v != "" {
		values["MONDAY_API_VERSION"] = v
	}
	if v := strings.TrimSpace(p.APIURL); v != "" {
		values["MONDAY_API_URL"] = v
	}
	return values, nil
}

// idList renders a []int64 of positive IDs as the comma-separated form config.go
// parses. A non-positive ID is rejected naming the profile and field.
func idList(profileName, field string, ids []int64) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return "", fmt.Errorf("profile %q: %s must be positive numeric IDs, got %d", profileName, field, id)
		}
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ","), nil
}

// profileNames returns the profile keys sorted, for a helpful "not found" error.
func profileNames(profiles map[string]profile) []string {
	names := make([]string, 0, len(profiles))
	for n := range profiles {
		names = append(names, n)
	}
	// simple insertion sort keeps the file dependency-free of "sort"
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}
