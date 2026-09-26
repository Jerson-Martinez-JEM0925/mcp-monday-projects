package mcpserver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/application/applicationtest"
	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
	"github.com/jersonmartinez/mcp-monday-projects/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const expectedToolCount = 72

func connect(t *testing.T, readOnly bool) (*mcp.ClientSession, []mcpserver.ToolSpec, *applicationtest.FakePort) {
	t.Helper()
	port := applicationtest.NewFakePort()
	guard := application.NewWriteGuard(readOnly, nil, nil)
	svc := application.NewService(port, application.Options{Guard: guard, Now: func() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) }})
	server, catalog := mcpserver.NewWithService(svc, mcpserver.Options{APIVersion: "2026-07", ReadOnly: readOnly, ReportMaxItems: 500})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, catalog, port
}

func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error %v", name, err)
	}
	var payload map[string]any
	if result.StructuredContent != nil {
		raw, _ := json.Marshal(result.StructuredContent)
		_ = json.Unmarshal(raw, &payload)
	}
	return result, payload
}

func errorText(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

func TestServerRegistersFullCatalogWithAnnotations(t *testing.T) {
	session, catalog, _ := connect(t, false)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != expectedToolCount || len(catalog) != expectedToolCount {
		t.Fatalf("tools = %d catalog = %d, want %d", len(tools.Tools), len(catalog), expectedToolCount)
	}
	specs := map[string]mcpserver.ToolSpec{}
	for _, spec := range catalog {
		specs[spec.Name] = spec
	}
	for _, tool := range tools.Tools {
		spec := specs[tool.Name]
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != spec.ReadOnly {
			t.Errorf("%s: readOnlyHint mismatch", tool.Name)
		}
		if !spec.ReadOnly && (tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint != spec.Destructive) {
			t.Errorf("%s: destructiveHint mismatch", tool.Name)
		}
		if tool.InputSchema == nil || tool.OutputSchema == nil || tool.Description == "" {
			t.Errorf("%s: missing schema or description", tool.Name)
		}
		if strings.Contains(strings.ToLower(tool.Name), "delete") {
			t.Errorf("%s: delete tools are not allowed; use archive semantics", tool.Name)
		}
	}
	for _, name := range []string{"archive_item", "archive_board", "archive_group", "bulk_archive_items"} {
		if !specs[name].Destructive {
			t.Errorf("%s must be flagged destructive", name)
		}
	}
}

func TestReadOnlyModeHidesWriteTools(t *testing.T) {
	session, catalog, _ := connect(t, true)
	for _, spec := range catalog {
		if !spec.ReadOnly {
			t.Fatalf("write tool %s registered in read-only mode", spec.Name)
		}
	}
	_, info := call(t, session, "server_info", nil)
	if info["hidden_write_tools"].(float64) != float64(expectedToolCount-len(catalog)) {
		t.Fatalf("server_info = %v", info)
	}
	if policy := info["write_policy"].(map[string]any); policy["read_only"] != true {
		t.Fatalf("policy = %v", policy)
	}
}

func TestToolCallsReachServiceAndReturnStructuredContent(t *testing.T) {
	session, _, port := connect(t, false)
	_, me := call(t, session, "get_me", nil)
	if me["me"].(map[string]any)["name"] != "Tester" {
		t.Fatalf("get_me = %v", me)
	}
	_, created := call(t, session, "create_item", map[string]any{"board_id": "100", "name": "From MCP", "column_values": map[string]any{"status": "Done", "est": 3}})
	if created["item"].(map[string]any)["name"] != "From MCP" || port.MutationCount() != 1 {
		t.Fatalf("create_item = %v", created)
	}
	_, plan := call(t, session, "validate_column_values", map[string]any{"board_id": "100", "column_values": map[string]any{"status": "Nope"}})
	if plan["plan"].(map[string]any)["valid"] != false {
		t.Fatalf("plan = %v", plan)
	}
	_, bulk := call(t, session, "bulk_update_items", map[string]any{"board_id": "100", "updates": []any{map[string]any{"item_id": "500", "column_values": map[string]any{"est": 1}}}})
	if bulk["result"].(map[string]any)["dry_run"] != true || port.MutationCount() != 1 {
		t.Fatalf("bulk default must be dry-run: %v", bulk)
	}
	_, summary := call(t, session, "board_summary", map[string]any{"board_id": "100"})
	if summary["summary"].(map[string]any)["total_items"].(float64) != 2 {
		t.Fatalf("summary = %v", summary)
	}
	_, catalog := call(t, session, "list_tool_catalog", map[string]any{"category": "reports"})
	if catalog["count"].(float64) != 10 {
		t.Fatalf("reports catalog = %v", catalog["count"])
	}
}

func TestToolErrorsAreReportedAsToolResults(t *testing.T) {
	session, _, port := connect(t, false)
	result, _ := call(t, session, "create_item", map[string]any{"board_id": "100", "name": "x", "column_values": map[string]any{"status": "Nope"}})
	if !result.IsError || !strings.Contains(errorText(result), "valid labels") {
		t.Fatalf("result = %+v", result)
	}
	result, _ = call(t, session, "archive_board", map[string]any{"board_id": "100"})
	if !result.IsError || !strings.Contains(errorText(result), "confirm=true") {
		t.Fatalf("archive without confirm = %s", errorText(result))
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestPromptsAreRegistered(t *testing.T) {
	session, _, _ := connect(t, true)
	prompts, err := session.ListPrompts(context.Background(), nil)
	if err != nil || len(prompts.Prompts) != 2 {
		t.Fatalf("prompts = %+v err = %v", prompts, err)
	}
	got, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "board_health_review", Arguments: map[string]string{"board_id": "100"}})
	if err != nil || !strings.Contains(got.Messages[0].Content.(*mcp.TextContent).Text, "board 100") {
		t.Fatalf("prompt = %+v err = %v", got, err)
	}
}

// TestEveryToolIsDocumented keeps docs/TOOLS.md in sync with the registry.
func TestEveryToolIsDocumented(t *testing.T) {
	_, catalog, _ := connect(t, false)
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "TOOLS.md"))
	if err != nil {
		t.Fatalf("docs/TOOLS.md is required: %v", err)
	}
	for _, spec := range catalog {
		if !strings.Contains(string(doc), "`"+spec.Name+"`") {
			t.Errorf("tool %s is not documented in docs/TOOLS.md", spec.Name)
		}
	}
}

func TestNewFromConfigBuildsServer(t *testing.T) {
	server := mcpserver.New(config.Config{APIToken: "t", APIURL: "https://example.invalid", APIVersion: "2026-07", HTTPTimeout: time.Second, MaxResponseBytes: 4096, ReadOnly: true})
	if server == nil {
		t.Fatal("New() returned nil")
	}
}
