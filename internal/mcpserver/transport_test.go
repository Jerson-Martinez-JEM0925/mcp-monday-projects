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
