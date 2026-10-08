package mcpserver

import (
	"context"
	"fmt"
	"sort"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolSpec is the catalog metadata of one MCP tool.
type ToolSpec struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
	Destructive bool   `json:"destructive"`
	// Permanent marks irreversible deletes, registered only at access level full.
	Permanent  bool   `json:"permanent,omitempty"`
	Capability string `json:"capability"`
}

// Tool categories, in catalog order.
const (
	CatDiagnostics = "diagnostics"
	CatWorkspaces  = "workspaces"
	CatBoards      = "boards"
	CatGroups      = "groups"
	CatColumns     = "columns"
	CatItemsRead   = "items.read"
	CatItemsWrite  = "items.write"
	CatBulk        = "bulk"
	CatPeople      = "people"
	CatCollab      = "collaboration"
	CatTags        = "tags"
	CatReports     = "reports"
	CatDeletes     = "deletes"
)

// CategoryOrder is the documented order of categories.
var CategoryOrder = []string{CatDiagnostics, CatWorkspaces, CatBoards, CatGroups, CatColumns, CatItemsRead, CatItemsWrite, CatBulk, CatPeople, CatCollab, CatTags, CatReports, CatDeletes}

type registry struct {
	server             *mcp.Server
	svc                *application.Service
	readOnly           bool
	full               bool
	specs              []ToolSpec
	skipped            []ToolSpec
	known              map[string]ToolSpec
	writeToolAllowlist map[string]bool
}

func boolPtr(value bool) *bool { return &value }

// add registers a typed tool. Mutating tools are skipped at access level read,
// when absent from a non-empty write allowlist, and permanent deletes below
// access level full, so clients never see them.
func add[In, Out any](r *registry, spec ToolSpec, fn func(context.Context, In) (Out, error)) {
	if spec.Capability == "" {
		spec.Capability = defaultCapability(spec)
	}
	if r.known == nil {
		r.known = map[string]ToolSpec{}
	}
	r.known[spec.Name] = spec
	if (!spec.ReadOnly && r.readOnly) ||
		(!spec.ReadOnly && len(r.writeToolAllowlist) > 0 && !r.writeToolAllowlist[spec.Name]) ||
		(spec.Permanent && !r.full) {
		r.skipped = append(r.skipped, spec)
		return
	}
	r.specs = append(r.specs, spec)
	annotations := &mcp.ToolAnnotations{
		Title:          spec.Title,
		ReadOnlyHint:   spec.ReadOnly,
		IdempotentHint: spec.ReadOnly,
		OpenWorldHint:  boolPtr(true),
	}
	if !spec.ReadOnly {
		annotations.DestructiveHint = boolPtr(spec.Destructive)
	}
	mcp.AddTool(r.server, &mcp.Tool{Name: spec.Name, Title: spec.Title, Description: spec.Description, Annotations: annotations},
		func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
			output, err := fn(ctx, input)
			if err != nil {
				var zero Out
				return nil, zero, err
			}
			return nil, output, nil
		})
}

func defaultCapability(spec ToolSpec) string {
	suffix := ".write"
	if spec.ReadOnly {
		suffix = ".read"
	}
	switch spec.Category {
	case CatItemsRead, CatItemsWrite, CatBulk:
		return "items" + suffix
	case CatReports:
		return "reports.read"
	case CatDiagnostics:
		return "account.read"
	}
	return spec.Category + suffix
}

func (r *registry) validateWriteToolAllowlist() error {
	for name := range r.writeToolAllowlist {
		if _, ok := r.known[name]; !ok {
			return fmt.Errorf("MCP_WRITE_TOOL_ALLOWLIST contains unknown tool %q", name)
		}
	}
	return nil
}

// Catalog returns the registered tool specs sorted by category then name.
func (r *registry) Catalog() []ToolSpec {
	return sortSpecs(r.specs)
}

func sortSpecs(specs []ToolSpec) []ToolSpec {
	order := map[string]int{}
	for index, category := range CategoryOrder {
		order[category] = index
	}
	list := append([]ToolSpec(nil), specs...)
	sort.SliceStable(list, func(i, j int) bool {
		if order[list[i].Category] != order[list[j].Category] {
			return order[list[i].Category] < order[list[j].Category]
		}
		return list[i].Name < list[j].Name
	})
	return list
}
