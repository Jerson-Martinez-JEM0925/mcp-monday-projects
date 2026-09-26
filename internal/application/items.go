package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// MaxBulkItems bounds every bulk mutation.
const MaxBulkItems = 50

var ruleOperators = []string{
	"any_of", "not_any_of", "is_empty", "is_not_empty", "within_the_last", "within_the_next",
	"greater_than", "greater_than_or_equals", "lower_than", "lower_than_or_equal", "between",
	"starts_with", "ends_with", "contains_text", "contains_terms", "not_contains_text",
}

func validateFilter(filter *domain.ItemFilter) error {
	if filter == nil {
		return nil
	}
	if len(filter.Rules) > 20 {
		return invalid("filter accepts at most 20 rules")
	}
	if err := oneOf("filter.operator", filter.Operator, "and", "or"); err != nil {
		return err
	}
	for index, rule := range filter.Rules {
		if strings.TrimSpace(rule.ColumnID) == "" {
			return invalid("filter.rules[%d].column_id is required", index)
		}
		if err := oneOf(fmt.Sprintf("filter.rules[%d].operator", index), rule.Operator, ruleOperators...); err != nil {
			return err
		}
	}
	return nil
}

// ListItems returns one cursor page of items, optionally filtered or scoped to a group.
func (s *Service) ListItems(ctx context.Context, query monday.ItemPageQuery) (domain.ItemPage, error) {
	limit, err := BoundLimit(query.Limit, 25, 100)
	if err != nil {
		return domain.ItemPage{}, err
	}
	query.Limit = limit
	if query.Cursor == "" {
		if err := requireID("board_id", query.BoardID); err != nil {
			return domain.ItemPage{}, err
		}
	}
	if err := validateFilter(query.Filter); err != nil {
		return domain.ItemPage{}, err
	}
	return s.port.ListItemsPage(ctx, query)
}

// SearchItems finds items whose name contains text, combined with optional rules.
func (s *Service) SearchItems(ctx context.Context, boardID, text string, filter *domain.ItemFilter, limit int) (domain.ItemPage, error) {
	if err := requireID("board_id", boardID); err != nil {
		return domain.ItemPage{}, err
	}
	combined := &domain.ItemFilter{Operator: "and"}
	if filter != nil {
		combined.Rules = append(combined.Rules, filter.Rules...)
		if filter.Operator != "" {
			combined.Operator = filter.Operator
		}
	}
	if strings.TrimSpace(text) != "" {
		combined.Rules = append(combined.Rules, domain.ItemFilterRule{ColumnID: "name", CompareValue: []any{text}, Operator: "contains_text"})
	}
	if len(combined.Rules) == 0 {
		return domain.ItemPage{}, invalid("provide text and/or filter rules")
	}
	return s.ListItems(ctx, monday.ItemPageQuery{BoardID: boardID, Limit: limit, Filter: combined})
}

// FindItemsByColumnValues finds items whose columns exactly match values.
func (s *Service) FindItemsByColumnValues(ctx context.Context, boardID string, matches []monday.ColumnMatch, limit int, cursor string) (domain.ItemPage, error) {
	if err := requireID("board_id", boardID); err != nil {
		return domain.ItemPage{}, err
	}
	if cursor == "" && len(matches) == 0 {
		return domain.ItemPage{}, invalid("columns must contain at least one {column_id, column_values} match")
	}
	for index, match := range matches {
		if match.ColumnID == "" || len(match.Values) == 0 {
			return domain.ItemPage{}, invalid("columns[%d] needs column_id and at least one value", index)
		}
	}
	limit, err := BoundLimit(limit, 25, 100)
	if err != nil {
		return domain.ItemPage{}, err
	}
	return s.port.ItemsByColumnValues(ctx, boardID, matches, limit, cursor)
}

// GetItems returns up to 100 items (with subitems) by ID.
func (s *Service) GetItems(ctx context.Context, ids []string) ([]domain.Item, error) {
	if err := requireIDs("item_ids", ids, 100); err != nil {
		return nil, err
	}
	return s.port.GetItems(ctx, ids)
}

// GetItem returns one item.
func (s *Service) GetItem(ctx context.Context, id string) (*domain.Item, error) {
	items, err := s.GetItems(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, &monday.NotFoundError{Resource: "item", ID: id}
	}
	return &items[0], nil
}

// ---- validation ------------------------------------------------------------

// PlanValues validates friendly column values against a board schema.
func (s *Service) PlanValues(ctx context.Context, boardID string, values map[string]any) (domain.ColumnValuePlan, error) {
	if err := requireID("board_id", boardID); err != nil {
		return domain.ColumnValuePlan{}, err
	}
	if len(values) == 0 {
		return domain.ColumnValuePlan{}, invalid("column_values must contain at least one column")
	}
	columns, err := s.port.ListColumns(ctx, boardID)
	if err != nil {
		return domain.ColumnValuePlan{}, fmt.Errorf("load board columns: %w", err)
	}
	return domain.PlanColumnValues(columns, values), nil
}

// subitemBoard returns the subitems board referenced by a parent's subtasks column.
func (s *Service) subitemColumns(ctx context.Context, parent domain.Item) ([]domain.Column, error) {
	columns, err := s.port.ListColumns(ctx, parent.BoardID)
	if err != nil {
		return nil, err
	}
	for _, column := range columns {
		if column.Type != "subtasks" {
			continue
		}
		var settings struct {
			BoardIDs []int64 `json:"boardIds"`
		}
		if err := decodeSettings(column.Settings, &settings); err == nil && len(settings.BoardIDs) > 0 {
			return s.port.ListColumns(ctx, fmt.Sprint(settings.BoardIDs[0]))
		}
	}
	return nil, invalid("the parent board has no subitems board yet; create a first subitem without column_values")
}

// ---- writes ----------------------------------------------------------------

// CreateItem validates values and creates an item.
func (s *Service) CreateItem(ctx context.Context, boardID, groupID, name string, values map[string]any) (*domain.Item, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	normalized, err := s.validated(ctx, boardID, values)
	if err != nil {
		return nil, err
	}
	return s.port.CreateItem(ctx, boardID, groupID, name, normalized)
}

func (s *Service) validated(ctx context.Context, boardID string, values map[string]any) (map[string]any, error) {
	if len(values) == 0 {
		return nil, nil
	}
	plan, err := s.PlanValues(ctx, boardID, values)
	if err != nil {
		return nil, err
	}
	if err := plan.Err(); err != nil {
		return nil, err
	}
	return plan.Normalized, nil
}

// CreateSubitem creates a subitem under a parent, validating values against the subitems board.
func (s *Service) CreateSubitem(ctx context.Context, parentID, name string, values map[string]any) (*domain.Item, error) {
	if err := requireID("parent_item_id", parentID); err != nil {
		return nil, err
	}
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	parent, err := s.GetItem(ctx, parentID)
	if err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(parent.BoardID); err != nil {
		return nil, err
	}
	var normalized map[string]any
	if len(values) > 0 {
		columns, err := s.subitemColumns(ctx, *parent)
		if err != nil {
			return nil, err
		}
		plan := domain.PlanColumnValues(columns, values)
		if err := plan.Err(); err != nil {
			return nil, err
		}
		normalized = plan.Normalized
	}
	return s.port.CreateSubitem(ctx, parentID, name, normalized)
}

// UpdateItemValues validates and applies column values in one mutation.
func (s *Service) UpdateItemValues(ctx context.Context, boardID, itemID string, values map[string]any) (*domain.Item, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, invalid("column_values must contain at least one column")
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	normalized, err := s.validated(ctx, boardID, values)
	if err != nil {
		return nil, err
	}
	return s.port.UpdateItemValues(ctx, boardID, itemID, normalized)
}

// RenameItem changes an item's name.
func (s *Service) RenameItem(ctx context.Context, boardID, itemID, name string) (*domain.Item, error) {
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	return s.UpdateItemValues(ctx, boardID, itemID, map[string]any{"name": name})
}

// SetItemColumn is a typed convenience for one column (status, date, people...).
func (s *Service) SetItemColumn(ctx context.Context, boardID, itemID, columnID string, value any) (*domain.Item, error) {
	if err := requireText("column_id", columnID); err != nil {
		return nil, err
	}
	return s.UpdateItemValues(ctx, boardID, itemID, map[string]any{columnID: value})
}

// SetDetectedColumn sets a status, date, or people value, auto-detecting the
// column when columnID is empty. It returns the column that was written.
func (s *Service) SetDetectedColumn(ctx context.Context, boardID, itemID, kind, columnID string, value any) (*domain.Item, string, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, "", err
	}
	if columnID == "" {
		columns, err := s.port.ListColumns(ctx, boardID)
		if err != nil {
			return nil, "", fmt.Errorf("load board columns: %w", err)
		}
		detected := domain.DetectReportColumns(columns, domain.ReportColumns{})
		columnID = map[string]string{"status": detected.StatusColumnID, "date": detected.DateColumnID, "people": detected.PeopleColumnID}[kind]
		if columnID == "" {
			return nil, "", invalid("board has no %s column; pass column_id explicitly", kind)
		}
	}
	item, err := s.SetItemColumn(ctx, boardID, itemID, columnID, value)
	return item, columnID, err
}

// MoveItem moves an item to another group.
func (s *Service) MoveItem(ctx context.Context, itemID, groupID string) (*domain.Item, error) {
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if err := requireText("group_id", groupID); err != nil {
		return nil, err
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.port.MoveItem(ctx, itemID, groupID)
}

// MoveItemToBoard moves an item to another board; both boards must be writable.
func (s *Service) MoveItemToBoard(ctx context.Context, itemID, boardID, groupID string) (*domain.Item, error) {
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("group_id", groupID); err != nil {
		return nil, err
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.MoveItemToBoard(ctx, itemID, boardID, groupID)
}

// DuplicateItem duplicates an item.
func (s *Service) DuplicateItem(ctx context.Context, boardID, itemID string, withUpdates bool) (*domain.Item, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.DuplicateItem(ctx, boardID, itemID, withUpdates)
}

// ArchiveItem archives an item (restorable from monday's archive).
func (s *Service) ArchiveItem(ctx context.Context, itemID string) (*domain.Item, error) {
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.port.ArchiveItem(ctx, itemID)
}

// ---- bulk ------------------------------------------------------------------

// BulkUpdate is one row of a bulk column-value update.
type BulkUpdate struct {
	ItemID       string         `json:"item_id" jsonschema:"monday item identifier"`
	ColumnValues map[string]any `json:"column_values" jsonschema:"column ID to friendly value mapping"`
}

// BulkResult reports the plan and, when executed, the per-item outcome.
type BulkResult struct {
	DryRun    bool             `json:"dry_run"`
	Planned   int              `json:"planned"`
	Succeeded int              `json:"succeeded"`
	Failed    int              `json:"failed"`
	Rows      []BulkRowOutcome `json:"rows"`
}

// BulkRowOutcome is the result for one item.
type BulkRowOutcome struct {
	ItemID     string                   `json:"item_id"`
	Status     string                   `json:"status"`
	Error      string                   `json:"error,omitempty"`
	Normalized map[string]any           `json:"normalized,omitempty"`
	Issues     []domain.ValidationIssue `json:"issues,omitempty"`
}

// BulkUpdateItems validates every row first; nothing is written if any row is
// invalid. With dryRun (the default at the MCP layer) nothing is written at all.
func (s *Service) BulkUpdateItems(ctx context.Context, boardID string, updates []BulkUpdate, dryRun bool) (BulkResult, error) {
	if err := requireID("board_id", boardID); err != nil {
		return BulkResult{}, err
	}
	if len(updates) == 0 || len(updates) > MaxBulkItems {
		return BulkResult{}, invalid("updates must contain between 1 and %d rows", MaxBulkItems)
	}
	if !dryRun {
		if err := s.guard.CheckBoard(boardID); err != nil {
			return BulkResult{}, err
		}
	}
	columns, err := s.port.ListColumns(ctx, boardID)
	if err != nil {
		return BulkResult{}, fmt.Errorf("load board columns: %w", err)
	}
	result := BulkResult{DryRun: dryRun, Planned: len(updates)}
	valid := true
	for _, update := range updates {
		row := BulkRowOutcome{ItemID: update.ItemID, Status: "planned"}
		if err := requireID("item_id", update.ItemID); err != nil {
			row.Status, row.Error, valid = "invalid", err.Error(), false
		} else if len(update.ColumnValues) == 0 {
			row.Status, row.Error, valid = "invalid", "column_values is empty", false
		} else {
			plan := domain.PlanColumnValues(columns, update.ColumnValues)
			row.Normalized, row.Issues = plan.Normalized, plan.Issues
			if !plan.Valid {
				row.Status, valid = "invalid", false
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if dryRun || !valid {
		if !valid {
			result.Failed = countStatus(result.Rows, "invalid")
		}
		return result, nil
	}
	for index := range result.Rows {
		row := &result.Rows[index]
		if _, err := s.port.UpdateItemValues(ctx, boardID, row.ItemID, row.Normalized); err != nil {
			row.Status, row.Error = "failed", err.Error()
			result.Failed++
			continue
		}
		row.Status = "updated"
		result.Succeeded++
	}
	return result, nil
}

func countStatus(rows []BulkRowOutcome, status string) int {
	count := 0
	for _, row := range rows {
		if row.Status == status {
			count++
		}
	}
	return count
}

// BulkItemAction runs move or archive over a bounded set of items.
func (s *Service) BulkItemAction(ctx context.Context, action string, itemIDs []string, groupID string, dryRun bool) (BulkResult, error) {
	if err := oneOf("action", action, "move", "archive"); err != nil || action == "" {
		return BulkResult{}, invalid("action must be move or archive")
	}
	if err := requireIDs("item_ids", itemIDs, MaxBulkItems); err != nil {
		return BulkResult{}, err
	}
	if action == "move" {
		if err := requireText("group_id", groupID); err != nil {
			return BulkResult{}, err
		}
	}
	items, err := s.port.GetItems(ctx, itemIDs)
	if err != nil {
		return BulkResult{}, err
	}
	found := map[string]domain.Item{}
	for _, item := range items {
		found[item.ID] = item
	}
	result := BulkResult{DryRun: dryRun, Planned: len(itemIDs)}
	for _, id := range itemIDs {
		row := BulkRowOutcome{ItemID: id, Status: "planned"}
		item, ok := found[id]
		switch {
		case !ok:
			row.Status, row.Error = "invalid", "item not found or not visible"
		case !dryRun:
			if err := s.guard.CheckBoard(item.BoardID); err != nil {
				row.Status, row.Error = "failed", err.Error()
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if dryRun {
		result.Failed = countStatus(result.Rows, "invalid")
		return result, nil
	}
	for index := range result.Rows {
		row := &result.Rows[index]
		if row.Status != "planned" {
			result.Failed++
			continue
		}
		var err error
		if action == "move" {
			_, err = s.port.MoveItem(ctx, row.ItemID, groupID)
		} else {
			_, err = s.port.ArchiveItem(ctx, row.ItemID)
		}
		if err != nil {
			row.Status, row.Error = "failed", err.Error()
			result.Failed++
			continue
		}
		row.Status = map[string]string{"move": "moved", "archive": "archived"}[action]
		result.Succeeded++
	}
	return result, nil
}
