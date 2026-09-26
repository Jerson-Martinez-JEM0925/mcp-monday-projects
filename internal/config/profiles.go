package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Profiles mirror mcp-github-projects (GH_PROJECT_PROFILE): a profile is a
// dotenv-style file profiles/<name>.env that pins one monday target — account
// token reference, workspace, access level, allowlists. It is selected with
// MCP_PROFILE=<name>; MCP_PROFILES_DIR overrides the directory (default
// "profiles", i.e. /profiles inside the container).
//
// Rules:
//   - A profile is an explicit target: keys it sets win over the process
//     environment, so a stray variable cannot silently widen it.
//   - Tokens never live in profile files. MONDAY_API_TOKEN is forbidden;
//     MONDAY_API_TOKEN_ENV names the environment variable that holds the
//     token for this account (e.g. MONDAY_TOKEN_DEVOPS).
//   - Only known keys are accepted, so a typo fails at startup.

var profileName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// profileKeys lists the variables a profile file may set.
var profileKeys = map[string]bool{
	"MONDAY_API_TOKEN_ENV": true, "MONDAY_API_VERSION": true, "MONDAY_API_URL": true,
	"MCP_ACCESS_LEVEL": true, "MONDAY_WORKSPACE_ID": true,
	"MONDAY_WRITE_BOARD_ALLOWLIST": true, "MONDAY_WRITE_WORKSPACE_ALLOWLIST": true,
	"MCP_REPORT_MAX_ITEMS": true, "MCP_HTTP_TIMEOUT": true, "MCP_MAX_RESPONSE_BYTES": true,
	"MCP_MAX_RETRIES": true, "MCP_LOG_LEVEL": true,
}

// source resolves a variable: profile value first, then the environment.
type source struct {
	profile map[string]string
	lookup  func(string) string
}

func (env source) get(name string) string {
	if value, ok := env.profile[name]; ok {
		return value
	}
	return env.lookup(name)
}

// token resolves the API token, honouring MONDAY_API_TOKEN_ENV.
func (env source) token() (string, error) {
	if ref := strings.TrimSpace(env.get("MONDAY_API_TOKEN_ENV")); ref != "" {
		if !envName.MatchString(ref) {
			return "", fmt.Errorf("MONDAY_API_TOKEN_ENV must name an environment variable, got %q", ref)
		}
		token := strings.TrimSpace(env.lookup(ref))
		if token == "" {
			return "", fmt.Errorf("MONDAY_API_TOKEN_ENV=%s but that environment variable is empty", ref)
		}
		return token, nil
	}
	token := strings.TrimSpace(env.lookup("MONDAY_API_TOKEN"))
	if token == "" {
		return "", fmt.Errorf("MONDAY_API_TOKEN is required")
	}
	return token, nil
}

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func loadSource() (source, string, error) {
	env := source{lookup: os.Getenv}
	name := strings.TrimSpace(os.Getenv("MCP_PROFILE"))
	if name == "" {
		return env, "", nil
	}
	if !profileName.MatchString(name) {
		return source{}, "", fmt.Errorf("MCP_PROFILE must be a lowercase name (a-z, 0-9, - or _), got %q", name)
	}
	dir := strings.TrimSpace(os.Getenv("MCP_PROFILES_DIR"))
	if dir == "" {
		dir = "profiles"
	}
	values, err := readProfile(filepath.Join(dir, name+".env"))
	if err != nil {
		return source{}, "", fmt.Errorf("profile %q: %w", name, err)
	}
	env.profile = values
	return env, name, nil
}

// readProfile parses a small dotenv subset: KEY=VALUE lines, # comments,
// optional surrounding quotes. Unknown and token keys are rejected.
func readProfile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s (set MCP_PROFILES_DIR or create the file from profiles/example.env)", path)
	}
	defer file.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(text, "export "), "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, line)
		}
		if key == "MONDAY_API_TOKEN" || key == "MCP_PROFILE" || key == "MCP_PROFILES_DIR" {
			return nil, fmt.Errorf("%s:%d: %s is not allowed in a profile; tokens come from the environment (use MONDAY_API_TOKEN_ENV)", path, line, key)
		}
		if !profileKeys[key] {
			return nil, fmt.Errorf("%s:%d: unknown key %s", path, line, key)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values, scanner.Err()
}
