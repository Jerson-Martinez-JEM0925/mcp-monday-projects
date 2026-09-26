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
	APIToken         string
	APIVersion       string
	APIURL           string
	LogLevel         string
	HTTPTimeout      time.Duration
	MaxResponseBytes int64
	MaxRetries       int
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
	token, err := env.token()
	if err != nil {
		return Config{}, err
	}

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

	return Config{
		APIToken:                token,
		APIVersion:              env.valueOrDefault("MONDAY_API_VERSION", defaultAPIVersion),
		APIURL:                  apiURL,
		LogLevel:                env.valueOrDefault("MCP_LOG_LEVEL", "info"),
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

// parseAccessLevel reads MCP_ACCESS_LEVEL, keeping MCP_READ_ONLY=true as a
// backward-compatible alias for read. Contradictory settings are rejected.
func (env source) parseAccessLevel() (string, error) {
	readOnly, err := strconv.ParseBool(env.valueOrDefault("MCP_READ_ONLY", "false"))
	if err != nil {
		return "", fmt.Errorf("MCP_READ_ONLY must be true or false")
	}
	raw := strings.ToLower(strings.TrimSpace(env.get("MCP_ACCESS_LEVEL")))
	switch raw {
	case "":
		if readOnly {
			return AccessRead, nil
		}
		return AccessWrite, nil
	case AccessRead, AccessWrite, AccessFull:
		if readOnly && raw != AccessRead {
			return "", fmt.Errorf("MCP_READ_ONLY=true contradicts MCP_ACCESS_LEVEL=%s; remove MCP_READ_ONLY (deprecated) or set MCP_ACCESS_LEVEL=read", raw)
		}
		return raw, nil
	}
	return "", fmt.Errorf("MCP_ACCESS_LEVEL must be read, write, or full")
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
