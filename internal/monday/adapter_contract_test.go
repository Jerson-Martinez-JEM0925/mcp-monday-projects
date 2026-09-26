package monday

import (
	"context"
	"testing"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
)

// adapterCase is one GraphQL method: the field the query must contain, a
// canned reply, the call, the variables it must send, and the variables it
// must omit (nulls are dropped by the transport).
type adapterCase struct {
	name   string
	field  string
	reply  string
	call   func(*Client) (any, error)
	vars   map[string]any
	absent []string
	check  func(any) bool
}

var bg = context.Background()

func adapterCases() []adapterCase {
	group := `{"id":"g1","title":"Todo"}`
	column := `{"id":"status","title":"Status","type":"status"}`
	return []adapterCase{
		{"list_workspaces", "workspaces", `{"data":{"workspaces":[{"id":"7","name":"DevOps"}]}}`,
			func(c *Client) (any, error) { return c.ListWorkspacesPage(bg, 5, 0, "closed", "") },
			map[string]any{"limit": float64(5), "kind": "closed"}, []string{"page", "state"},
			func(v any) bool { return len(v.([]Workspace)) == 1 }},
		{"list_workspaces_default", "workspaces", `{"data":{"workspaces":[]}}`,
			func(c *Client) (any, error) { return c.ListWorkspaces(bg, 3) },
			map[string]any{"limit": float64(3)}, nil, nil},
		{"get_workspace", "workspaces", `{"data":{"workspaces":[{"id":"7","name":"DevOps"}]}}`,
			func(c *Client) (any, error) { return c.GetWorkspace(bg, "7") },
			map[string]any{"id": "7"}, nil, func(v any) bool { return v.(*Workspace).Name == "DevOps" }},
		{"create_workspace", "create_workspace", `{"data":{"create_workspace":{"id":"8","name":"New"}}}`,
			func(c *Client) (any, error) { return c.CreateWorkspace(bg, "New", "closed", "") },
			map[string]any{"name": "New", "kind": "closed"}, []string{"description"},
			func(v any) bool { return v.(*Workspace).ID == "8" }},
		{"list_folders", "folders", `{"data":{"folders":[{"id":"3","name":"Ops"}]}}`,
			func(c *Client) (any, error) { return c.ListFolders(bg, "7", 10, 2) },
			map[string]any{"limit": float64(10), "page": float64(2)}, nil,
			func(v any) bool { return len(v.([]Folder)) == 1 }},
		{"create_folder", "create_folder", `{"data":{"create_folder":{"id":"3","name":"Ops"}}}`,
			func(c *Client) (any, error) { return c.CreateFolder(bg, "7", "Ops") },
			map[string]any{"workspaceID": "7", "name": "Ops"}, nil, nil},
		{"create_board", "create_board", `{"data":{"create_board":{"id":"100","name":"B"}}}`,
			func(c *Client) (any, error) {
				return c.CreateBoard(bg, CreateBoardInput{Name: "B", Kind: "private", WorkspaceID: "7", Empty: true})
			},
			map[string]any{"name": "B", "kind": "private", "workspaceID": "7", "empty": true},
			[]string{"folderID", "description", "ownerIDs", "subscriberIDs", "templateID"},
			func(v any) bool { return v.(*Board).ID == "100" }},
		{"update_board", "update_board", `{"data":{"update_board":"{\"success\":true}"}}`,
			func(c *Client) (any, error) { return nil, c.UpdateBoard(bg, "100", "description", "d") },
			map[string]any{"id": "100", "attribute": "description", "value": "d"}, nil, nil},
		{"archive_board", "archive_board", `{"data":{"archive_board":{"id":"100","state":"archived"}}}`,
			func(c *Client) (any, error) { return c.ArchiveBoard(bg, "100") },
			map[string]any{"id": "100"}, nil, func(v any) bool { return v.(*Board).State == "archived" }},
		{"duplicate_board", "duplicate_board", `{"data":{"duplicate_board":{"board":{"id":"101"}}}}`,
			func(c *Client) (any, error) {
				return c.DuplicateBoard(bg, "100", "duplicate_board_with_structure", "Copy", "", "", true)
			},
			map[string]any{"id": "100", "type": "duplicate_board_with_structure", "name": "Copy", "keepSubscribers": true},
			[]string{"workspaceID", "folderID"}, func(v any) bool { return v.(*Board).ID == "101" }},
		{"add_users_to_board", "add_users_to_board", `{"data":{"add_users_to_board":[{"id":"1"}]}}`,
			func(c *Client) (any, error) { return c.AddUsersToBoard(bg, "100", []string{"1"}, "") },
			map[string]any{"id": "100"}, []string{"kind"}, nil},
		{"list_groups", "groups", `{"data":{"boards":[{"groups":[` + group + `]}]}}`,
			func(c *Client) (any, error) { return c.ListGroups(bg, "100") },
			map[string]any{"boardID": "100"}, nil, func(v any) bool { return len(v.([]Group)) == 1 }},
		{"create_group", "create_group", `{"data":{"create_group":` + group + `}}`,
			func(c *Client) (any, error) { return c.CreateGroup(bg, "100", "Todo", "", "", "") },
			map[string]any{"boardID": "100", "name": "Todo"}, []string{"color", "relativeTo", "method"}, nil},
		{"update_group", "update_group", `{"data":{"update_group":` + group + `}}`,
			func(c *Client) (any, error) { return c.UpdateGroup(bg, "100", "g1", "title", "Doing") },
			map[string]any{"boardID": "100", "groupID": "g1", "attribute": "title", "value": "Doing"}, nil, nil},
		{"duplicate_group", "duplicate_group", `{"data":{"duplicate_group":` + group + `}}`,
			func(c *Client) (any, error) { return c.DuplicateGroup(bg, "100", "g1", "", true) },
			map[string]any{"boardID": "100", "groupID": "g1", "addToTop": true}, []string{"title"}, nil},
		{"archive_group", "archive_group", `{"data":{"archive_group":` + group + `}}`,
			func(c *Client) (any, error) { return c.ArchiveGroup(bg, "100", "g1") },
			map[string]any{"boardID": "100", "groupID": "g1"}, nil, nil},
		{"change_column_title", "change_column_title", `{"data":{"change_column_title":` + column + `}}`,
			func(c *Client) (any, error) { return c.ChangeColumnTitle(bg, "100", "status", "State") },
			map[string]any{"boardID": "100", "columnID": "status", "title": "State"}, nil, nil},
		{"change_column_metadata", "change_column_metadata", `{"data":{"change_column_metadata":` + column + `}}`,
			func(c *Client) (any, error) { return c.ChangeColumnDescription(bg, "100", "status", "d") },
			map[string]any{"boardID": "100", "columnID": "status", "property": "description", "value": "d"}, nil, nil},
		{"items_by_column_values", "items_page_by_column_values", `{"data":{"items_page_by_column_values":{"cursor":"c2","items":[` + itemJSON + `]}}}`,
			func(c *Client) (any, error) {
				return c.ItemsByColumnValues(bg, "7", []ColumnMatch{{ColumnID: "status", Values: []string{"Done"}}}, 10, "")
			},
			map[string]any{"boardID": "7", "limit": float64(10)}, []string{"cursor"},
			func(v any) bool { page := v.(domain.ItemPage); return page.Cursor == "c2" && len(page.Items) == 1 }},
		{"items_by_column_values_cursor", "items_page_by_column_values", `{"data":{"items_page_by_column_values":{"cursor":null,"items":[]}}}`,
			func(c *Client) (any, error) {
				return c.ItemsByColumnValues(bg, "7", []ColumnMatch{{ColumnID: "status", Values: []string{"Done"}}}, 10, "c2")
			},
			map[string]any{"cursor": "c2"}, []string{"columns"}, nil},
		{"create_subitem", "create_subitem", `{"data":{"create_subitem":` + itemJSON + `}}`,
			func(c *Client) (any, error) { return c.CreateSubitem(bg, "11", "Child", map[string]any{"text": "x"}) },
			map[string]any{"parentID": "11", "itemName": "Child", "columnValues": `{"text":"x"}`, "createLabels": false}, nil, nil},
		{"move_item_to_group", "move_item_to_group", `{"data":{"move_item_to_group":` + itemJSON + `}}`,
			func(c *Client) (any, error) { return c.MoveItem(bg, "11", "g1") },
			map[string]any{"itemID": "11", "groupID": "g1"}, nil, nil},
		{"move_item_to_board", "move_item_to_board", `{"data":{"move_item_to_board":` + itemJSON + `}}`,
			func(c *Client) (any, error) { return c.MoveItemToBoard(bg, "11", "8", "g1") },
			map[string]any{"itemID": "11", "boardID": "8", "groupID": "g1"}, nil, nil},
		{"duplicate_item", "duplicate_item", `{"data":{"duplicate_item":` + itemJSON + `}}`,
			func(c *Client) (any, error) { return c.DuplicateItem(bg, "7", "11", true) },
			map[string]any{"boardID": "7", "itemID": "11", "withUpdates": true}, nil, nil},
		{"archive_item", "archive_item", `{"data":{"archive_item":` + itemJSON + `}}`,
			func(c *Client) (any, error) { return c.ArchiveItem(bg, "11") },
			map[string]any{"itemID": "11"}, nil, nil},
		{"list_teams", "teams", `{"data":{"teams":[{"id":"1","name":"Ops"}]}}`,
			func(c *Client) (any, error) { return c.ListTeams(bg, nil) },
			nil, []string{"ids"}, nil},
		{"board_updates", "updates", `{"data":{"boards":[{"updates":[{"id":"u1"}]}]}}`,
			func(c *Client) (any, error) { return c.ListBoardUpdates(bg, "7", 5) },
			map[string]any{"boardID": "7", "limit": float64(5)}, nil, nil},
		{"like_update", "like_update", `{"data":{"like_update":{"id":"u1"}}}`,
			func(c *Client) (any, error) { return nil, c.LikeUpdate(bg, "u1") },
			map[string]any{"updateID": "u1"}, nil, nil},
		{"create_notification", "create_notification", `{"data":{"create_notification":{"text":"hi"}}}`,
			func(c *Client) (any, error) { return nil, c.CreateNotification(bg, "1", "11", "Project", "hi") },
			map[string]any{"userID": "1", "targetID": "11", "targetType": "Project", "text": "hi"}, nil, nil},
		{"list_tags", "tags", `{"data":{"tags":[{"id":"5","name":"ops"}]}}`,
			func(c *Client) (any, error) { return c.ListTags(bg, []string{"5"}) },
			nil, nil, nil},
		{"create_or_get_tag", "create_or_get_tag", `{"data":{"create_or_get_tag":{"id":"5","name":"ops"}}}`,
			func(c *Client) (any, error) { return c.CreateOrGetTag(bg, "", "ops") },
			map[string]any{"name": "ops"}, []string{"boardID"}, nil},
	}
}

func TestAdapterMethodsSendExactVariablesAndMapReplies(t *testing.T) {
	for _, tc := range adapterCases() {
		t.Run(tc.name, func(t *testing.T) {
			client, c := newContractClient(t, tc.field, tc.reply)
			out, err := tc.call(client)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			for key, want := range tc.vars {
				if got := c.variables[key]; got != want {
					t.Errorf("variable %s = %#v, want %#v (all: %v)", key, got, want, c.variables)
				}
			}
			for _, key := range tc.absent {
				if _, present := c.variables[key]; present {
					t.Errorf("variable %s must be omitted when empty (all: %v)", key, c.variables)
				}
			}
			if tc.check != nil && !tc.check(out) {
				t.Errorf("unexpected result %+v", out)
			}
		})
	}
}

func TestAdapterNotFoundPaths(t *testing.T) {
	cases := map[string]struct {
		field, reply string
		call         func(*Client) error
	}{
		"workspace": {"workspaces", `{"data":{"workspaces":[]}}`, func(c *Client) error { _, err := c.GetWorkspace(bg, "1"); return err }},
		"groups":    {"groups", `{"data":{"boards":[]}}`, func(c *Client) error { _, err := c.ListGroups(bg, "1"); return err }},
		"updates":   {"updates", `{"data":{"boards":[]}}`, func(c *Client) error { _, err := c.ListBoardUpdates(bg, "1", 5); return err }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client, _ := newContractClient(t, tc.field, tc.reply)
			if err := tc.call(client); !IsNotFound(err) {
				t.Fatalf("err = %v, want not found", err)
			}
		})
	}
}

func TestAdapterPropagatesGraphQLErrors(t *testing.T) {
	reply := `{"errors":[{"message":"boom","extensions":{"code":"InvalidBoardIdException"}}]}`
	for _, tc := range adapterCases() {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := newContractClient(t, "", reply)
			if _, err := tc.call(client); err == nil {
				t.Fatal("GraphQL error was swallowed")
			}
		})
	}
}
