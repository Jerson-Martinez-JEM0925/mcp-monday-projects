package mcpserver

import (
	"context"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// ItemIDInput identifies one item.
type ItemIDInput struct {
	ItemID string `json:"item_id" jsonschema:"monday item identifier"`
}

// ListItemsInput reads one cursor page of items.
type ListItemsInput struct {
	BoardID string             `json:"board_id" jsonschema:"monday board identifier"`
	Limit   int                `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
	Cursor  string             `json:"cursor,omitempty" jsonschema:"cursor from a previous page; when set, board_id and filter are ignored"`
	Filter  *domain.ItemFilter `json:"filter,omitempty" jsonschema:"optional server-side filter rules"`
}

// ListGroupItemsInput reads items of one group.
type ListGroupItemsInput struct {
	BoardID string `json:"board_id" jsonschema:"monday board identifier"`
	GroupID string `json:"group_id" jsonschema:"group identifier"`
	Limit   int    `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"cursor from a previous page"`
}

// ItemsOutput is one page of items.
type ItemsOutput struct {
	Items   []monday.Item `json:"items"`
	Count   int           `json:"count"`
	Cursor  string        `json:"cursor,omitempty"`
	HasMore bool          `json:"has_more"`
}

// SubitemsOutput lists subitems.
type SubitemsOutput struct {
	Subitems []domain.Subitem `json:"subitems"`
	Count    int              `json:"count"`
}

func pageOutput(page domain.ItemPage) ItemsOutput {
	return ItemsOutput{Items: page.Items, Count: len(page.Items), Cursor: page.Cursor, HasMore: page.Cursor != ""}
}

// SearchItemsInput searches items.
type SearchItemsInput struct {
	BoardID string             `json:"board_id" jsonschema:"monday board identifier"`
	Text    string             `json:"text,omitempty" jsonschema:"text contained in the item name"`
	Filter  *domain.ItemFilter `json:"filter,omitempty" jsonschema:"additional filter rules (combined with AND by default)"`
	Limit   int                `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
}

// FindByValuesInput matches exact column values.
type FindByValuesInput struct {
	BoardID string               `json:"board_id" jsonschema:"monday board identifier"`
	Columns []monday.ColumnMatch `json:"columns" jsonschema:"exact matches: [{column_id, column_values: [text...]}]"`
	Limit   int                  `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
	Cursor  string               `json:"cursor,omitempty" jsonschema:"cursor from a previous page"`
}

// ItemIDsInput identifies several items.
type ItemIDsInput struct {
	ItemIDs []string `json:"item_ids" jsonschema:"up to 100 item IDs"`
}

// ItemOutput wraps one item.
type ItemOutput struct {
	Item monday.Item `json:"item"`
}

// ItemValuesInput updates column values.
type ItemValuesInput struct {
	BoardID      string         `json:"board_id" jsonschema:"monday board identifier"`
	ItemID       string         `json:"item_id" jsonschema:"monday item identifier"`
	ColumnValues map[string]any `json:"column_values" jsonschema:"column ID to friendly value mapping; validated against the board schema (see describe_column_formats)"`
}

// ValidateValuesInput validates without writing.
type ValidateValuesInput struct {
	BoardID      string         `json:"board_id" jsonschema:"monday board identifier"`
	ColumnValues map[string]any `json:"column_values" jsonschema:"column ID to friendly value mapping"`
}

// PlanOutput returns a validation plan.
type PlanOutput struct {
	Plan domain.ColumnValuePlan `json:"plan"`
}

// CreateItemInput creates an item.
type CreateItemInput struct {
	BoardID      string         `json:"board_id" jsonschema:"monday board identifier"`
	GroupID      string         `json:"group_id,omitempty" jsonschema:"optional monday group identifier"`
	Name         string         `json:"name" jsonschema:"item name"`
	ColumnValues map[string]any `json:"column_values,omitempty" jsonschema:"optional column ID to friendly value mapping; validated before writing"`
}

// CreateSubitemInput creates a subitem.
type CreateSubitemInput struct {
	ParentItemID string         `json:"parent_item_id" jsonschema:"parent item identifier"`
	Name         string         `json:"name" jsonschema:"subitem name"`
	ColumnValues map[string]any `json:"column_values,omitempty" jsonschema:"optional values for the subitems board columns"`
}

// SetStatusInput sets a status label.
type SetStatusInput struct {
	BoardID  string `json:"board_id" jsonschema:"monday board identifier"`
	ItemID   string `json:"item_id" jsonschema:"monday item identifier"`
	Label    string `json:"label" jsonschema:"status label text (validated against the column labels)"`
	ColumnID string `json:"column_id,omitempty" jsonschema:"status column; auto-detected when empty"`
}

// SetDateInput sets a date.
type SetDateInput struct {
	BoardID  string `json:"board_id" jsonschema:"monday board identifier"`
	ItemID   string `json:"item_id" jsonschema:"monday item identifier"`
	Date     string `json:"date" jsonschema:"YYYY-MM-DD or 'YYYY-MM-DD HH:MM'; empty clears the date"`
	ColumnID string `json:"column_id,omitempty" jsonschema:"date column; auto-detected when empty (prefers due/deadline)"`
}

// AssignPeopleInput assigns people.
type AssignPeopleInput struct {
	BoardID  string   `json:"board_id" jsonschema:"monday board identifier"`
	ItemID   string   `json:"item_id" jsonschema:"monday item identifier"`
	UserIDs  []string `json:"user_ids" jsonschema:"user IDs (or team:<id>); empty clears the column"`
	ColumnID string   `json:"column_id,omitempty" jsonschema:"people column; auto-detected when empty"`
}

// ColumnWriteOutput reports a typed single-column write.
type ColumnWriteOutput struct {
	Item     monday.Item `json:"item"`
	ColumnID string      `json:"column_id"`
}

// RenameItemInput renames an item.
type RenameItemInput struct {
	BoardID string `json:"board_id" jsonschema:"monday board identifier"`
	ItemID  string `json:"item_id" jsonschema:"monday item identifier"`
	Name    string `json:"name" jsonschema:"new item name"`
}

// MoveItemInput moves an item to a group.
type MoveItemInput struct {
	ItemID  string `json:"item_id" jsonschema:"monday item identifier"`
	GroupID string `json:"group_id" jsonschema:"monday group identifier"`
}

// MoveItemToBoardInput moves an item to another board.
type MoveItemToBoardInput struct {
	ItemID  string `json:"item_id" jsonschema:"monday item identifier"`
	BoardID string `json:"board_id" jsonschema:"destination board"`
	GroupID string `json:"group_id" jsonschema:"destination group"`
}

// DuplicateItemInput duplicates an item.
type DuplicateItemInput struct {
	BoardID     string `json:"board_id" jsonschema:"monday board identifier"`
	ItemID      string `json:"item_id" jsonschema:"item to duplicate"`
	WithUpdates bool   `json:"with_updates,omitempty" jsonschema:"copy the item's updates too"`
}

// BulkUpdateInput updates many items.
type BulkUpdateInput struct {
	BoardID string                   `json:"board_id" jsonschema:"monday board identifier"`
	Updates []application.BulkUpdate `json:"updates" jsonschema:"rows of {item_id, column_values} (max 50)"`
	DryRun  *bool                    `json:"dry_run,omitempty" jsonschema:"default true: validate and plan only; set false to write"`
}

// BulkMoveInput moves many items.
type BulkMoveInput struct {
	ItemIDs []string `json:"item_ids" jsonschema:"items to move (max 50)"`
	GroupID string   `json:"group_id" jsonschema:"destination group"`
	DryRun  *bool    `json:"dry_run,omitempty" jsonschema:"default true: plan only; set false to move"`
}

// BulkArchiveInput archives many items.
type BulkArchiveInput struct {
	ItemIDs []string `json:"item_ids" jsonschema:"items to archive (max 50)"`
	DryRun  *bool    `json:"dry_run,omitempty" jsonschema:"default true: plan only; set false to archive"`
}

// BulkOutput wraps a bulk result.
type BulkOutput struct {
	Result application.BulkResult `json:"result"`
}

func dryRun(value *bool) bool { return value == nil || *value }

func registerItemTools(r *registry) {
	svc := r.svc
	add(r, ToolSpec{Name: "list_items", Category: CatItemsRead, Title: "List items", ReadOnly: true,
		Description: "Read one cursor page of board items with column values; pass the returned cursor to continue. Supports server-side filter rules."},
		func(ctx context.Context, in ListItemsInput) (ItemsOutput, error) {
			page, err := svc.ListItems(ctx, monday.ItemPageQuery{BoardID: in.BoardID, Limit: in.Limit, Cursor: in.Cursor, Filter: in.Filter})
			if err != nil {
				return ItemsOutput{}, wrap("list items", err)
			}
			return pageOutput(page), nil
		})
	add(r, ToolSpec{Name: "list_group_items", Category: CatItemsRead, Title: "List group items", ReadOnly: true,
		Description: "Read one cursor page of items in a single group."},
		func(ctx context.Context, in ListGroupItemsInput) (ItemsOutput, error) {
			if err := requireGroup(in.GroupID); err != nil {
				return ItemsOutput{}, err
			}
			page, err := svc.ListItems(ctx, monday.ItemPageQuery{BoardID: in.BoardID, GroupID: in.GroupID, Limit: in.Limit, Cursor: in.Cursor})
			if err != nil {
				return ItemsOutput{}, wrap("list group items", err)
			}
			return pageOutput(page), nil
		})
	add(r, ToolSpec{Name: "search_items", Category: CatItemsRead, Title: "Search items", ReadOnly: true,
		Description: "Search a board for items whose name contains text, optionally combined with filter rules (status any_of, date within_the_next, etc.)."},
		func(ctx context.Context, in SearchItemsInput) (ItemsOutput, error) {
			page, err := svc.SearchItems(ctx, in.BoardID, in.Text, in.Filter, in.Limit)
			if err != nil {
				return ItemsOutput{}, wrap("search items", err)
			}
			return pageOutput(page), nil
		})
	add(r, ToolSpec{Name: "find_items_by_column_values", Category: CatItemsRead, Title: "Find items by column values", ReadOnly: true,
		Description: "Find items whose columns exactly match given display values (e.g. status=Done and environment=prod)."},
		func(ctx context.Context, in FindByValuesInput) (ItemsOutput, error) {
			page, err := svc.FindItemsByColumnValues(ctx, in.BoardID, in.Columns, in.Limit, in.Cursor)
			if err != nil {
				return ItemsOutput{}, wrap("find items", err)
			}
			return pageOutput(page), nil
		})
	add(r, ToolSpec{Name: "get_item", Category: CatItemsRead, Title: "Get item", ReadOnly: true,
		Description: "Get one item with its board, group, creator, column values, and subitems."},
		func(ctx context.Context, in ItemIDInput) (ItemOutput, error) {
			item, err := svc.GetItem(ctx, in.ItemID)
			if err != nil {
				return ItemOutput{}, wrap("get item", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "get_items", Category: CatItemsRead, Title: "Get items", ReadOnly: true,
		Description: "Get up to 100 items by ID in one request."},
		func(ctx context.Context, in ItemIDsInput) (ItemsOutput, error) {
			items, err := svc.GetItems(ctx, in.ItemIDs)
			if err != nil {
				return ItemsOutput{}, wrap("get items", err)
			}
			return ItemsOutput{Items: items, Count: len(items)}, nil
		})
	add(r, ToolSpec{Name: "list_subitems", Category: CatItemsRead, Title: "List subitems", ReadOnly: true,
		Description: "List the subitems of an item."},
		func(ctx context.Context, in ItemIDInput) (SubitemsOutput, error) {
			item, err := svc.GetItem(ctx, in.ItemID)
			if err != nil {
				return SubitemsOutput{}, wrap("list subitems", err)
			}
			return SubitemsOutput{Subitems: item.Subitems, Count: len(item.Subitems)}, nil
		})
	add(r, ToolSpec{Name: "validate_column_values", Category: CatItemsRead, Title: "Validate column values", ReadOnly: true, Capability: "items.read",
		Description: "Dry-run: validate friendly column values against a board schema and return the exact monday JSON that a write would send, or per-column issues."},
		func(ctx context.Context, in ValidateValuesInput) (PlanOutput, error) {
			plan, err := svc.PlanValues(ctx, in.BoardID, in.ColumnValues)
			if err != nil {
				return PlanOutput{}, wrap("validate column values", err)
			}
			return PlanOutput{Plan: plan}, nil
		})

	// Writes
	add(r, ToolSpec{Name: "create_item", Category: CatItemsWrite, Title: "Create item",
		Description: "Create an item; column values are validated by type against the board schema before monday is called."},
		func(ctx context.Context, in CreateItemInput) (ItemOutput, error) {
			item, err := svc.CreateItem(ctx, in.BoardID, in.GroupID, in.Name, in.ColumnValues)
			if err != nil {
				return ItemOutput{}, wrap("create item", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "create_subitem", Category: CatItemsWrite, Title: "Create subitem",
		Description: "Create a subitem under a parent item; values are validated against the subitems board."},
		func(ctx context.Context, in CreateSubitemInput) (ItemOutput, error) {
			item, err := svc.CreateSubitem(ctx, in.ParentItemID, in.Name, in.ColumnValues)
			if err != nil {
				return ItemOutput{}, wrap("create subitem", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "update_item_column_values", Category: CatItemsWrite, Title: "Update column values",
		Description: "Update several column values of an item in one mutation, after typed validation."},
		func(ctx context.Context, in ItemValuesInput) (ItemOutput, error) {
			item, err := svc.UpdateItemValues(ctx, in.BoardID, in.ItemID, in.ColumnValues)
			if err != nil {
				return ItemOutput{}, wrap("update item values", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "set_item_status", Category: CatItemsWrite, Title: "Set item status",
		Description: "Set a status label on an item; the status column is auto-detected and the label is validated."},
		func(ctx context.Context, in SetStatusInput) (ColumnWriteOutput, error) {
			item, column, err := svc.SetDetectedColumn(ctx, in.BoardID, in.ItemID, "status", in.ColumnID, in.Label)
			if err != nil {
				return ColumnWriteOutput{}, wrap("set item status", err)
			}
			return ColumnWriteOutput{Item: *item, ColumnID: column}, nil
		})
	add(r, ToolSpec{Name: "set_item_date", Category: CatItemsWrite, Title: "Set item date",
		Description: "Set or clear a date on an item; the date column is auto-detected (prefers due/deadline)."},
		func(ctx context.Context, in SetDateInput) (ColumnWriteOutput, error) {
			var value any = in.Date
			if in.Date == "" {
				value = nil
			}
			item, column, err := svc.SetDetectedColumn(ctx, in.BoardID, in.ItemID, "date", in.ColumnID, value)
			if err != nil {
				return ColumnWriteOutput{}, wrap("set item date", err)
			}
			return ColumnWriteOutput{Item: *item, ColumnID: column}, nil
		})
	add(r, ToolSpec{Name: "assign_item_people", Category: CatItemsWrite, Title: "Assign people",
		Description: "Assign users or teams to an item's people column (auto-detected); an empty list clears it."},
		func(ctx context.Context, in AssignPeopleInput) (ColumnWriteOutput, error) {
			var value any
			if len(in.UserIDs) > 0 {
				ids := make([]any, 0, len(in.UserIDs))
				for _, id := range in.UserIDs {
					ids = append(ids, id)
				}
				value = ids
			}
			item, column, err := svc.SetDetectedColumn(ctx, in.BoardID, in.ItemID, "people", in.ColumnID, value)
			if err != nil {
				return ColumnWriteOutput{}, wrap("assign item people", err)
			}
			return ColumnWriteOutput{Item: *item, ColumnID: column}, nil
		})
	add(r, ToolSpec{Name: "rename_item", Category: CatItemsWrite, Title: "Rename item",
		Description: "Rename an item."},
		func(ctx context.Context, in RenameItemInput) (ItemOutput, error) {
			item, err := svc.RenameItem(ctx, in.BoardID, in.ItemID, in.Name)
			if err != nil {
				return ItemOutput{}, wrap("rename item", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "move_item", Category: CatItemsWrite, Title: "Move item to group",
		Description: "Move a monday.com item to another group."},
		func(ctx context.Context, in MoveItemInput) (ItemOutput, error) {
			item, err := svc.MoveItem(ctx, in.ItemID, in.GroupID)
			if err != nil {
				return ItemOutput{}, wrap("move item", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "move_item_to_board", Category: CatItemsWrite, Title: "Move item to board",
		Description: "Move an item to a group on another board (both boards must be writable)."},
		func(ctx context.Context, in MoveItemToBoardInput) (ItemOutput, error) {
			item, err := svc.MoveItemToBoard(ctx, in.ItemID, in.BoardID, in.GroupID)
			if err != nil {
				return ItemOutput{}, wrap("move item to board", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "duplicate_item", Category: CatItemsWrite, Title: "Duplicate item",
		Description: "Duplicate an item, optionally with its updates."},
		func(ctx context.Context, in DuplicateItemInput) (ItemOutput, error) {
			item, err := svc.DuplicateItem(ctx, in.BoardID, in.ItemID, in.WithUpdates)
			if err != nil {
				return ItemOutput{}, wrap("duplicate item", err)
			}
			return ItemOutput{Item: *item}, nil
		})
	add(r, ToolSpec{Name: "archive_item", Category: CatItemsWrite, Title: "Archive item", Destructive: true,
		Description: "Archive (never delete) a monday.com item; it can be restored from monday's archive."},
		func(ctx context.Context, in ItemIDInput) (ItemOutput, error) {
			item, err := svc.ArchiveItem(ctx, in.ItemID)
			if err != nil {
				return ItemOutput{}, wrap("archive item", err)
			}
			return ItemOutput{Item: *item}, nil
		})

	// Bulk
	add(r, ToolSpec{Name: "bulk_update_items", Category: CatBulk, Title: "Bulk update items",
		Description: "Validate and apply column values to up to 50 items. dry_run defaults to true; nothing is written if any row is invalid."},
		func(ctx context.Context, in BulkUpdateInput) (BulkOutput, error) {
			result, err := svc.BulkUpdateItems(ctx, in.BoardID, in.Updates, dryRun(in.DryRun))
			if err != nil {
				return BulkOutput{}, wrap("bulk update items", err)
			}
			return BulkOutput{Result: result}, nil
		})
	add(r, ToolSpec{Name: "bulk_move_items", Category: CatBulk, Title: "Bulk move items",
		Description: "Move up to 50 items to a group. dry_run defaults to true."},
		func(ctx context.Context, in BulkMoveInput) (BulkOutput, error) {
			result, err := svc.BulkItemAction(ctx, "move", in.ItemIDs, in.GroupID, dryRun(in.DryRun))
			if err != nil {
				return BulkOutput{}, wrap("bulk move items", err)
			}
			return BulkOutput{Result: result}, nil
		})
	add(r, ToolSpec{Name: "bulk_archive_items", Category: CatBulk, Title: "Bulk archive items", Destructive: true,
		Description: "Archive (never delete) up to 50 items. dry_run defaults to true."},
		func(ctx context.Context, in BulkArchiveInput) (BulkOutput, error) {
			result, err := svc.BulkItemAction(ctx, "archive", in.ItemIDs, "", dryRun(in.DryRun))
			if err != nil {
				return BulkOutput{}, wrap("bulk archive items", err)
			}
			return BulkOutput{Result: result}, nil
		})
}

func requireGroup(groupID string) error {
	if groupID == "" {
		return wrap("list group items", errGroupRequired)
	}
	return nil
}
