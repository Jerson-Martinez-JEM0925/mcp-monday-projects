package mcpserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPAuthOptions configures authentication for the MCP HTTP endpoint.
type HTTPAuthOptions struct {
	Mode                 string
	AllowSharedToken     bool
	ClientKey            string
	AllowedTokenPrefixes []string
}

// NewHTTPHandler returns a stateless Streamable HTTP handler with liveness and
// readiness endpoints. Configuration is loaded before this handler is built,
// so a registered handler is ready to serve requests.
func NewHTTPHandler(server *mcp.Server, path string, authOptions ...HTTPAuthOptions) http.Handler {
	path = normalizeHTTPPath(path)
	stream := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})

	var auth HTTPAuthOptions
	if len(authOptions) > 0 {
		auth = authOptions[0]
	}
	mcpHandler := http.Handler(stream)
	if auth.Mode == "request" || auth.ClientKey != "" {
		mcpHandler = authMiddleware(mcpHandler, auth)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", healthHandler)
	mux.Handle(path, mcpHandler)
	return mux
}

func normalizeHTTPPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func authMiddleware(next http.Handler, options HTTPAuthOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if options.Mode == "env" && !options.AllowSharedToken {
			writeUnauthorized(w)
			return
		}
		if !validClientKey(r.Header.Get("X-MCP-Client-Key"), options.ClientKey) {
			writeUnauthorized(w)
			return
		}
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if options.Mode == "request" {
			if !ok || !allowedPrefix(token, options.AllowedTokenPrefixes) {
				writeUnauthorized(w)
				return
			}
			r = r.WithContext(monday.WithRequestToken(r.Context(), token))
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func allowedPrefix(token string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

func validClientKey(actual, expected string) bool {
	if expected == "" {
		return true
	}
	actualHash := sha256.Sum256([]byte(actual))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) == 1
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
