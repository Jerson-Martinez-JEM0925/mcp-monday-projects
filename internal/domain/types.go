// Package domain holds provider-independent business types, column-value
// validation, board templates, and pure report calculations. It has no
// knowledge of GraphQL, HTTP, or MCP.
package domain

import "encoding/json"

// Account describes the monday.com account that owns the API token.
type Account struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Slug               string `json:"slug"`
	Tier               string `json:"tier,omitempty"`
	CountryCode        string `json:"country_code,omitempty"`
	ActiveMembersCount int    `json:"active_members_count,omitempty"`
}

// Workspace is a monday workspace.
type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
}

// Folder is a workspace folder that groups boards.
type Folder struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Color     string     `json:"color,omitempty"`
	Workspace *Workspace `json:"workspace,omitempty"`
}

// Board is a monday board.
type Board struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
	BoardKind   string `json:"board_kind,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ItemsCount  int    `json:"items_count,omitempty"`
	URL         string `json:"url,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// Group is a board group (a section of items).
type Group struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Color    string `json:"color,omitempty"`
	Position string `json:"position,omitempty"`
	Archived bool   `json:"archived,omitempty"`
}

// JSONObject is a free-form JSON object. It accepts monday's JSON scalar
// either as an object or as a JSON-encoded string, and always renders as an
// object so tool output schemas stay accurate.
type JSONObject map[string]any

// UnmarshalJSON implements json.Unmarshaler.
func (o *JSONObject) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*o = nil
		return nil
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		if text == "" {
			*o = nil
			return nil
		}
		data = []byte(text)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*o = m
	return nil
}

// Column is a board column definition. Settings is the raw monday settings
// object (labels for status/dropdown columns, linked boards, etc.).
type Column struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Description string     `json:"description,omitempty"`
	Archived    bool       `json:"archived,omitempty"`
	Settings    JSONObject `json:"settings,omitempty"`
}

// ColumnValue is the display text and raw JSON value of one item cell.
type ColumnValue struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Value string `json:"value,omitempty"`
	Type  string `json:"type"`
}

// UserRef is a compact reference to a user.
type UserRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Item is a board item (row).
type Item struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	State        string        `json:"state,omitempty"`
	URL          string        `json:"url,omitempty"`
	CreatedAt    string        `json:"created_at,omitempty"`
	UpdatedAt    string        `json:"updated_at,omitempty"`
	BoardID      string        `json:"board_id,omitempty"`
	Group        *Group        `json:"group,omitempty"`
	Creator      *UserRef      `json:"creator,omitempty"`
	ParentItemID string        `json:"parent_item_id,omitempty"`
	ColumnValues []ColumnValue `json:"column_values,omitempty"`
	Subitems     []Subitem     `json:"subitems,omitempty"`
}

// Subitem is a child item. It is a separate flat type so tool schemas stay
// acyclic (JSON Schema inference rejects self-referencing types).
type Subitem struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	State        string        `json:"state,omitempty"`
	URL          string        `json:"url,omitempty"`
	UpdatedAt    string        `json:"updated_at,omitempty"`
	ColumnValues []ColumnValue `json:"column_values,omitempty"`
}

// ItemPage is one cursor page of items.
type ItemPage struct {
	Items  []Item `json:"items"`
	Cursor string `json:"cursor,omitempty"`
}

// TeamRef is a compact reference to a team.
type TeamRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User is a monday user.
type User struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email,omitempty"`
	Title    string    `json:"title,omitempty"`
	Kind     string    `json:"kind,omitempty"`
	Status   string    `json:"status,omitempty"`
	TimeZone string    `json:"time_zone,omitempty"`
	URL      string    `json:"url,omitempty"`
	Teams    []TeamRef `json:"teams,omitempty"`
}

// Team is a monday team with its members.
type Team struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Users []UserRef `json:"users,omitempty"`
}

// Reply is a reply on an update.
type Reply struct {
	ID        string   `json:"id"`
	TextBody  string   `json:"text_body,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	Creator   *UserRef `json:"creator,omitempty"`
}

// Update is an item update (comment thread root).
type Update struct {
	ID        string   `json:"id"`
	Body      string   `json:"body,omitempty"`
	TextBody  string   `json:"text_body,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	ItemID    string   `json:"item_id,omitempty"`
	Creator   *UserRef `json:"creator,omitempty"`
	Replies   []Reply  `json:"replies,omitempty"`
}

// Tag is an account or board tag.
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// Me describes the authenticated user.
type Me struct {
	User
	IsAdmin bool     `json:"is_admin"`
	Account *Account `json:"account,omitempty"`
}

// Complexity is monday's per-minute query complexity budget.
type Complexity struct {
	Before          int `json:"before"`
	After           int `json:"after"`
	Query           int `json:"query"`
	ResetInXSeconds int `json:"reset_in_x_seconds"`
}

// APIVersion is the API version that served the request.
type APIVersion struct {
	Value string `json:"value"`
	Kind  string `json:"kind"`
}

// ItemFilterRule is a provider-neutral item query rule.
type ItemFilterRule struct {
	ColumnID     string `json:"column_id" jsonschema:"column ID to filter on (use 'name' for the item name)"`
	CompareValue []any  `json:"compare_value,omitempty" jsonschema:"values to compare; status/dropdown accept label indexes or texts"`
	Operator     string `json:"operator,omitempty" jsonschema:"any_of (default), not_any_of, is_empty, is_not_empty, greater_than, lower_than, between, contains_text, not_contains_text, starts_with, ends_with, within_the_last, within_the_next"`
}

// ItemFilter combines rules with a logical operator.
type ItemFilter struct {
	Rules    []ItemFilterRule `json:"rules,omitempty"`
	Operator string           `json:"operator,omitempty"`
}
