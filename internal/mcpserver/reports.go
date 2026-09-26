package mcpserver

import (
	"context"
	"fmt"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ReportInput selects a board and optional column overrides.
type ReportInput struct {
	BoardID        string `json:"board_id" jsonschema:"monday board identifier"`
	StatusColumnID string `json:"status_column_id,omitempty" jsonschema:"override the auto-detected status column"`
	DateColumnID   string `json:"date_column_id,omitempty" jsonschema:"override the auto-detected due-date column"`
	PeopleColumnID string `json:"people_column_id,omitempty" jsonschema:"override the auto-detected owner column"`
	MaxItems       int    `json:"max_items,omitempty" jsonschema:"items to load (bounded by MCP_REPORT_MAX_ITEMS)"`
}

func (in ReportInput) columns() domain.ReportColumns {
	return domain.ReportColumns{StatusColumnID: in.StatusColumnID, DateColumnID: in.DateColumnID, PeopleColumnID: in.PeopleColumnID}
}

// SummaryOutput wraps a board summary.
type SummaryOutput struct {
	Summary domain.BoardSummary `json:"summary"`
}

// DistributionInput selects a column to count.
type DistributionInput struct {
	BoardID  string `json:"board_id" jsonschema:"monday board identifier"`
	ColumnID string `json:"column_id" jsonschema:"column whose display values are counted"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"items to load (bounded by MCP_REPORT_MAX_ITEMS)"`
}

// DistributionOutput counts values.
type DistributionOutput struct {
	ColumnID  string         `json:"column_id"`
	Total     int            `json:"total"`
	Truncated bool           `json:"truncated"`
	Values    []domain.Count `json:"values"`
}

// WorkloadOutput lists per-person load.
type WorkloadOutput struct {
	Columns   domain.ReportColumns   `json:"columns"`
	Truncated bool                   `json:"truncated"`
	People    []domain.WorkloadEntry `json:"people"`
}

// ItemListOutput lists report rows.
type ItemListOutput struct {
	Columns   domain.ReportColumns `json:"columns"`
	Truncated bool                 `json:"truncated"`
	Items     []domain.ItemBrief   `json:"items"`
	Count     int                  `json:"count"`
}

// StaleInput adds a threshold.
type StaleInput struct {
	ReportInput
	Days int `json:"days,omitempty" jsonschema:"days without updates (default 14, max 365)"`
}

// StandupInput adds a time window.
type StandupInput struct {
	ReportInput
	Hours int `json:"hours,omitempty" jsonschema:"look-back window in hours (default 24, max 336)"`
}

// StandupOutput wraps a standup digest.
type StandupOutput struct {
	Standup domain.Standup `json:"standup"`
}

// HealthInput adds a stale threshold.
type HealthInput struct {
	ReportInput
	StaleDays int `json:"stale_days,omitempty" jsonschema:"stale threshold in days (default 14)"`
}

// HealthOutput wraps a health report.
type HealthOutput struct {
	Report domain.HealthReport `json:"report"`
}

// ExportInput selects export columns.
type ExportInput struct {
	BoardID   string   `json:"board_id" jsonschema:"monday board identifier"`
	ColumnIDs []string `json:"column_ids,omitempty" jsonschema:"columns to include (default: all writable/visible)"`
	MaxItems  int      `json:"max_items,omitempty" jsonschema:"items to load (bounded by MCP_REPORT_MAX_ITEMS)"`
}

// ExportOutput returns rendered content.
type ExportOutput struct {
	Format    string `json:"format"`
	Items     int    `json:"items"`
	Truncated bool   `json:"truncated"`
	Content   string `json:"content"`
}

// OverviewOutput wraps a workspace overview.
type OverviewOutput struct {
	Overview application.WorkspaceOverview `json:"overview"`
}

func bounded(value, fallback, max int) (int, error) {
	if value == 0 {
		return fallback, nil
	}
	if value < 1 || value > max {
		return 0, fmt.Errorf("value must be between 1 and %d", max)
	}
	return value, nil
}

func registerReportTools(r *registry) {
	svc := r.svc
	snapshot := func(ctx context.Context, boardID string, maxItems int) (domain.BoardSnapshot, error) {
		return svc.Snapshot(ctx, boardID, maxItems)
	}
	add(r, ToolSpec{Name: "board_summary", Category: CatReports, Title: "Board summary", ReadOnly: true,
		Description: "Summarize a board: items per group and status, completion %, overdue, blocked, and unassigned open work."},
		func(ctx context.Context, in ReportInput) (SummaryOutput, error) {
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return SummaryOutput{}, wrap("board summary", err)
			}
			return SummaryOutput{Summary: domain.Summarize(snap, in.columns(), svc.Now())}, nil
		})
	add(r, ToolSpec{Name: "column_distribution", Category: CatReports, Title: "Column distribution", ReadOnly: true,
		Description: "Count items by the display value of any column (priority, environment, owner, ...)."},
		func(ctx context.Context, in DistributionInput) (DistributionOutput, error) {
			if in.ColumnID == "" {
				return DistributionOutput{}, fmt.Errorf("column distribution: column_id is required")
			}
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return DistributionOutput{}, wrap("column distribution", err)
			}
			return DistributionOutput{ColumnID: in.ColumnID, Total: len(snap.Items), Truncated: snap.Truncated, Values: domain.ColumnDistribution(snap, in.ColumnID)}, nil
		})
	add(r, ToolSpec{Name: "workload_report", Category: CatReports, Title: "Workload report", ReadOnly: true,
		Description: "Open, done, overdue, and blocked items per assignee of the owner column."},
		func(ctx context.Context, in ReportInput) (WorkloadOutput, error) {
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return WorkloadOutput{}, wrap("workload report", err)
			}
			cols, people := domain.Workload(snap, in.columns(), svc.Now())
			return WorkloadOutput{Columns: cols, Truncated: snap.Truncated, People: people}, nil
		})
	add(r, ToolSpec{Name: "overdue_items", Category: CatReports, Title: "Overdue items", ReadOnly: true,
		Description: "List open items whose due date has passed, most late first, with days late."},
		func(ctx context.Context, in ReportInput) (ItemListOutput, error) {
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return ItemListOutput{}, wrap("overdue items", err)
			}
			cols, items := domain.Overdue(snap, in.columns(), svc.Now())
			return ItemListOutput{Columns: cols, Truncated: snap.Truncated, Items: items, Count: len(items)}, nil
		})
	add(r, ToolSpec{Name: "stale_items", Category: CatReports, Title: "Stale items", ReadOnly: true,
		Description: "List open items with no update for N days (default 14)."},
		func(ctx context.Context, in StaleInput) (ItemListOutput, error) {
			days, err := bounded(in.Days, 14, 365)
			if err != nil {
				return ItemListOutput{}, wrap("stale items: days", err)
			}
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return ItemListOutput{}, wrap("stale items", err)
			}
			cols, items := domain.Stale(snap, in.columns(), days, svc.Now())
			return ItemListOutput{Columns: cols, Truncated: snap.Truncated, Items: items, Count: len(items)}, nil
		})
	add(r, ToolSpec{Name: "daily_standup", Category: CatReports, Title: "Daily standup", ReadOnly: true,
		Description: "Standup digest for the last N hours: completed, in motion, blocked, and overdue, plus ready-to-paste Markdown."},
		func(ctx context.Context, in StandupInput) (StandupOutput, error) {
			hours, err := bounded(in.Hours, 24, 336)
			if err != nil {
				return StandupOutput{}, wrap("daily standup: hours", err)
			}
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return StandupOutput{}, wrap("daily standup", err)
			}
			return StandupOutput{Standup: domain.BuildStandup(snap, in.columns(), hours, svc.Now())}, nil
		})
	add(r, ToolSpec{Name: "board_health_report", Category: CatReports, Title: "Board health report", ReadOnly: true,
		Description: "Score a board 0-100 (grade A-D) from overdue, blocked, stale, and unassigned work, with explained signals and Markdown."},
		func(ctx context.Context, in HealthInput) (HealthOutput, error) {
			days, err := bounded(in.StaleDays, 14, 365)
			if err != nil {
				return HealthOutput{}, wrap("board health: stale_days", err)
			}
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return HealthOutput{}, wrap("board health report", err)
			}
			return HealthOutput{Report: domain.BuildHealthReport(snap, in.columns(), days, svc.Now())}, nil
		})
	add(r, ToolSpec{Name: "export_board_markdown", Category: CatReports, Title: "Export board (Markdown)", ReadOnly: true,
		Description: "Export a board as Markdown tables grouped by group."},
		func(ctx context.Context, in ExportInput) (ExportOutput, error) {
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return ExportOutput{}, wrap("export markdown", err)
			}
			return ExportOutput{Format: "markdown", Items: len(snap.Items), Truncated: snap.Truncated, Content: domain.ExportMarkdown(snap, in.ColumnIDs)}, nil
		})
	add(r, ToolSpec{Name: "export_board_csv", Category: CatReports, Title: "Export board (CSV)", ReadOnly: true,
		Description: "Export a board as RFC 4180 CSV (item id, name, group, updated_at, then columns)."},
		func(ctx context.Context, in ExportInput) (ExportOutput, error) {
			snap, err := snapshot(ctx, in.BoardID, in.MaxItems)
			if err != nil {
				return ExportOutput{}, wrap("export csv", err)
			}
			content, err := domain.ExportCSV(snap, in.ColumnIDs)
			if err != nil {
				return ExportOutput{}, wrap("export csv", err)
			}
			return ExportOutput{Format: "csv", Items: len(snap.Items), Truncated: snap.Truncated, Content: content}, nil
		})
	add(r, ToolSpec{Name: "workspace_overview", Category: CatReports, Title: "Workspace overview", ReadOnly: true,
		Description: "Overview of a workspace: active boards (excluding subitem boards), item totals, and URLs."},
		func(ctx context.Context, in WorkspaceIDInput) (OverviewOutput, error) {
			overview, err := svc.BuildWorkspaceOverview(ctx, in.WorkspaceID)
			if err != nil {
				return OverviewOutput{}, wrap("workspace overview", err)
			}
			return OverviewOutput{Overview: *overview}, nil
		})
}

// registerPrompts adds reusable prompt templates for common workflows.
func registerPrompts(server *mcp.Server) {
	boardArg := []*mcp.PromptArgument{{Name: "board_id", Description: "monday board ID", Required: true}}
	server.AddPrompt(&mcp.Prompt{Name: "board_health_review", Title: "Board health review", Description: "Review a board's health and propose concrete actions.", Arguments: boardArg},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			board := req.Params.Arguments["board_id"]
			text := fmt.Sprintf("Run board_health_report and workload_report for board %s. Explain the score, then propose at most five concrete actions "+
				"(owner, item, and change). Before writing anything, validate values with validate_column_values and use bulk tools with dry_run=true first.", board)
			return &mcp.GetPromptResult{Description: "Board health review", Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
		})
	server.AddPrompt(&mcp.Prompt{Name: "standup_digest", Title: "Standup digest", Description: "Produce a team standup from recent board activity.", Arguments: boardArg},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			board := req.Params.Arguments["board_id"]
			text := fmt.Sprintf("Run daily_standup for board %s and list_board_updates (limit 20). Write a concise standup: done, in progress, blockers with owners, and overdue risks.", board)
			return &mcp.GetPromptResult{Description: "Standup digest", Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
		})
}
