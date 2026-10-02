package mcpserver

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

const qualityMaxPages = 50

// QualityPageInput controls bounded aggregation over cursor-paginated items.
type QualityPageInput struct {
	BoardID  string `json:"board_id" jsonschema:"monday board identifier"`
	MaxItems int    `json:"max_items,omitempty" jsonschema:"maximum items to inspect (1-5000, default 500)"`
	MaxPages int    `json:"max_pages,omitempty" jsonschema:"maximum cursor pages to inspect (1-50, default 50)"`
}

// QualityItem is a compact, integration-friendly item projection.
type QualityItem struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url,omitempty"`
	BoardID   string   `json:"board_id,omitempty"`
	Group     string   `json:"group,omitempty"`
	Status    string   `json:"status,omitempty"`
	Priority  string   `json:"priority,omitempty"`
	OwnerIDs  []string `json:"owner_ids,omitempty"`
	DueDate   string   `json:"due_date,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

// QualityItemsOutput reports bounded results and why a result may be partial.
type QualityItemsOutput struct {
	Items          []QualityItem `json:"items"`
	Count          int           `json:"count"`
	Pages          int           `json:"pages"`
	HasMore        bool          `json:"has_more"`
	Warnings       []string      `json:"warnings,omitempty"`
	Truncated      bool          `json:"truncated"`
	GeneratedAtUTC string        `json:"generated_at_utc"`
}

// ItemContextOutput returns a derived item context without changing get_item.
type ItemContextOutput struct {
	Item           QualityItem `json:"item"`
	ColumnCount    int         `json:"column_count"`
	SubitemCount   int         `json:"subitem_count"`
	AgeDays        int         `json:"age_days,omitempty"`
	MissingSignals []string    `json:"missing_signals,omitempty"`
	Warnings       []string    `json:"warnings,omitempty"`
}

// RiskOutput gives actionable board risk counts.
type RiskOutput struct {
	BoardID    string        `json:"board_id"`
	BoardName  string        `json:"board_name"`
	RiskScore  int           `json:"risk_score"`
	Grade      string        `json:"grade"`
	TotalItems int           `json:"total_items"`
	DueSoon    []QualityItem `json:"due_soon"`
	Overdue    []QualityItem `json:"overdue"`
	Blocked    []QualityItem `json:"blocked"`
	Unassigned []QualityItem `json:"unassigned"`
	Warnings   []string      `json:"warnings,omitempty"`
	Truncated  bool          `json:"truncated"`
}

// ActivityOutput summarizes recent updates with authors and replies.
type ActivityOutput struct {
	ItemID         string         `json:"item_id,omitempty"`
	BoardID        string         `json:"board_id,omitempty"`
	UpdateCount    int            `json:"update_count"`
	ReplyCount     int            `json:"reply_count"`
	AuthorCounts   map[string]int `json:"author_counts"`
	LatestUpdateAt string         `json:"latest_update_at,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
}

// IntegrationOutput is a stable payload for Monday↔GitHub/n8n synchronization.
type IntegrationOutput struct {
	ExternalKey string      `json:"external_key"`
	EventType   string      `json:"event_type"`
	Item        QualityItem `json:"item"`
	Correlation string      `json:"correlation_id"`
	Idempotency string      `json:"idempotency_key"`
	Warnings    []string    `json:"warnings,omitempty"`
}

// QualityDiagnosticsOutput validates the effective catalog without exposing secrets.
type QualityDiagnosticsOutput struct {
	ToolCount        int      `json:"tool_count"`
	ReadOnlyCount    int      `json:"read_only_count"`
	WriteCount       int      `json:"write_count"`
	DestructiveCount int      `json:"destructive_count"`
	MissingMetadata  []string `json:"missing_metadata,omitempty"`
	Healthy          bool     `json:"healthy"`
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func qualityBounds(in QualityPageInput, fallback int) (int, int, error) {
	maxItems := in.MaxItems
	if maxItems == 0 {
		maxItems = fallback
	}
	if maxItems < 1 || maxItems > 5000 {
		return 0, 0, fmt.Errorf("max_items must be between 1 and 5000")
	}
	maxPages := in.MaxPages
	if maxPages == 0 {
		maxPages = qualityMaxPages
	}
	if maxPages < 1 || maxPages > qualityMaxPages {
		return 0, 0, fmt.Errorf("max_pages must be between 1 and %d", qualityMaxPages)
	}
	return maxItems, maxPages, nil
}

func qualityProjection(item domain.Item, columns []domain.Column) QualityItem {
	cols := domain.DetectReportColumns(columns, domain.ReportColumns{})
	priority := ""
	for _, column := range columns {
		if strings.Contains(strings.ToLower(column.Title), "priority") || strings.Contains(strings.ToLower(column.Title), "prioridad") {
			priority = item.CellText(column.ID)
			break
		}
	}
	group := ""
	if item.Group != nil {
		group = item.Group.Title
	}
	return QualityItem{ID: item.ID, Name: item.Name, URL: item.URL, BoardID: item.BoardID, Group: group,
		Status: item.CellText(cols.StatusColumnID), Priority: priority, OwnerIDs: item.PersonIDs(cols.PeopleColumnID),
		DueDate: item.DueDate(cols.DateColumnID), UpdatedAt: item.UpdatedAt}
}

// loadItems uses the application's typed service without adding a provider port.
func loadItems(ctx context.Context, svc *application.Service, in QualityPageInput) (domain.BoardSnapshot, int, []string, error) {
	maxItems, maxPages, err := qualityBounds(in, 500)
	if err != nil {
		return domain.BoardSnapshot{}, 0, nil, err
	}
	schema, err := svc.GetBoardSchema(ctx, in.BoardID)
	if err != nil {
		return domain.BoardSnapshot{}, 0, nil, err
	}
	snapshot := domain.BoardSnapshot{Board: schema.Board, Columns: schema.Columns, Groups: schema.Groups}
	warnings := []string{}
	cursor := ""
	pages := 0
	for pages < maxPages && len(snapshot.Items) < maxItems {
		pageSize := 100
		if remaining := maxItems - len(snapshot.Items); remaining < pageSize {
			pageSize = remaining
		}
		page, pageErr := svc.ListItems(ctx, monday.ItemPageQuery{BoardID: in.BoardID, Limit: pageSize, Cursor: cursor})
		if pageErr != nil {
			return domain.BoardSnapshot{}, 0, nil, pageErr
		}
		snapshot.Items = append(snapshot.Items, page.Items...)
		pages++
		cursor = page.Cursor
		if cursor == "" {
			break
		}
	}
	if cursor != "" {
		snapshot.Truncated = true
		if len(snapshot.Items) >= maxItems {
			warnings = append(warnings, fmt.Sprintf("result capped at %d items; increase max_items for a wider view", maxItems))
		}
		if pages >= maxPages {
			warnings = append(warnings, fmt.Sprintf("result capped at %d pages; increase max_pages for a wider view", maxPages))
		}
	}
	return snapshot, pages, warnings, nil
}

func projectItems(snapshot domain.BoardSnapshot) []QualityItem {
	items := make([]QualityItem, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		items = append(items, qualityProjection(item, snapshot.Columns))
	}
	return items
}

func byDue(items []QualityItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].DueDate < items[j].DueDate })
}

func registerQualityTools(r *registry) {
	svc := r.svc
	add(r, ToolSpec{Name: "list_items_all", Category: CatItemsRead, Title: "List all items (bounded)", ReadOnly: true,
		Description: "Load all visible board items up to explicit max_items/max_pages bounds and report truncation warnings."},
		func(ctx context.Context, in QualityPageInput) (QualityItemsOutput, error) {
			snapshot, pages, warnings, err := loadItems(ctx, svc, in)
			if err != nil {
				return QualityItemsOutput{}, wrap("list items all", err)
			}
			return QualityItemsOutput{Items: projectItems(snapshot), Count: len(snapshot.Items), Pages: pages, HasMore: snapshot.Truncated, Truncated: snapshot.Truncated, Warnings: warnings, GeneratedAtUTC: nowUTC()}, nil
		})
	add(r, ToolSpec{Name: "search_items_all", Category: CatItemsRead, Title: "Search all items (bounded)", ReadOnly: true,
		Description: "Search all visible items with a bounded result set and explicit truncation metadata."},
		func(ctx context.Context, in struct {
			BoardID  string `json:"board_id"`
			Text     string `json:"text"`
			MaxItems int    `json:"max_items,omitempty"`
		}) (QualityItemsOutput, error) {
			if strings.TrimSpace(in.Text) == "" {
				return QualityItemsOutput{}, fmt.Errorf("search items all: text is required")
			}
			page, err := svc.SearchItems(ctx, in.BoardID, in.Text, nil, in.MaxItems)
			if err != nil {
				return QualityItemsOutput{}, wrap("search items all", err)
			}
			items := make([]QualityItem, 0, len(page.Items))
			for _, item := range page.Items {
				items = append(items, qualityProjection(item, nil))
			}
			warnings := []string{}
			if page.Cursor != "" {
				warnings = append(warnings, "search result has more items; use search_items with its cursor for the next page")
			}
			return QualityItemsOutput{Items: items, Count: len(items), Pages: 1, HasMore: page.Cursor != "", Truncated: page.Cursor != "", Warnings: warnings, GeneratedAtUTC: nowUTC()}, nil
		})
	add(r, ToolSpec{Name: "get_item_context", Category: CatItemsRead, Title: "Get item context", ReadOnly: true,
		Description: "Return a compact item projection with derived status, owner, priority, due date, age, and missing signals."},
		func(ctx context.Context, in ItemIDInput) (ItemContextOutput, error) {
			item, err := svc.GetItem(ctx, in.ItemID)
			if err != nil {
				return ItemContextOutput{}, wrap("get item context", err)
			}
			projection := qualityProjection(*item, nil)
			missing := []string{}
			if projection.Status == "" {
				missing = append(missing, "status")
			}
			if projection.OwnerIDs == nil {
				missing = append(missing, "owner")
			}
			if projection.DueDate == "" {
				missing = append(missing, "due_date")
			}
			age := 0
			if created, parseErr := time.Parse(time.RFC3339, item.CreatedAt); parseErr == nil {
				age = int(time.Since(created).Hours() / 24)
				if age < 0 {
					age = 0
				}
			}
			return ItemContextOutput{Item: projection, ColumnCount: len(item.ColumnValues), SubitemCount: len(item.Subitems), AgeDays: age, MissingSignals: missing}, nil
		})
	add(r, ToolSpec{Name: "find_due_soon_items", Category: CatReports, Title: "Find due-soon items", ReadOnly: true,
		Description: "Find open items due within a configurable number of days, ordered by due date."},
		func(ctx context.Context, in struct {
			QualityPageInput
			Days int `json:"days,omitempty"`
		}) (QualityItemsOutput, error) {
			days := in.Days
			if days == 0 {
				days = 7
			}
			if days < 1 || days > 90 {
				return QualityItemsOutput{}, fmt.Errorf("days must be between 1 and 90")
			}
			snapshot, pages, warnings, err := loadItems(ctx, svc, in.QualityPageInput)
			if err != nil {
				return QualityItemsOutput{}, wrap("find due soon items", err)
			}
			today := time.Now().UTC()
			out := []QualityItem{}
			for _, item := range projectItems(snapshot) {
				if item.DueDate == "" {
					continue
				}
				due, parseErr := time.Parse("2006-01-02", item.DueDate)
				if parseErr == nil && !due.Before(today.Truncate(24*time.Hour)) && due.Before(today.AddDate(0, 0, days+1)) {
					out = append(out, item)
				}
			}
			byDue(out)
			return QualityItemsOutput{Items: out, Count: len(out), Pages: pages, HasMore: snapshot.Truncated, Truncated: snapshot.Truncated, Warnings: warnings, GeneratedAtUTC: nowUTC()}, nil
		})
	add(r, ToolSpec{Name: "find_unassigned_items", Category: CatReports, Title: "Find unassigned items", ReadOnly: true,
		Description: "Find open items without an owner, making staffing gaps explicit instead of silently treating unreadable owners as empty."},
		func(ctx context.Context, in QualityPageInput) (QualityItemsOutput, error) {
			snapshot, pages, warnings, err := loadItems(ctx, svc, in)
			if err != nil {
				return QualityItemsOutput{}, wrap("find unassigned items", err)
			}
			out := []QualityItem{}
			for _, item := range projectItems(snapshot) {
				if len(item.OwnerIDs) == 0 {
					out = append(out, item)
				}
			}
			return QualityItemsOutput{Items: out, Count: len(out), Pages: pages, HasMore: snapshot.Truncated, Truncated: snapshot.Truncated, Warnings: warnings, GeneratedAtUTC: nowUTC()}, nil
		})
	add(r, ToolSpec{Name: "find_blocked_items", Category: CatReports, Title: "Find blocked items", ReadOnly: true,
		Description: "Find items whose status indicates blocked, stuck, or stopped work."},
		func(ctx context.Context, in QualityPageInput) (QualityItemsOutput, error) {
			snapshot, pages, warnings, err := loadItems(ctx, svc, in)
			if err != nil {
				return QualityItemsOutput{}, wrap("find blocked items", err)
			}
			out := []QualityItem{}
			for _, item := range projectItems(snapshot) {
				status := strings.ToLower(item.Status)
				if strings.Contains(status, "block") || strings.Contains(status, "stuck") || strings.Contains(status, "bloque") || strings.Contains(status, "detenido") {
					out = append(out, item)
				}
			}
			return QualityItemsOutput{Items: out, Count: len(out), Pages: pages, HasMore: snapshot.Truncated, Truncated: snapshot.Truncated, Warnings: warnings, GeneratedAtUTC: nowUTC()}, nil
		})
	add(r, ToolSpec{Name: "board_risk_report", Category: CatReports, Title: "Board risk report", ReadOnly: true,
		Description: "Return a compact, actionable risk view with due-soon, overdue, blocked, and unassigned items plus a 0-100 score."},
		func(ctx context.Context, in QualityPageInput) (RiskOutput, error) {
			snapshot, _, warnings, err := loadItems(ctx, svc, in)
			if err != nil {
				return RiskOutput{}, wrap("board risk report", err)
			}
			items := projectItems(snapshot)
			risk := RiskOutput{BoardID: snapshot.Board.ID, BoardName: snapshot.Board.Name, TotalItems: len(items), Warnings: warnings, Truncated: snapshot.Truncated}
			today := time.Now().UTC()
			for _, item := range items {
				if len(item.OwnerIDs) == 0 {
					risk.Unassigned = append(risk.Unassigned, item)
				}
				status := strings.ToLower(item.Status)
				if strings.Contains(status, "block") || strings.Contains(status, "stuck") || strings.Contains(status, "bloque") {
					risk.Blocked = append(risk.Blocked, item)
				}
				if item.DueDate != "" {
					if due, e := time.Parse("2006-01-02", item.DueDate); e == nil {
						if due.Before(today.Truncate(24 * time.Hour)) {
							risk.Overdue = append(risk.Overdue, item)
						} else if due.Before(today.AddDate(0, 0, 8)) {
							risk.DueSoon = append(risk.DueSoon, item)
						}
					}
				}
			}
			pressure := len(risk.Overdue)*4 + len(risk.Blocked)*3 + len(risk.Unassigned)*2 + len(risk.DueSoon)
			risk.RiskScore = pressure
			if risk.RiskScore > 100 {
				risk.RiskScore = 100
			}
			switch {
			case risk.RiskScore < 20:
				risk.Grade = "A"
			case risk.RiskScore < 40:
				risk.Grade = "B"
			case risk.RiskScore < 70:
				risk.Grade = "C"
			default:
				risk.Grade = "D"
			}
			return risk, nil
		})
	add(r, ToolSpec{Name: "item_activity_summary", Category: CatCollab, Title: "Item activity summary", ReadOnly: true,
		Description: "Summarize an item's update activity by author and reply count, including the latest update timestamp."},
		func(ctx context.Context, in struct {
			ItemID string `json:"item_id"`
			Limit  int    `json:"limit,omitempty"`
		}) (ActivityOutput, error) {
			updates, err := svc.ListItemUpdates(ctx, in.ItemID, in.Limit)
			if err != nil {
				return ActivityOutput{}, wrap("item activity summary", err)
			}
			out := ActivityOutput{ItemID: in.ItemID, UpdateCount: len(updates), AuthorCounts: map[string]int{}}
			for _, update := range updates {
				name := "unknown"
				if update.Creator != nil {
					name = update.Creator.Name
				}
				out.AuthorCounts[name]++
				out.ReplyCount += len(update.Replies)
				if update.CreatedAt > out.LatestUpdateAt {
					out.LatestUpdateAt = update.CreatedAt
				}
			}
			return out, nil
		})
	add(r, ToolSpec{Name: "board_activity_summary", Category: CatCollab, Title: "Board activity summary", ReadOnly: true,
		Description: "Summarize recent board updates by author and reply volume for standups and audits."},
		func(ctx context.Context, in struct {
			BoardID string `json:"board_id"`
			Limit   int    `json:"limit,omitempty"`
		}) (ActivityOutput, error) {
			updates, err := svc.ListBoardUpdates(ctx, in.BoardID, in.Limit)
			if err != nil {
				return ActivityOutput{}, wrap("board activity summary", err)
			}
			out := ActivityOutput{BoardID: in.BoardID, UpdateCount: len(updates), AuthorCounts: map[string]int{}}
			for _, update := range updates {
				name := "unknown"
				if update.Creator != nil {
					name = update.Creator.Name
				}
				out.AuthorCounts[name]++
				out.ReplyCount += len(update.Replies)
				if update.CreatedAt > out.LatestUpdateAt {
					out.LatestUpdateAt = update.CreatedAt
				}
			}
			return out, nil
		})
	add(r, ToolSpec{Name: "item_integration_payload", Category: CatItemsRead, Title: "Build item integration payload", ReadOnly: true,
		Description: "Build a stable idempotent payload for synchronizing a Monday item with GitHub, n8n, or another system."},
		func(ctx context.Context, in struct {
			ItemID        string `json:"item_id"`
			EventType     string `json:"event_type,omitempty"`
			CorrelationID string `json:"correlation_id,omitempty"`
		}) (IntegrationOutput, error) {
			item, err := svc.GetItem(ctx, in.ItemID)
			if err != nil {
				return IntegrationOutput{}, wrap("item integration payload", err)
			}
			event := in.EventType
			if event == "" {
				event = "item.snapshot"
			}
			correlation := in.CorrelationID
			if correlation == "" {
				correlation = fmt.Sprintf("monday-item-%s", item.ID)
			}
			return IntegrationOutput{ExternalKey: fmt.Sprintf("monday:%s:%s", item.BoardID, item.ID), EventType: event, Item: qualityProjection(*item, nil), Correlation: correlation, Idempotency: fmt.Sprintf("%s:%s", event, item.ID)}, nil
		})
	add(r, ToolSpec{Name: "quality_diagnostics", Category: CatDiagnostics, Title: "Response quality diagnostics", ReadOnly: true,
		Description: "Inspect the effective catalog for missing metadata and report read/write/destructive counts without exposing secrets."},
		func(_ context.Context, _ struct{}) (QualityDiagnosticsOutput, error) {
			out := QualityDiagnosticsOutput{}
			for _, spec := range r.Catalog() {
				out.ToolCount++
				if spec.ReadOnly {
					out.ReadOnlyCount++
				} else {
					out.WriteCount++
				}
				if spec.Destructive {
					out.DestructiveCount++
				}
				if strings.TrimSpace(spec.Description) == "" || strings.TrimSpace(spec.Capability) == "" {
					out.MissingMetadata = append(out.MissingMetadata, spec.Name)
				}
			}
			out.Healthy = len(out.MissingMetadata) == 0
			return out, nil
		})
}
