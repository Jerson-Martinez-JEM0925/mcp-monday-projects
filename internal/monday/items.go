package monday

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
)

// Provider-neutral types re-exported for adapter callers and existing tests.
type (
	Group       = domain.Group
	Column      = domain.Column
	ColumnValue = domain.ColumnValue
	Item        = domain.Item
)

const itemFields = `id name state url created_at updated_at
  board { id }
  group { id title }
  creator { id name }
  parent_item { id }
  column_values { id text value type }`

const listGroupsQuery = `query ListGroups($boardID: ID!) {
  boards(ids: [$boardID]) { groups { id title color position archived } }
}`

const listColumnsQuery = `query ListColumns($boardID: ID!) {
  boards(ids: [$boardID]) { columns { id title type description archived settings } }
}`

const listItemsQuery = `query ListItems($boardID: ID!, $limit: Int!, $query: ItemsQuery) {
  boards(ids: [$boardID]) {
    items_page(limit: $limit, query_params: $query) { cursor items { ` + itemFields + ` } }
  }
}`

const listGroupItemsQuery = `query ListGroupItems($boardID: ID!, $groupID: String!, $limit: Int!, $query: ItemsQuery) {
  boards(ids: [$boardID]) {
    groups(ids: [$groupID]) {
      items_page(limit: $limit, query_params: $query) { cursor items { ` + itemFields + ` } }
    }
  }
}`

const nextItemsPageQuery = `query NextItemsPage($cursor: String!, $limit: Int!) {
  next_items_page(cursor: $cursor, limit: $limit) { cursor items { ` + itemFields + ` } }
}`

const itemsByColumnValuesQuery = `query ItemsByColumnValues($boardID: ID!, $limit: Int!, $columns: [ItemsPageByColumnValuesQuery!], $cursor: String) {
  items_page_by_column_values(board_id: $boardID, limit: $limit, columns: $columns, cursor: $cursor) { cursor items { ` + itemFields + ` } }
}`

const getItemsQuery = `query GetItems($ids: [ID!]!, $limit: Int!) {
  items(ids: $ids, limit: $limit) { ` + itemFields + `
    subitems { id name state url updated_at column_values { id text value type } }
  }
}`

const createItemMutation = `mutation CreateItem($boardID: ID!, $groupID: String, $itemName: String!, $columnValues: JSON, $createLabels: Boolean) {
  create_item(board_id: $boardID, group_id: $groupID, item_name: $itemName, column_values: $columnValues, create_labels_if_missing: $createLabels) { ` + itemFields + ` }
}`

const createSubitemMutation = `mutation CreateSubitem($parentID: ID!, $itemName: String!, $columnValues: JSON, $createLabels: Boolean) {
  create_subitem(parent_item_id: $parentID, item_name: $itemName, column_values: $columnValues, create_labels_if_missing: $createLabels) { ` + itemFields + ` }
}`

const updateItemValuesMutation = `mutation UpdateItemValues($boardID: ID!, $itemID: ID!, $columnValues: JSON!, $createLabels: Boolean) {
  change_multiple_column_values(board_id: $boardID, item_id: $itemID, column_values: $columnValues, create_labels_if_missing: $createLabels) { ` + itemFields + ` }
}`

const moveItemMutation = `mutation MoveItem($itemID: ID!, $groupID: String!) {
  move_item_to_group(item_id: $itemID, group_id: $groupID) { ` + itemFields + ` }
}`

const moveItemToBoardMutation = `mutation MoveItemToBoard($itemID: ID!, $boardID: ID!, $groupID: ID!) {
  move_item_to_board(item_id: $itemID, board_id: $boardID, group_id: $groupID) { ` + itemFields + ` }
}`

const duplicateItemMutation = `mutation DuplicateItem($boardID: ID!, $itemID: ID!, $withUpdates: Boolean) {
  duplicate_item(board_id: $boardID, item_id: $itemID, with_updates: $withUpdates) { ` + itemFields + ` }
}`

const archiveItemMutation = `mutation ArchiveItem($itemID: ID!) {
  archive_item(item_id: $itemID) { id name state url }
}`

const createGroupMutation = `mutation CreateGroup($boardID: ID!, $name: String!, $color: String, $relativeTo: String, $method: PositionRelative) {
  create_group(board_id: $boardID, group_name: $name, group_color: $color, relative_to: $relativeTo, position_relative_method: $method) { id title color position }
}`

const updateGroupMutation = `mutation UpdateGroup($boardID: ID!, $groupID: String!, $attribute: GroupAttributes!, $value: String!) {
  update_group(board_id: $boardID, group_id: $groupID, group_attribute: $attribute, new_value: $value) { id title color position }
}`

const duplicateGroupMutation = `mutation DuplicateGroup($boardID: ID!, $groupID: String!, $title: String, $addToTop: Boolean) {
  duplicate_group(board_id: $boardID, group_id: $groupID, group_title: $title, add_to_top: $addToTop) { id title color position }
}`

const archiveGroupMutation = `mutation ArchiveGroup($boardID: ID!, $groupID: String!) {
  archive_group(board_id: $boardID, group_id: $groupID) { id title archived }
}`

const createColumnMutation = `mutation CreateColumn($boardID: ID!, $title: String!, $type: ColumnType!, $id: String, $description: String, $defaults: JSON, $after: ID) {
  create_column(board_id: $boardID, title: $title, column_type: $type, id: $id, description: $description, defaults: $defaults, after_column_id: $after) { id title type description archived settings }
}`

const changeColumnTitleMutation = `mutation ChangeColumnTitle($boardID: ID!, $columnID: String!, $title: String!) {
  change_column_title(board_id: $boardID, column_id: $columnID, title: $title) { id title type description archived settings }
}`

const changeColumnMetadataMutation = `mutation ChangeColumnMetadata($boardID: ID!, $columnID: String!, $property: ColumnProperty, $value: String) {
  change_column_metadata(board_id: $boardID, column_id: $columnID, column_property: $property, value: $value) { id title type description archived settings }
}`

// wireItem mirrors monday's nested item payload.
type wireItem struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	State        string               `json:"state"`
	URL          string               `json:"url"`
	CreatedAt    string               `json:"created_at"`
	UpdatedAt    string               `json:"updated_at"`
	Board        *struct{ ID string } `json:"board"`
	Group        *domain.Group        `json:"group"`
	Creator      *domain.UserRef      `json:"creator"`
	ParentItem   *struct{ ID string } `json:"parent_item"`
	ColumnValues []wireColumnValue    `json:"column_values"`
	Subitems     []wireItem           `json:"subitems"`
}

type wireColumnValue struct {
	ID    string          `json:"id"`
	Text  *string         `json:"text"`
	Value json.RawMessage `json:"value"`
	Type  string          `json:"type"`
}

func (w wireItem) toDomain() domain.Item {
	item := domain.Item{ID: w.ID, Name: w.Name, State: w.State, URL: w.URL, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt, Group: w.Group, Creator: w.Creator}
	if w.Board != nil {
		item.BoardID = w.Board.ID
	}
	if w.ParentItem != nil {
		item.ParentItemID = w.ParentItem.ID
	}
	for _, value := range w.ColumnValues {
		cell := domain.ColumnValue{ID: value.ID, Type: value.Type}
		if value.Text != nil {
			cell.Text = *value.Text
		}
		cell.Value = rawJSONString(value.Value)
		item.ColumnValues = append(item.ColumnValues, cell)
	}
	for _, sub := range w.Subitems {
		child := sub.toDomain()
		item.Subitems = append(item.Subitems, domain.Subitem{ID: child.ID, Name: child.Name, State: child.State, URL: child.URL, UpdatedAt: child.UpdatedAt, ColumnValues: child.ColumnValues})
	}
	return item
}

// rawJSONString returns monday's JSON value as a compact JSON document string.
func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var encoded string
	if json.Unmarshal(raw, &encoded) == nil {
		return encoded
	}
	return string(raw)
}

type wirePage struct {
	Cursor *string    `json:"cursor"`
	Items  []wireItem `json:"items"`
}

func (p wirePage) toDomain() domain.ItemPage {
	page := domain.ItemPage{Items: make([]domain.Item, 0, len(p.Items))}
	if p.Cursor != nil {
		page.Cursor = *p.Cursor
	}
	for _, item := range p.Items {
		page.Items = append(page.Items, item.toDomain())
	}
	return page
}

// encodeJSONArg serializes a value for a monday JSON scalar argument, which
// must be sent as a JSON-encoded string.
func encodeJSONArg(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if m, ok := value.(map[string]any); ok && len(m) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode JSON argument: %w", err)
	}
	return string(encoded), nil
}

// ItemPageQuery controls one items_page request.
type ItemPageQuery struct {
	BoardID string
	GroupID string
	Limit   int
	Cursor  string
	Filter  *domain.ItemFilter
}

func buildItemsQuery(filter *domain.ItemFilter) any {
	if filter == nil || len(filter.Rules) == 0 {
		return nil
	}
	rules := make([]map[string]any, 0, len(filter.Rules))
	for _, rule := range filter.Rules {
		entry := map[string]any{"column_id": rule.ColumnID, "compare_value": rule.CompareValue}
		if rule.CompareValue == nil {
			entry["compare_value"] = []any{}
		}
		if rule.Operator != "" {
			entry["operator"] = rule.Operator
		}
		rules = append(rules, entry)
	}
	query := map[string]any{"rules": rules}
	if filter.Operator != "" {
		query["operator"] = filter.Operator
	}
	return query
}

// ListGroups lists the groups of a board.
func (c *Client) ListGroups(ctx context.Context, boardID string) ([]Group, error) {
	var data struct {
		Boards []struct {
			Groups []Group `json:"groups"`
		} `json:"boards"`
	}
	if err := c.Do(ctx, listGroupsQuery, map[string]any{"boardID": boardID}, &data); err != nil {
		return nil, err
	}
	if len(data.Boards) == 0 {
		return nil, &NotFoundError{Resource: "board", ID: boardID}
	}
	return data.Boards[0].Groups, nil
}

// ListColumns lists the columns of a board.
func (c *Client) ListColumns(ctx context.Context, boardID string) ([]Column, error) {
	var data struct {
		Boards []struct {
			Columns []Column `json:"columns"`
		} `json:"boards"`
	}
	if err := c.Do(ctx, listColumnsQuery, map[string]any{"boardID": boardID}, &data); err != nil {
		return nil, err
	}
	if len(data.Boards) == 0 {
		return nil, &NotFoundError{Resource: "board", ID: boardID}
	}
	return data.Boards[0].Columns, nil
}

// ListItems returns the first page of items of a board.
func (c *Client) ListItems(ctx context.Context, boardID string, limit int) ([]Item, error) {
	page, err := c.ListItemsPage(ctx, ItemPageQuery{BoardID: boardID, Limit: limit})
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// ListItemsPage returns one cursor page of items, optionally filtered or
// scoped to a group. A non-empty Cursor continues a previous page.
func (c *Client) ListItemsPage(ctx context.Context, query ItemPageQuery) (domain.ItemPage, error) {
	if query.Cursor != "" {
		var data struct {
			Page wirePage `json:"next_items_page"`
		}
		if err := c.Do(ctx, nextItemsPageQuery, map[string]any{"cursor": query.Cursor, "limit": query.Limit}, &data); err != nil {
			return domain.ItemPage{}, err
		}
		return data.Page.toDomain(), nil
	}
	vars := map[string]any{"boardID": query.BoardID, "limit": query.Limit, "query": buildItemsQuery(query.Filter)}
	if query.GroupID != "" {
		vars["groupID"] = query.GroupID
		var data struct {
			Boards []struct {
				Groups []struct {
					Page wirePage `json:"items_page"`
				} `json:"groups"`
			} `json:"boards"`
		}
		if err := c.Do(ctx, listGroupItemsQuery, vars, &data); err != nil {
			return domain.ItemPage{}, err
		}
		if len(data.Boards) == 0 {
			return domain.ItemPage{}, &NotFoundError{Resource: "board", ID: query.BoardID}
		}
		if len(data.Boards[0].Groups) == 0 {
			return domain.ItemPage{}, &NotFoundError{Resource: "group", ID: query.GroupID}
		}
		return data.Boards[0].Groups[0].Page.toDomain(), nil
	}
	var data struct {
		Boards []struct {
			Page wirePage `json:"items_page"`
		} `json:"boards"`
	}
	if err := c.Do(ctx, listItemsQuery, vars, &data); err != nil {
		return domain.ItemPage{}, err
	}
	if len(data.Boards) == 0 {
		return domain.ItemPage{}, &NotFoundError{Resource: "board", ID: query.BoardID}
	}
	return data.Boards[0].Page.toDomain(), nil
}

// ColumnMatch is one items_page_by_column_values condition.
type ColumnMatch struct {
	ColumnID string   `json:"column_id"`
	Values   []string `json:"column_values"`
}

// ItemsByColumnValues finds items whose columns exactly match values.
func (c *Client) ItemsByColumnValues(ctx context.Context, boardID string, matches []ColumnMatch, limit int, cursor string) (domain.ItemPage, error) {
	columns := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		columns = append(columns, map[string]any{"column_id": match.ColumnID, "column_values": match.Values})
	}
	var data struct {
		Page wirePage `json:"items_page_by_column_values"`
	}
	vars := map[string]any{"boardID": boardID, "limit": limit, "columns": columns, "cursor": optional(cursor)}
	if cursor != "" {
		vars["columns"] = nil
	}
	if err := c.Do(ctx, itemsByColumnValuesQuery, vars, &data); err != nil {
		return domain.ItemPage{}, err
	}
	return data.Page.toDomain(), nil
}

// GetItems returns items (with subitems) by ID.
func (c *Client) GetItems(ctx context.Context, ids []string) ([]Item, error) {
	var data struct {
		Items []wireItem `json:"items"`
	}
	if err := c.Do(ctx, getItemsQuery, map[string]any{"ids": ids, "limit": len(ids)}, &data); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(data.Items))
	for _, item := range data.Items {
		items = append(items, item.toDomain())
	}
	return items, nil
}

// GetItem returns one item by ID.
func (c *Client) GetItem(ctx context.Context, id string) (*Item, error) {
	items, err := c.GetItems(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, &NotFoundError{Resource: "item", ID: id}
	}
	return &items[0], nil
}

// CreateItem creates an item. values must already be monday-ready.
func (c *Client) CreateItem(ctx context.Context, boardID, groupID, name string, values map[string]any) (*Item, error) {
	encoded, err := encodeJSONArg(values)
	if err != nil {
		return nil, err
	}
	var data struct {
		Item wireItem `json:"create_item"`
	}
	vars := map[string]any{"boardID": boardID, "groupID": optional(groupID), "itemName": name, "columnValues": encoded, "createLabels": false}
	if err := c.Do(ctx, createItemMutation, vars, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// CreateSubitem creates a subitem under a parent item.
func (c *Client) CreateSubitem(ctx context.Context, parentID, name string, values map[string]any) (*Item, error) {
	encoded, err := encodeJSONArg(values)
	if err != nil {
		return nil, err
	}
	var data struct {
		Item wireItem `json:"create_subitem"`
	}
	vars := map[string]any{"parentID": parentID, "itemName": name, "columnValues": encoded, "createLabels": false}
	if err := c.Do(ctx, createSubitemMutation, vars, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// UpdateItemValues changes several column values in one mutation.
func (c *Client) UpdateItemValues(ctx context.Context, boardID, itemID string, values map[string]any) (*Item, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode column values: %w", err)
	}
	var data struct {
		Item wireItem `json:"change_multiple_column_values"`
	}
	vars := map[string]any{"boardID": boardID, "itemID": itemID, "columnValues": string(encoded), "createLabels": false}
	if err := c.Do(ctx, updateItemValuesMutation, vars, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// MoveItem moves an item to another group of the same board.
func (c *Client) MoveItem(ctx context.Context, itemID, groupID string) (*Item, error) {
	var data struct {
		Item wireItem `json:"move_item_to_group"`
	}
	if err := c.Do(ctx, moveItemMutation, map[string]any{"itemID": itemID, "groupID": groupID}, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// MoveItemToBoard moves an item to a group of another board.
func (c *Client) MoveItemToBoard(ctx context.Context, itemID, boardID, groupID string) (*Item, error) {
	var data struct {
		Item wireItem `json:"move_item_to_board"`
	}
	if err := c.Do(ctx, moveItemToBoardMutation, map[string]any{"itemID": itemID, "boardID": boardID, "groupID": groupID}, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// DuplicateItem duplicates an item, optionally with its updates.
func (c *Client) DuplicateItem(ctx context.Context, boardID, itemID string, withUpdates bool) (*Item, error) {
	var data struct {
		Item wireItem `json:"duplicate_item"`
	}
	if err := c.Do(ctx, duplicateItemMutation, map[string]any{"boardID": boardID, "itemID": itemID, "withUpdates": withUpdates}, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// ArchiveItem archives (never deletes) an item.
func (c *Client) ArchiveItem(ctx context.Context, itemID string) (*Item, error) {
	var data struct {
		Item wireItem `json:"archive_item"`
	}
	if err := c.Do(ctx, archiveItemMutation, map[string]any{"itemID": itemID}, &data); err != nil {
		return nil, err
	}
	item := data.Item.toDomain()
	return &item, nil
}

// CreateGroup creates a group, optionally positioned relative to another.
func (c *Client) CreateGroup(ctx context.Context, boardID, name, color, relativeTo, method string) (*Group, error) {
	var data struct {
		Group Group `json:"create_group"`
	}
	vars := map[string]any{"boardID": boardID, "name": name, "color": optional(color), "relativeTo": optional(relativeTo), "method": optional(method)}
	if err := c.Do(ctx, createGroupMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Group, nil
}

// UpdateGroup changes a group attribute (title, color, position...).
func (c *Client) UpdateGroup(ctx context.Context, boardID, groupID, attribute, value string) (*Group, error) {
	var data struct {
		Group Group `json:"update_group"`
	}
	vars := map[string]any{"boardID": boardID, "groupID": groupID, "attribute": attribute, "value": value}
	if err := c.Do(ctx, updateGroupMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Group, nil
}

// DuplicateGroup duplicates a group with its items.
func (c *Client) DuplicateGroup(ctx context.Context, boardID, groupID, title string, addToTop bool) (*Group, error) {
	var data struct {
		Group Group `json:"duplicate_group"`
	}
	vars := map[string]any{"boardID": boardID, "groupID": groupID, "title": optional(title), "addToTop": addToTop}
	if err := c.Do(ctx, duplicateGroupMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Group, nil
}

// ArchiveGroup archives (never deletes) a group.
func (c *Client) ArchiveGroup(ctx context.Context, boardID, groupID string) (*Group, error) {
	var data struct {
		Group Group `json:"archive_group"`
	}
	if err := c.Do(ctx, archiveGroupMutation, map[string]any{"boardID": boardID, "groupID": groupID}, &data); err != nil {
		return nil, err
	}
	return &data.Group, nil
}

// CreateColumnInput describes a new column.
type CreateColumnInput struct {
	BoardID       string
	ID            string
	Title         string
	Type          string
	Description   string
	Defaults      map[string]any
	AfterColumnID string
}

// CreateColumn creates a column.
func (c *Client) CreateColumn(ctx context.Context, input CreateColumnInput) (*Column, error) {
	defaults, err := encodeJSONArg(input.Defaults)
	if err != nil {
		return nil, err
	}
	var data struct {
		Column Column `json:"create_column"`
	}
	vars := map[string]any{
		"boardID": input.BoardID, "title": input.Title, "type": input.Type, "id": optional(input.ID),
		"description": optional(input.Description), "defaults": defaults, "after": optional(input.AfterColumnID),
	}
	if err := c.Do(ctx, createColumnMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Column, nil
}

// ChangeColumnTitle renames a column.
func (c *Client) ChangeColumnTitle(ctx context.Context, boardID, columnID, title string) (*Column, error) {
	var data struct {
		Column Column `json:"change_column_title"`
	}
	if err := c.Do(ctx, changeColumnTitleMutation, map[string]any{"boardID": boardID, "columnID": columnID, "title": title}, &data); err != nil {
		return nil, err
	}
	return &data.Column, nil
}

// ChangeColumnDescription sets a column description.
func (c *Client) ChangeColumnDescription(ctx context.Context, boardID, columnID, description string) (*Column, error) {
	var data struct {
		Column Column `json:"change_column_metadata"`
	}
	vars := map[string]any{"boardID": boardID, "columnID": columnID, "property": "description", "value": description}
	if err := c.Do(ctx, changeColumnMetadataMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Column, nil
}
