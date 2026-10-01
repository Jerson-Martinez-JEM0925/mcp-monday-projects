package mcpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/application/applicationtest"
	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// expectedToolCount is the write-level catalog; full adds the delete tools.
const expectedToolCount = 72

const expectedDeleteTools = 7

func connect(t *testing.T, readOnly bool) (*mcp.ClientSession, []mcpserver.ToolSpec, *applicationtest.FakePort) {
	t.Helper()
	if readOnly {
		return connectLevel(t, config.AccessRead)
	}
	return connectLevel(t, config.AccessWrite)
}

func connectLevel(t *testing.T, level string) (*mcp.ClientSession, []mcpserver.ToolSpec, *applicationtest.FakePort) {
	t.Helper()
	port := applicationtest.NewFakePort()
	guard := application.NewWriteGuard(level == config.AccessRead, nil, nil)
	svc := application.NewService(port, application.Options{Guard: guard, AllowDelete: level == config.AccessFull, Now: func() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) }})
	server, catalog := mcpserver.NewWithService(svc, mcpserver.Options{APIVersion: "2026-07", AccessLevel: level, ReportMaxItems: 500})
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
			t.Errorf("%s: permanent deletes must not be registered below MCP_ACCESS_LEVEL=full", tool.Name)
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
	if info["hidden_write_tools"].(float64) != float64(expectedToolCount+expectedDeleteTools-len(catalog)) {
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
	_, catalog, _ := connectLevel(t, config.AccessFull)
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

func TestWorkspaceScopeIsReportedAndEnforced(t *testing.T) {
	port := applicationtest.NewFakePort()
	port.Columns["200"] = port.Columns["100"]
	port.BoardWorkspace["200"] = "8"
	svc := application.NewService(port, application.Options{WorkspaceScope: "7"})
	server, _ := mcpserver.NewWithService(svc, mcpserver.Options{APIVersion: "2026-07", ReportMaxItems: 500})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	if !strings.Contains(session.InitializeResult().Instructions, "MONDAY_WORKSPACE_ID") {
		t.Fatalf("instructions do not mention the scope: %q", session.InitializeResult().Instructions)
	}
	_, info := call(t, session, "server_info", nil)
	scope, _ := info["workspace_scope"].(map[string]any)
	if scope["id"] != "7" || scope["name"] != "DevOps" {
		t.Fatalf("workspace_scope = %v", info["workspace_scope"])
	}
	_, overview := call(t, session, "workspace_overview", map[string]any{})
	if overview["overview"] == nil {
		t.Fatalf("workspace_overview without workspace_id = %v", overview)
	}
	result, _ := call(t, session, "create_item", map[string]any{"board_id": "200", "name": "x"})
	if !result.IsError || !strings.Contains(errorText(result), "outside the configured scope") {
		t.Fatalf("foreign board = %s", errorText(result))
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestUnscopedInstructionsAskToChooseWorkspace(t *testing.T) {
	session, _, _ := connect(t, false)
	if !strings.Contains(session.InitializeResult().Instructions, "list_workspaces") {
		t.Fatalf("instructions = %q", session.InitializeResult().Instructions)
	}
}

func TestFullAccessRegistersPermanentDeletes(t *testing.T) {
	session, catalog, port := connectLevel(t, config.AccessFull)
	if len(catalog) != expectedToolCount+expectedDeleteTools {
		t.Fatalf("full catalog = %d, want %d", len(catalog), expectedToolCount+expectedDeleteTools)
	}
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	deletes := 0
	for _, tool := range tools.Tools {
		if !strings.HasPrefix(tool.Name, "delete_") {
			continue
		}
		deletes++
		if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint || tool.Annotations.ReadOnlyHint {
			t.Errorf("%s: must be annotated destructive", tool.Name)
		}
	}
	if deletes != expectedDeleteTools {
		t.Fatalf("delete tools = %d", deletes)
	}
	result, _ := call(t, session, "delete_item", map[string]any{"item_id": "500"})
	if !result.IsError || !strings.Contains(errorText(result), "confirm=true") || !strings.Contains(errorText(result), "archive_item") {
		t.Fatalf("delete without confirm = %s", errorText(result))
	}
	if port.MutationCount() != 0 {
		t.Fatalf("unconfirmed delete mutated: %v", port.Mutations)
	}
	_, info := call(t, session, "server_info", nil)
	if info["access_level"] != "full" {
		t.Fatalf("access_level = %v", info["access_level"])
	}
	if !strings.Contains(session.InitializeResult().Instructions, "PERMANENTLY") {
		t.Fatalf("instructions = %q", session.InitializeResult().Instructions)
	}
}

func TestReadAndWriteLevelsHideDeletes(t *testing.T) {
	for _, level := range []string{config.AccessRead, config.AccessWrite} {
		session, catalog, _ := connectLevel(t, level)
		for _, spec := range catalog {
			if spec.Permanent {
				t.Fatalf("%s registered at level %s", spec.Name, level)
			}
		}
		_, info := call(t, session, "server_info", nil)
		if info["access_level"] != level {
			t.Fatalf("access_level = %v, want %s", info["access_level"], level)
		}
	}
}

func TestServerInfoReportsProfile(t *testing.T) {
	port := applicationtest.NewFakePort()
	svc := application.NewService(port, application.Options{})
	server, _ := mcpserver.NewWithService(svc, mcpserver.Options{APIVersion: "2026-07", Profile: "devops"})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	if _, info := call(t, session, "server_info", nil); info["profile"] != "devops" {
		t.Fatalf("profile = %v", info["profile"])
	}
}

// TestStableToolSchemas freezes the v1.x protocol surface. Intentional schema
// changes must update this digest and the release notes in the same PR.
func TestStableToolSchemas(t *testing.T) {
	session, _, _ := connectLevel(t, config.AccessFull)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	type contract struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Input       any    `json:"input_schema"`
		Output      any    `json:"output_schema"`
	}
	contracts := make([]contract, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		contracts = append(contracts, contract{Name: tool.Name, Description: tool.Description, Input: tool.InputSchema, Output: tool.OutputSchema})
	}
	sort.Slice(contracts, func(i, j int) bool { return contracts[i].Name < contracts[j].Name })
	raw, err := json.Marshal(contracts)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	const expected = "2d814cd612f63421d0290f9740fc2e64523a439286b4e83bc757a81f95288e18"
	if got != expected {
		t.Fatalf("stable tool schema digest changed: got %s; update intentionally with release notes", got)
	}
	t.Logf("stable tool schema digest: %s (%d tools)", got, len(contracts))
}

func TestWritesAndReportsWarnAboutColumnsTheTokenCannotRead(t *testing.T) {
	session, _, port := connect(t, false)

	_, created := call(t, session, "create_item", map[string]any{"board_id": "100", "name": "Visible", "column_values": map[string]any{"status": "Done", "est": 3}})
	if _, ok := created["warnings"]; ok {
		t.Fatalf("visible columns must not warn: %v", created)
	}

	port.HiddenColumns["status"] = true
	_, hidden := call(t, session, "create_item", map[string]any{"board_id": "100", "name": "Hidden", "column_values": map[string]any{"status": "Done", "est": 3}})
	warnings, _ := hidden["warnings"].([]any)
	if len(warnings) != 1 || !strings.Contains(warnings[0].(string), "status") || strings.Contains(warnings[0].(string), "est,") {
		t.Fatalf("create_item warnings = %v", hidden)
	}
	_, updated := call(t, session, "update_item_column_values", map[string]any{"board_id": "100", "item_id": "500", "column_values": map[string]any{"status": "Done"}})
	if w, _ := updated["warnings"].([]any); len(w) != 1 {
		t.Fatalf("update warnings = %v", updated)
	}
	_, status := call(t, session, "set_item_status", map[string]any{"board_id": "100", "item_id": "500", "label": "Done"})
	if w, _ := status["warnings"].([]any); len(w) != 1 || status["column_id"] != "status" {
		t.Fatalf("set_item_status warnings = %v", status)
	}

	// Reports read stored items; hide the column from every one of them.
	for id, item := range port.Items {
		var visible []domain.ColumnValue
		for _, value := range item.ColumnValues {
			if value.ID != "status" {
				visible = append(visible, value)
			}
		}
		item.ColumnValues = visible
		port.Items[id] = item
	}
	_, dist := call(t, session, "column_distribution", map[string]any{"board_id": "100", "column_id": "status"})
	if w, _ := dist["warnings"].([]any); len(w) != 1 {
		t.Fatalf("column_distribution warnings = %v", dist)
	}
}
