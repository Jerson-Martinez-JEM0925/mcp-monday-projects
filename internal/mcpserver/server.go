package mcpserver

import (
	"context"
	"runtime"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the server release version.
const Version = "0.2.0"

// ServerInfoInput is intentionally empty; it makes the diagnostic tool easy to call.
type ServerInfoInput struct{}

// ServerInfoOutput describes the running MCP binary without exposing secrets.
type ServerInfoOutput struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Runtime      string         `json:"runtime"`
	APIVersion   string         `json:"api_version"`
	AccessLevel  string         `json:"access_level"`
	ToolCount    int            `json:"tool_count"`
	HiddenTools  int            `json:"hidden_write_tools"`
	WritePolicy  map[string]any `json:"write_policy"`
	ReportBudget int            `json:"report_max_items"`
	// WorkspaceScope is set when MONDAY_WORKSPACE_ID confines the server.
	WorkspaceScope *WorkspaceScope `json:"workspace_scope,omitempty"`
}

// WorkspaceScope describes the workspace the server is confined to.
type WorkspaceScope struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Error string `json:"error,omitempty"`
}

// ServerInfo returns safe build/runtime metadata. It is kept as a standalone
// handler for backwards compatibility.
func ServerInfo(_ context.Context, _ *mcp.CallToolRequest, _ ServerInfoInput) (*mcp.CallToolResult, ServerInfoOutput, error) {
	return nil, ServerInfoOutput{Name: "mcp-monday-projects", Version: Version, Runtime: runtime.Version(), APIVersion: "configured-at-runtime"}, nil
}

// Options configure a server built around an existing application service.
type Options struct {
	APIVersion string
	// AccessLevel is read, write, or full; empty derives it from ReadOnly.
	AccessLevel    string
	ReadOnly       bool
	ReportMaxItems int
}

// level resolves the effective access level.
func (o Options) level() string {
	switch {
	case o.AccessLevel != "":
		return o.AccessLevel
	case o.ReadOnly:
		return config.AccessRead
	}
	return config.AccessWrite
}

const baseInstructions = "Tools for monday.com workspaces, boards, items, people, updates, and reports. " +
	"Read tools are safe. Write tools validate column values against the board schema before calling monday, " +
	"bulk tools default to dry_run=true, and destructive operations archive instead of delete."

// accessNotes explains the effective access level to the client.
var accessNotes = map[string]string{
	config.AccessRead:  " Access level: read (MCP_ACCESS_LEVEL=read): only read tools are available.",
	config.AccessWrite: " Access level: write (MCP_ACCESS_LEVEL=write): create, update, and archive are available; permanent deletes are not.",
	config.AccessFull: " Access level: full (MCP_ACCESS_LEVEL=full): delete_* tools PERMANENTLY delete and require confirm=true; " +
		"prefer archive_* unless the user explicitly asks for a permanent delete.",
}

// instructions tells the client the access level and how to pick a workspace.
func instructions(level, scope string) string {
	return baseInstructions + accessNotes[level] + workspaceNote(scope)
}

func workspaceNote(scope string) string {
	if scope != "" {
		return " This server is scoped to workspace " + scope +
			" (MONDAY_WORKSPACE_ID): omit workspace_id to use it; every other workspace, and any board or item outside it, is refused. " +
			"Call server_info to see the workspace name."
	}
	return " No workspace scope is configured: resolve the workspace from the user's request, " +
		"or call list_workspaces and ask the user which workspace to act on before any write."
}

// New creates the MCP server from configuration and registers every tool.
func New(cfg config.Config) *mcp.Server {
	client := monday.NewClient(cfg)
	guard := application.NewWriteGuard(cfg.ReadOnly, cfg.WriteBoardAllowlist, cfg.WriteWorkspaceAllowlist)
	svc := application.NewService(client, application.Options{
		Guard: guard, ReportMaxItems: cfg.ReportMaxItems, WorkspaceScope: cfg.WorkspaceID,
		AllowDelete: cfg.AccessLevel == config.AccessFull,
	})
	server, _ := NewWithService(svc, Options{APIVersion: cfg.APIVersion, AccessLevel: cfg.AccessLevel, ReadOnly: cfg.ReadOnly, ReportMaxItems: cfg.ReportMaxItems})
	return server
}

// NewWithService builds the server around a service and returns its catalog.
func NewWithService(svc *application.Service, options Options) (*mcp.Server, []ToolSpec) {
	server := mcp.NewServer(&mcp.Implementation{Name: "mcp-monday-projects", Title: "monday.com MCP", Version: Version}, &mcp.ServerOptions{
		Instructions: instructions(options.level(), svc.WorkspaceScope()),
	})
	level := options.level()
	r := &registry{server: server, svc: svc, readOnly: level == config.AccessRead, full: level == config.AccessFull}

	add(r, ToolSpec{Name: "server_info", Category: CatDiagnostics, Title: "Server info", ReadOnly: true,
		Description: "Return safe server metadata: version, runtime, API version, tool count, the access level (MCP_ACCESS_LEVEL), the effective write policy (allowlists), and the workspace scope (MONDAY_WORKSPACE_ID) with its resolved name."},
		func(ctx context.Context, _ ServerInfoInput) (ServerInfoOutput, error) {
			out := ServerInfoOutput{
				Name: "mcp-monday-projects", Version: Version, Runtime: runtime.Version(), APIVersion: options.APIVersion, AccessLevel: level,
				ToolCount: len(r.specs), HiddenTools: len(r.skipped), WritePolicy: svc.Guard().Describe(), ReportBudget: options.ReportMaxItems,
			}
			if scope := svc.WorkspaceScope(); scope != "" {
				out.WorkspaceScope = &WorkspaceScope{ID: scope}
				if workspace, err := svc.GetWorkspace(ctx, scope); err != nil {
					out.WorkspaceScope.Error = err.Error()
				} else {
					out.WorkspaceScope.Name = workspace.Name
				}
			}
			return out, nil
		})
	add(r, ToolSpec{Name: "list_tool_catalog", Category: CatDiagnostics, Title: "Tool catalog", ReadOnly: true,
		Description: "List every registered tool with its category, read-only/destructive hints, and required capability. Optionally filter by category."},
		func(_ context.Context, in CatalogInput) (CatalogOutput, error) {
			var tools []ToolSpec
			for _, spec := range r.Catalog() {
				if in.Category == "" || spec.Category == in.Category {
					tools = append(tools, spec)
				}
			}
			return CatalogOutput{Tools: tools, Count: len(tools), Categories: CategoryOrder}, nil
		})
	add(r, ToolSpec{Name: "get_me", Category: CatDiagnostics, Title: "Authenticated user", ReadOnly: true,
		Description: "Return the user and account behind the configured token (connectivity check). Never returns the token."},
		func(ctx context.Context, _ struct{}) (MeOutput, error) {
			me, err := svc.Me(ctx)
			if err != nil {
				return MeOutput{}, wrap("get me", err)
			}
			return MeOutput{Me: *me}, nil
		})
	add(r, ToolSpec{Name: "get_api_status", Category: CatDiagnostics, Title: "API budget and version", ReadOnly: true,
		Description: "Return monday's remaining per-minute complexity budget, reset time, and the API version that served the request."},
		func(ctx context.Context, _ struct{}) (APIStatusOutput, error) {
			complexity, version, err := svc.APIStatus(ctx)
			if err != nil {
				return APIStatusOutput{}, wrap("get api status", err)
			}
			used := 0.0
			if complexity.Before > 0 {
				used = float64(complexity.Before-complexity.After) * 100 / float64(complexity.Before)
			}
			return APIStatusOutput{Complexity: complexity, Version: version, BudgetUsedPct: used}, nil
		})

	registerStructureTools(r)
	registerItemTools(r)
	registerCollaborationTools(r)
	registerReportTools(r)
	registerDeleteTools(r)
	registerPrompts(server)
	return server, r.Catalog()
}

// CatalogInput filters the tool catalog.
type CatalogInput struct {
	Category string `json:"category,omitempty" jsonschema:"optional category filter, e.g. items.write or reports"`
}

// CatalogOutput lists tool metadata.
type CatalogOutput struct {
	Tools      []ToolSpec `json:"tools"`
	Count      int        `json:"count"`
	Categories []string   `json:"categories"`
}

// MeOutput wraps the authenticated identity.
type MeOutput struct {
	Me domain.Me `json:"me"`
}

// APIStatusOutput reports the complexity budget.
type APIStatusOutput struct {
	Complexity    domain.Complexity `json:"complexity"`
	Version       domain.APIVersion `json:"version"`
	BudgetUsedPct float64           `json:"budget_used_pct"`
}
