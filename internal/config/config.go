package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultAPIURL = "https://api.monday.com/v2"
const defaultAPIVersion = "2026-07"

// Config contains validated runtime configuration for the MCP server.
type Config struct {
	APIToken             string
	Transport            string
	HTTPHost             string
	HTTPPort             int
	HTTPPath             string
	AuthMode             string
	AllowSharedToken     bool
	ClientKey            string
	AllowedTokenPrefixes []string
	APIVersion           string
	APIURL               string
	LogLevel             string
	HTTPTimeout          time.Duration
	MaxResponseBytes     int64
	MaxRetries           int
	// AccessLevel is read, write (default), or full. It decides which tools
	// are registered: read hides every mutation, write exposes create/update/
	// archive, and full also exposes permanent deletes.
	AccessLevel string
	// ReadOnly is true at AccessLevel=read (mutation tools are not registered).
	ReadOnly bool
	// WriteBoardAllowlist restricts mutations to these board IDs when set.
	WriteBoardAllowlist []string
	// WriteWorkspaceAllowlist restricts board/folder creation to these workspaces.
	WriteWorkspaceAllowlist []string
	// WorkspaceID scopes every read and write to one workspace when set.
	WorkspaceID string
	// ReportMaxItems bounds how many items a report loads per board.
	ReportMaxItems int
	// Profile is the active MCP_PROFILE name ("" when none).
	Profile string
}

// Load reads and validates configuration from environment variables and, when
// MCP_PROFILE is set, from that profile file (see profiles.go).
func Load() (Config, error) {
	env, profile, err := loadSource()
	if err != nil {
		return Config{}, err
	}
	cfg, err := env.config()
	if err != nil {
		if profile != "" {
			return Config{}, fmt.Errorf("profile %q: %w", profile, err)
		}
		return Config{}, err
	}
	cfg.Profile = profile
	return cfg, nil
}

func (env source) config() (Config, error) {
	transport := strings.ToLower(strings.TrimSpace(env.valueOrDefault("MCP_TRANSPORT", "stdio")))
	if transport != "stdio" && transport != "streamable-http" {
		return Config{}, fmt.Errorf("MCP_TRANSPORT must be stdio or streamable-http")
	}
	authMode, err := env.parseAuthMode(transport)
	if err != nil {
		return Config{}, err
	}
	allowShared, err := env.parseBool("MCP_ALLOW_SHARED_TOKEN", false)
	if err != nil {
		return Config{}, err
	}
	if transport == "streamable-http" && authMode == "env" && !allowShared {
		return Config{}, fmt.Errorf("streamable-http with MCP_AUTH_MODE=env requires MCP_ALLOW_SHARED_TOKEN=true")
	}

	var token string
	if authMode == "env" {
		token, err = env.token()
		if err != nil {
			return Config{}, err
		}
	}

	host := env.valueOrDefault("MCP_HTTP_HOST", "127.0.0.1")
	port, err := env.parseHTTPPort()
	if err != nil {
		return Config{}, err
	}
	path := env.valueOrDefault("MCP_HTTP_PATH", "/mcp")
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#") {
		return Config{}, fmt.Errorf("MCP_HTTP_PATH must be an absolute path without query or fragment")
	}
	prefixes := env.parseTokenPrefixes()

	apiURL := env.valueOrDefault("MONDAY_API_URL", defaultAPIURL)
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return Config{}, fmt.Errorf("MONDAY_API_URL must be a valid HTTPS URL")
	}

	timeout, err := env.parseDuration("MCP_HTTP_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxResponseBytes, err := env.parseInt64("MCP_MAX_RESPONSE_BYTES", 4*1024*1024)
	if err != nil || maxResponseBytes < 1024 {
		return Config{}, fmt.Errorf("MCP_MAX_RESPONSE_BYTES must be at least 1024 bytes")
	}
	retries, err := env.parseInt64("MCP_MAX_RETRIES", 2)
	if err != nil || retries < 0 || retries > 5 {
		return Config{}, fmt.Errorf("MCP_MAX_RETRIES must be between 0 and 5")
	}

	level, err := env.parseAccessLevel()
	if err != nil {
		return Config{}, err
	}
	boards, err := env.parseIDList("MONDAY_WRITE_BOARD_ALLOWLIST")
	if err != nil {
		return Config{}, err
	}
	workspaces, err := env.parseIDList("MONDAY_WRITE_WORKSPACE_ALLOWLIST")
	if err != nil {
		return Config{}, err
	}
	workspaceID := strings.TrimSpace(env.get("MONDAY_WORKSPACE_ID"))
	if workspaceID != "" {
		if _, err := strconv.ParseUint(workspaceID, 10, 64); err != nil {
			return Config{}, fmt.Errorf("MONDAY_WORKSPACE_ID must be a single numeric workspace ID (names are not unique in monday)")
		}
	}
	reportMax, err := env.parseInt64("MCP_REPORT_MAX_ITEMS", 500)
	if err != nil || reportMax < 1 || reportMax > 5000 {
		return Config{}, fmt.Errorf("MCP_REPORT_MAX_ITEMS must be between 1 and 5000")
	}
	logLevel := strings.ToLower(strings.TrimSpace(env.valueOrDefault("MCP_LOG_LEVEL", "info")))
	switch logLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("MCP_LOG_LEVEL must be debug, info, warn, or error")
	}

	return Config{
		APIToken:                token,
		Transport:               transport,
		HTTPHost:                host,
		HTTPPort:                port,
		HTTPPath:                path,
		AuthMode:                authMode,
		AllowSharedToken:        allowShared,
		ClientKey:               strings.TrimSpace(env.lookup("MCP_CLIENT_KEY")),
		AllowedTokenPrefixes:    prefixes,
		APIVersion:              env.valueOrDefault("MONDAY_API_VERSION", defaultAPIVersion),
		APIURL:                  apiURL,
		LogLevel:                logLevel,
		HTTPTimeout:             timeout,
		MaxResponseBytes:        maxResponseBytes,
		MaxRetries:              int(retries),
		AccessLevel:             level,
		ReadOnly:                level == AccessRead,
		WriteBoardAllowlist:     boards,
		WriteWorkspaceAllowlist: workspaces,
		WorkspaceID:             workspaceID,
		ReportMaxItems:          int(reportMax),
	}, nil
}

// Access levels accepted by MCP_ACCESS_LEVEL.
const (
	AccessRead  = "read"
	AccessWrite = "write"
	AccessFull  = "full"
)

// parseAccessLevel reads the stable MCP_ACCESS_LEVEL contract.
// MCP_READ_ONLY was removed before v1.0.0; keeping one source of truth avoids
// contradictory policies when both variables are present.
func (env source) parseAccessLevel() (string, error) {
	if strings.TrimSpace(env.get("MCP_READ_ONLY")) != "" {
		return "", fmt.Errorf("MCP_READ_ONLY was removed; use MCP_ACCESS_LEVEL=read, write, or full")
	}
	raw := strings.ToLower(strings.TrimSpace(env.get("MCP_ACCESS_LEVEL")))
	switch raw {
	case "":
		return AccessWrite, nil
	case AccessRead, AccessWrite, AccessFull:
		return raw, nil
	default:
		return "", fmt.Errorf("MCP_ACCESS_LEVEL must be read, write, or full")
	}
}

func (env source) parseIDList(name string) ([]string, error) {
	raw := strings.TrimSpace(env.get(name))
	if raw == "" {
		return nil, nil
	}
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			return nil, fmt.Errorf("%s must be a comma-separated list of numeric IDs", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (env source) valueOrDefault(name, fallback string) string {
	value := strings.TrimSpace(env.get(name))
	if value == "" {
		return fallback
	}
	return value
}

func (env source) parseDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := env.valueOrDefault(name, fallback.String())
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return parsed, nil
}

func (env source) parseInt64(name string, fallback int64) (int64, error) {
	value := env.valueOrDefault(name, strconv.FormatInt(fallback, 10))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func (env source) parseHTTPPort() (int, error) {
	value := env.valueOrDefault("MCP_HTTP_PORT", "8080")
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("MCP_HTTP_PORT must be between 1 and 65535")
	}
	return parsed, nil
}

func (env source) parseAuthMode(transport string) (string, error) {
	fallback := "env"
	if transport == "streamable-http" {
		fallback = "request"
	}
	mode := strings.ToLower(strings.TrimSpace(env.valueOrDefault("MCP_AUTH_MODE", fallback)))
	if mode != "env" && mode != "request" {
		return "", fmt.Errorf("MCP_AUTH_MODE must be env or request")
	}
	return mode, nil
}

func (env source) parseBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(env.get(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func (env source) parseTokenPrefixes() []string {
	raw := strings.TrimSpace(env.lookup("MCP_ALLOWED_TOKEN_PREFIXES"))
	if raw == "" {
		return nil
	}
	var prefixes []string
	for _, part := range strings.Split(raw, ",") {
		if prefix := strings.TrimSpace(part); prefix != "" {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}
