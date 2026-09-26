// Package applicationtest provides an in-memory monday port for tests.
package applicationtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// FakePort is an in-memory Port used by application and MCP tests. It keeps
// one board with a status/date/people schema and records every mutation.
type FakePort struct {
	mu        sync.Mutex
	Columns   map[string][]domain.Column
	Groups    map[string][]domain.Group
	Items     map[string]domain.Item
	Order     []string
	Mutations []string
	nextID    int
	PageSize  int
	FailOn    map[string]error
}

// NewFakePort returns a fake with board "100" (group "todo") and item "500".
func NewFakePort() *FakePort {
	status := domain.Column{ID: "status", Title: "Status", Type: "status", Settings: domain.JSONObject{"labels": []any{
		map[string]any{"id": 0, "label": "Working on it"}, map[string]any{"id": 1, "label": "Done", "is_done": true}, map[string]any{"id": 2, "label": "Stuck"},
	}}}
	f := &FakePort{
		Columns: map[string][]domain.Column{"100": {
			{ID: "name", Title: "Name", Type: "name"}, status,
			{ID: "due", Title: "Due date", Type: "date"}, {ID: "owner", Title: "Owner", Type: "people"}, {ID: "est", Title: "Estimate", Type: "numbers"},
		}},
		Groups: map[string][]domain.Group{"100": {{ID: "todo", Title: "To Do"}}},
		Items:  map[string]domain.Item{},
		nextID: 1000,
		FailOn: map[string]error{},
	}
	f.Items["500"] = domain.Item{ID: "500", Name: "Seed", BoardID: "100", Group: &domain.Group{ID: "todo", Title: "To Do"}}
	f.Order = []string{"500"}
	return f
}

func (f *FakePort) record(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Mutations = append(f.Mutations, op)
	return f.FailOn[op]
}

func (f *FakePort) id() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return fmt.Sprint(f.nextID)
}

// MutationCount returns how many mutations reached the port.
func (f *FakePort) MutationCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Mutations)
}

func (f *FakePort) Me(context.Context) (*domain.Me, error) {
	return &domain.Me{User: domain.User{ID: "1", Name: "Tester", Kind: "admin"}, IsAdmin: true, Account: &domain.Account{ID: "9", Name: "Acme", Slug: "acme"}}, nil
}
func (f *FakePort) APIStatus(context.Context) (domain.Complexity, domain.APIVersion, error) {
	return domain.Complexity{Before: 1000, After: 900, Query: 100, ResetInXSeconds: 30}, domain.APIVersion{Value: "2026-07", Kind: "current"}, nil
}
func (f *FakePort) ListWorkspacesPage(context.Context, int, int, string, string) ([]domain.Workspace, error) {
	return []domain.Workspace{{ID: "7", Name: "DevOps", Kind: "closed"}}, nil
}
func (f *FakePort) GetWorkspace(_ context.Context, id string) (*domain.Workspace, error) {
	return &domain.Workspace{ID: id, Name: "DevOps"}, nil
}
func (f *FakePort) CreateWorkspace(_ context.Context, name, kind, _ string) (*domain.Workspace, error) {
	return &domain.Workspace{ID: f.id(), Name: name, Kind: kind}, f.record("create_workspace")
}
func (f *FakePort) ListFolders(context.Context, string, int, int) ([]domain.Folder, error) {
	return nil, nil
}
func (f *FakePort) CreateFolder(_ context.Context, _, name string) (*domain.Folder, error) {
	return &domain.Folder{ID: f.id(), Name: name}, f.record("create_folder")
}
func (f *FakePort) ListBoardsPage(context.Context, monday.BoardQuery) ([]domain.Board, error) {
	return []domain.Board{{ID: "100", Name: "Ops", ItemsCount: 1}, {ID: "101", Name: "Subitems of Ops", ItemsCount: 4}}, nil
}
func (f *FakePort) GetBoard(_ context.Context, id string) (*domain.Board, error) {
	if _, ok := f.Columns[id]; !ok {
		return nil, &monday.NotFoundError{Resource: "board", ID: id}
	}
	return &domain.Board{ID: id, Name: "Ops", WorkspaceID: "7"}, nil
}
func (f *FakePort) GetBoardSchema(ctx context.Context, id string) (*monday.BoardSchema, error) {
	board, err := f.GetBoard(ctx, id)
	if err != nil {
		return nil, err
	}
	return &monday.BoardSchema{Board: *board, Columns: f.Columns[id], Groups: f.Groups[id]}, nil
}
func (f *FakePort) CreateBoard(_ context.Context, input monday.CreateBoardInput) (*domain.Board, error) {
	id := f.id()
	f.mu.Lock()
	f.Columns[id] = []domain.Column{{ID: "name", Title: "Name", Type: "name"}}
	f.Groups[id] = []domain.Group{{ID: "topics", Title: "Group Title"}}
	f.mu.Unlock()
	return &domain.Board{ID: id, Name: input.Name, WorkspaceID: input.WorkspaceID}, f.record("create_board")
}
func (f *FakePort) UpdateBoard(context.Context, string, string, string) error {
	return f.record("update_board")
}
func (f *FakePort) ArchiveBoard(_ context.Context, id string) (*domain.Board, error) {
	return &domain.Board{ID: id, State: "archived"}, f.record("archive_board")
}
func (f *FakePort) DuplicateBoard(context.Context, string, string, string, string, string, bool) (*domain.Board, error) {
	return &domain.Board{ID: f.id()}, f.record("duplicate_board")
}
func (f *FakePort) AddUsersToBoard(_ context.Context, _ string, ids []string, _ string) ([]domain.UserRef, error) {
	refs := make([]domain.UserRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, domain.UserRef{ID: id})
	}
	return refs, f.record("add_users_to_board")
}
func (f *FakePort) ListGroups(_ context.Context, boardID string) ([]domain.Group, error) {
	return f.Groups[boardID], nil
}
func (f *FakePort) CreateGroup(_ context.Context, boardID, name, _, relativeTo, method string) (*domain.Group, error) {
	group := domain.Group{ID: f.id(), Title: name}
	f.mu.Lock()
	groups := f.Groups[boardID]
	index := len(groups)
	if method == "after_at" {
		for i, g := range groups {
			if g.ID == relativeTo {
				index = i + 1
			}
		}
	}
	groups = append(groups[:index], append([]domain.Group{group}, groups[index:]...)...)
	f.Groups[boardID] = groups
	f.mu.Unlock()
	return &group, f.record("create_group")
}
func (f *FakePort) UpdateGroup(_ context.Context, boardID, groupID, attribute, value string) (*domain.Group, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, g := range f.Groups[boardID] {
		if g.ID == groupID && attribute == "title" {
			f.Groups[boardID][i].Title = value
			f.Mutations = append(f.Mutations, "update_group")
			return &f.Groups[boardID][i], nil
		}
	}
	return nil, &monday.NotFoundError{Resource: "group", ID: groupID}
}
func (f *FakePort) DuplicateGroup(context.Context, string, string, string, bool) (*domain.Group, error) {
	return &domain.Group{ID: f.id()}, f.record("duplicate_group")
}
func (f *FakePort) ArchiveGroup(_ context.Context, _, groupID string) (*domain.Group, error) {
	return &domain.Group{ID: groupID, Archived: true}, f.record("archive_group")
}
func (f *FakePort) ListColumns(_ context.Context, boardID string) ([]domain.Column, error) {
	columns, ok := f.Columns[boardID]
	if !ok {
		return nil, &monday.NotFoundError{Resource: "board", ID: boardID}
	}
	return columns, nil
}
func (f *FakePort) CreateColumn(_ context.Context, input monday.CreateColumnInput) (*domain.Column, error) {
	column := domain.Column{ID: input.ID, Title: input.Title, Type: input.Type}
	if input.Defaults != nil {
		if labels, ok := input.Defaults["labels"].(map[string]any); ok {
			list := []any{}
			for key, label := range labels {
				var id int
				fmt.Sscan(key, &id)
				list = append(list, map[string]any{"id": id, "label": label})
			}
			column.Settings = domain.JSONObject{"labels": list}
		}
	}
	f.mu.Lock()
	f.Columns[input.BoardID] = append(f.Columns[input.BoardID], column)
	f.mu.Unlock()
	return &column, f.record("create_column")
}
func (f *FakePort) ChangeColumnTitle(_ context.Context, _, columnID, title string) (*domain.Column, error) {
	return &domain.Column{ID: columnID, Title: title}, f.record("change_column_title")
}
func (f *FakePort) ChangeColumnDescription(_ context.Context, _, columnID, description string) (*domain.Column, error) {
	return &domain.Column{ID: columnID, Description: description}, f.record("change_column_description")
}
func (f *FakePort) ListItemsPage(_ context.Context, query monday.ItemPageQuery) (domain.ItemPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	start := 0
	if query.Cursor != "" {
		fmt.Sscan(query.Cursor, &start)
	}
	var page domain.ItemPage
	for index := start; index < len(f.Order) && len(page.Items) < query.Limit; index++ {
		page.Items = append(page.Items, f.Items[f.Order[index]])
		if index+1 < len(f.Order) && len(page.Items) == query.Limit {
			page.Cursor = fmt.Sprint(index + 1)
		}
	}
	return page, nil
}
func (f *FakePort) ItemsByColumnValues(context.Context, string, []monday.ColumnMatch, int, string) (domain.ItemPage, error) {
	return domain.ItemPage{}, nil
}
func (f *FakePort) GetItems(_ context.Context, ids []string) ([]domain.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []domain.Item
	for _, id := range ids {
		if item, ok := f.Items[id]; ok {
			items = append(items, item)
		}
	}
	return items, nil
}
func (f *FakePort) CreateItem(_ context.Context, boardID, groupID, name string, _ map[string]any) (*domain.Item, error) {
	item := domain.Item{ID: f.id(), Name: name, BoardID: boardID, Group: &domain.Group{ID: groupID}}
	f.mu.Lock()
	f.Items[item.ID] = item
	f.Order = append(f.Order, item.ID)
	f.mu.Unlock()
	return &item, f.record("create_item")
}
func (f *FakePort) CreateSubitem(_ context.Context, _, name string, _ map[string]any) (*domain.Item, error) {
	return &domain.Item{ID: f.id(), Name: name}, f.record("create_subitem")
}
func (f *FakePort) UpdateItemValues(_ context.Context, boardID, itemID string, _ map[string]any) (*domain.Item, error) {
	if err := f.record("update_item_values:" + itemID); err != nil {
		return nil, err
	}
	return &domain.Item{ID: itemID, BoardID: boardID}, nil
}
func (f *FakePort) MoveItem(_ context.Context, itemID, groupID string) (*domain.Item, error) {
	return &domain.Item{ID: itemID, Group: &domain.Group{ID: groupID}}, f.record("move_item")
}
func (f *FakePort) MoveItemToBoard(_ context.Context, itemID, boardID, _ string) (*domain.Item, error) {
	return &domain.Item{ID: itemID, BoardID: boardID}, f.record("move_item_to_board")
}
func (f *FakePort) DuplicateItem(_ context.Context, boardID, _ string, _ bool) (*domain.Item, error) {
	return &domain.Item{ID: f.id(), BoardID: boardID}, f.record("duplicate_item")
}
func (f *FakePort) ArchiveItem(_ context.Context, itemID string) (*domain.Item, error) {
	return &domain.Item{ID: itemID, State: "archived"}, f.record("archive_item")
}
func (f *FakePort) ListUsers(_ context.Context, query monday.UserQuery) ([]domain.User, error) {
	if len(query.IDs) > 0 && query.IDs[0] == "404" {
		return nil, nil
	}
	return []domain.User{{ID: "1", Name: "Tester"}}, nil
}
func (f *FakePort) ListTeams(context.Context, []string) ([]domain.Team, error) {
	return []domain.Team{{ID: "3", Name: "Ops"}}, nil
}
func (f *FakePort) ListItemUpdates(context.Context, string, int) ([]domain.Update, error) {
	return []domain.Update{{ID: "u1"}}, nil
}
func (f *FakePort) ListBoardUpdates(context.Context, string, int) ([]domain.Update, error) {
	return []domain.Update{{ID: "u1"}}, nil
}
func (f *FakePort) CreateUpdate(_ context.Context, itemID, body, _ string) (*domain.Update, error) {
	return &domain.Update{ID: f.id(), ItemID: itemID, Body: body}, f.record("create_update")
}
func (f *FakePort) LikeUpdate(context.Context, string) error { return f.record("like_update") }
func (f *FakePort) CreateNotification(context.Context, string, string, string, string) error {
	return f.record("create_notification")
}
func (f *FakePort) ListTags(context.Context, []string) ([]domain.Tag, error) {
	return []domain.Tag{{ID: "8", Name: "ops"}}, nil
}
func (f *FakePort) CreateOrGetTag(_ context.Context, _, name string) (*domain.Tag, error) {
	return &domain.Tag{ID: f.id(), Name: name}, f.record("create_or_get_tag")
}

var _ application.Port = (*FakePort)(nil)
