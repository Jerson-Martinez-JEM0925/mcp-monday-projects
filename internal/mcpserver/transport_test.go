package mcpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcpserver "github.com/jersonmartinez/mcp-monday-projects/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPHandlerHealthEndpoints(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	handler := mcpserver.NewHTTPHandler(server, "/mcp")
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	for _, path := range []string{"/healthz", "/readyz"} {
		response, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		var body map[string]string
		decodeErr := json.NewDecoder(response.Body).Decode(&body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || decodeErr != nil || body["status"] != "ok" {
			t.Fatalf("GET %s: status=%d body=%v decode=%v", path, response.StatusCode, body, decodeErr)
		}
	}
}

func TestHTTPHandlerInitializeAndListTools(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "ping", Description: "test tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		})
	ts := httptest.NewServer(mcpserver.NewHTTPHandler(server, "/mcp"))
	t.Cleanup(ts.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "ping" {
		t.Fatalf("tools = %+v, want ping", result.Tools)
	}
}

func TestHTTPHandlerRejectsMissingBearer(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	handler := mcpserver.NewHTTPHandler(server, "/mcp", mcpserver.HTTPAuthOptions{
		Mode:                 "request",
		AllowedTokenPrefixes: []string{"monday_"},
	})
	for _, authorization := range []string{"", "Basic value", "Bearer"} {
		request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		request.Header.Set("Authorization", authorization)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("authorization=%q status=%d headers=%v", authorization, recorder.Code, recorder.Header())
		}
		if body := recorder.Body.String(); body != "{\"error\":\"unauthorized\"}\n" {
			t.Fatalf("authorization=%q body=%q", authorization, body)
		}
	}
}

func TestHTTPHandlerRejectsWrongKeyAndPrefix(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	handler := mcpserver.NewHTTPHandler(server, "/mcp", mcpserver.HTTPAuthOptions{
		Mode:                 "request",
		AllowedTokenPrefixes: []string{"monday_"},
		ClientKey:            "client-secret",
	})
	for name, key := range map[string]string{"wrong key": "wrong", "right key": "client-secret"} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			request.Header.Set("Authorization", "Bearer ghp_classic")
			request.Header.Set("X-MCP-Client-Key", key)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d, want 401", recorder.Code)
			}
		})
	}
}

func TestHTTPHandlerRequestBearerInitializes(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "ping", Description: "test tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		})
	ts := httptest.NewServer(mcpserver.NewHTTPHandler(server, "/mcp", mcpserver.HTTPAuthOptions{
		Mode: "request", AllowedTokenPrefixes: []string{"monday_"},
	}))
	t.Cleanup(ts.Close)

	httpClient := &http.Client{}
	transport := &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: httpClient}
	transportHeaderClient := &headerRoundTripper{base: httpClient.Transport, token: "monday_user"}
	transport.HTTPClient = &http.Client{Transport: transportHeaderClient}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.ListTools(context.Background(), nil)
	if err != nil || len(result.Tools) != 1 {
		t.Fatalf("ListTools() result=%+v error=%v", result, err)
	}
}

type headerRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (r *headerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+r.token)
	base := r.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}
