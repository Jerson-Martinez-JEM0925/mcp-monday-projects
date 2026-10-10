package application_test

import (
	"errors"
	"testing"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/application/applicationtest"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// useCases exercises every service method with valid input. Run against both
// an unscoped and a scoped service it covers the use cases and every
// ScopedPort pass-through for in-scope targets.
func useCases(svc *application.Service) map[string]func() error {
	values := map[string]any{"est": 2}
	return map[string]func() error{
		"me":              func() error { _, err := svc.Me(ctx); return err },
		"api_status":      func() error { _, _, err := svc.APIStatus(ctx); return err },
		"list_workspaces": func() error { _, err := svc.ListWorkspaces(ctx, 5, 1, "closed", "active"); return err },
		"list_folders":    func() error { _, err := svc.ListFolders(ctx, "7", 5, 1); return err },
		"list_boards": func() error {
			_, err := svc.ListBoards(ctx, monday.BoardQuery{WorkspaceIDs: []string{"7"}, State: "active"})
			return err
		},
		"update_board":    func() error { return svc.UpdateBoard(ctx, "100", "description", "d") },
		"duplicate_board": func() error { _, err := svc.DuplicateBoard(ctx, "100", "", "Copy", "7", "", false); return err },
		"add_subscribers": func() error { _, err := svc.AddBoardSubscribers(ctx, "100", []string{"1"}, "owner"); return err },
		"list_groups":     func() error { _, err := svc.ListGroups(ctx, "100"); return err },
		"create_group":    func() error { _, err := svc.CreateGroup(ctx, "100", "Next", "", "", ""); return err },
		"update_group":    func() error { _, err := svc.UpdateGroup(ctx, "100", "todo", "title", "Doing"); return err },
		"duplicate_group": func() error { _, err := svc.DuplicateGroup(ctx, "100", "todo", "Copy", true); return err },
		"archive_group":   func() error { _, err := svc.ArchiveGroup(ctx, "100", "todo", true); return err },
		"list_columns":    func() error { _, err := svc.ListColumns(ctx, "100"); return err },
		"create_column": func() error {
			_, err := svc.CreateColumn(ctx, "100", "", "Risk", "status", "d", "", []string{"Low"})
			return err
		},
		"change_column": func() error { _, err := svc.ChangeColumn(ctx, "100", "est", "Points", "Story points"); return err },
		"list_items":    func() error { _, err := svc.ListItems(ctx, monday.ItemPageQuery{BoardID: "100", Limit: 1}); return err },
		"search_items":  func() error { _, err := svc.SearchItems(ctx, "100", "Seed", nil, 5); return err },
		"find_by_values": func() error {
			_, err := svc.FindItemsByColumnValues(ctx, "100", []monday.ColumnMatch{{ColumnID: "status", Values: []string{"Done"}}}, 5, "")
			return err
		},
		"create_subitem": func() error { _, err := svc.CreateSubitem(ctx, "500", "child", nil); return err },
		"update_values":  func() error { _, err := svc.UpdateItemValues(ctx, "100", "500", values); return err },
		"rename_item":    func() error { _, err := svc.RenameItem(ctx, "100", "500", "Renamed"); return err },
		"set_detected":   func() error { _, _, err := svc.SetDetectedColumn(ctx, "100", "500", "status", "", "Done"); return err },
		"move_item":      func() error { _, err := svc.MoveItem(ctx, "500", "todo"); return err },
		"move_to_board":  func() error { _, err := svc.MoveItemToBoard(ctx, "500", "100", "todo"); return err },
		"duplicate_item": func() error { _, err := svc.DuplicateItem(ctx, "100", "500", true); return err },
		"archive_item":   func() error { _, err := svc.ArchiveItem(ctx, "500"); return err },
		"list_users":     func() error { _, err := svc.ListUsers(ctx, monday.UserQuery{Limit: 5}); return err },
		"list_teams":     func() error { _, err := svc.ListTeams(ctx, []string{"3"}); return err },
		"item_updates":   func() error { _, err := svc.ListItemUpdates(ctx, "500", 5); return err },
		"board_updates":  func() error { _, err := svc.ListBoardUpdates(ctx, "100", 5); return err },
		"create_update":  func() error { _, err := svc.CreateUpdate(ctx, "500", "hi"); return err },
		"reply_update":   func() error { _, err := svc.ReplyToUpdate(ctx, "500", "77", "ok"); return err },
		"like_update":    func() error { return svc.LikeUpdate(ctx, "500", "77") },

		"edit_update":        func() error { _, err := svc.EditUpdate(ctx, "500", "77", "## edited"); return err },
		"notify":             func() error { return svc.Notify(ctx, "1", "500", "Project", "hi") },
		"list_tags":          func() error { _, err := svc.ListTags(ctx, []string{"8"}); return err },
		"board_tag":          func() error { _, err := svc.CreateOrGetTag(ctx, "100", "ops"); return err },
		"account_tag":        func() error { _, err := svc.CreateOrGetTag(ctx, "", "ops"); return err },
		"workspace_overview": func() error { _, err := svc.BuildWorkspaceOverview(ctx, "7"); return err },
		"snapshot":           func() error { _, err := svc.Snapshot(ctx, "100", 10); return err },
	}
}

func TestEveryUseCaseSucceedsUnscopedAndScoped(t *testing.T) {
	for name, options := range map[string]application.Options{
		"unscoped":    {},
		"scoped":      {WorkspaceScope: "7"},
		"allowlisted": {Guard: application.NewWriteGuard(false, []string{"100"}, []string{"7"})},
	} {
		svc, _ := newWith(options)
		for useCase, call := range useCases(svc) {
			if err := call(); err != nil {
				t.Errorf("%s/%s: %v", name, useCase, err)
			}
		}
	}
}

func newWith(options application.Options) (*application.Service, *applicationtest.FakePort) {
	port := applicationtest.NewFakePort()
	return application.NewService(port, options), port
}

func TestScopedPortRefusesForeignTargetsForEveryWrite(t *testing.T) {
	svc, port := newScoped(nil)
	port.Items["601"] = domain.Item{ID: "601", Name: "Child", BoardID: "200"}
	calls := map[string]func() error{
		"update_board":    func() error { return svc.UpdateBoard(ctx, "200", "name", "x") },
		"duplicate_board": func() error { _, err := svc.DuplicateBoard(ctx, "200", "", "x", "", "", false); return err },
		"subscribers":     func() error { _, err := svc.AddBoardSubscribers(ctx, "200", []string{"1"}, ""); return err },
		"update_group":    func() error { _, err := svc.UpdateGroup(ctx, "200", "todo", "title", "x"); return err },
		"duplicate_group": func() error { _, err := svc.DuplicateGroup(ctx, "200", "todo", "", false); return err },
		"archive_group":   func() error { _, err := svc.ArchiveGroup(ctx, "200", "todo", true); return err },
		"create_column":   func() error { _, err := svc.CreateColumn(ctx, "200", "", "x", "text", "", "", nil); return err },
		"change_column":   func() error { _, err := svc.ChangeColumn(ctx, "200", "est", "x", "y"); return err },
		"find_by_values": func() error {
			_, err := svc.FindItemsByColumnValues(ctx, "200", []monday.ColumnMatch{{ColumnID: "status", Values: []string{"x"}}}, 5, "")
			return err
		},
		"create_subitem": func() error { _, err := svc.CreateSubitem(ctx, "601", "x", nil); return err },
		"move_item":      func() error { _, err := svc.MoveItem(ctx, "601", "todo"); return err },
		"duplicate_item": func() error { _, err := svc.DuplicateItem(ctx, "200", "601", false); return err },
		"board_updates":  func() error { _, err := svc.ListBoardUpdates(ctx, "200", 5); return err },
		"board_tag":      func() error { _, err := svc.CreateOrGetTag(ctx, "200", "x"); return err },
		"list_columns":   func() error { _, err := svc.ListColumns(ctx, "200"); return err },
	}
	for name, call := range calls {
		wantScope(t, name, call())
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestUseCaseValidationRejectsBeforeThePort(t *testing.T) {
	svc, port := newService(nil)
	calls := map[string]func() error{
		"workspaces_kind":  func() error { _, err := svc.ListWorkspaces(ctx, 5, 1, "weird", ""); return err },
		"workspaces_limit": func() error { _, err := svc.ListWorkspaces(ctx, 500, 1, "", ""); return err },
		"folders_id":       func() error { _, err := svc.ListFolders(ctx, "abc", 5, 1); return err },
		"boards_order":     func() error { _, err := svc.ListBoards(ctx, monday.BoardQuery{OrderBy: "name"}); return err },
		"boards_ws": func() error {
			_, err := svc.ListBoards(ctx, monday.BoardQuery{WorkspaceIDs: []string{"x"}})
			return err
		},
		"update_board":     func() error { return svc.UpdateBoard(ctx, "100", "name", " ") },
		"duplicate_type":   func() error { _, err := svc.DuplicateBoard(ctx, "100", "all", "", "", "", false); return err },
		"subscribers":      func() error { _, err := svc.AddBoardSubscribers(ctx, "100", nil, ""); return err },
		"update_group":     func() error { _, err := svc.UpdateGroup(ctx, "100", "todo", "size", "x"); return err },
		"duplicate_group":  func() error { _, err := svc.DuplicateGroup(ctx, "100", "", "", false); return err },
		"archive_group":    func() error { _, err := svc.ArchiveGroup(ctx, "100", "todo", false); return err },
		"create_column":    func() error { _, err := svc.CreateColumn(ctx, "100", "", "", "status", "", "", nil); return err },
		"change_column":    func() error { _, err := svc.ChangeColumn(ctx, "100", "est", "", ""); return err },
		"search_empty":     func() error { _, err := svc.SearchItems(ctx, "100", "", nil, 5); return err },
		"find_empty":       func() error { _, err := svc.FindItemsByColumnValues(ctx, "100", nil, 5, ""); return err },
		"subitem_name":     func() error { _, err := svc.CreateSubitem(ctx, "500", "", nil); return err },
		"subitem_values":   func() error { _, err := svc.CreateSubitem(ctx, "500", "x", map[string]any{"est": 1}); return err },
		"update_empty":     func() error { _, err := svc.UpdateItemValues(ctx, "100", "500", nil); return err },
		"rename_empty":     func() error { _, err := svc.RenameItem(ctx, "100", "500", ""); return err },
		"detect_missing":   func() error { _, _, err := svc.SetDetectedColumn(ctx, "100", "500", "timeline", "", "x"); return err },
		"move_group":       func() error { _, err := svc.MoveItem(ctx, "500", ""); return err },
		"move_board":       func() error { _, err := svc.MoveItemToBoard(ctx, "500", "x", "todo"); return err },
		"duplicate_item":   func() error { _, err := svc.DuplicateItem(ctx, "", "500", false); return err },
		"users_limit":      func() error { _, err := svc.ListUsers(ctx, monday.UserQuery{Limit: 1000}); return err },
		"users_ids":        func() error { _, err := svc.ListUsers(ctx, monday.UserQuery{IDs: []string{"x"}}); return err },
		"teams_ids":        func() error { _, err := svc.ListTeams(ctx, []string{"x"}); return err },
		"board_updates":    func() error { _, err := svc.ListBoardUpdates(ctx, "x", 5); return err },
		"reply_body":       func() error { _, err := svc.ReplyToUpdate(ctx, "500", "77", ""); return err },
		"reply_id":         func() error { _, err := svc.ReplyToUpdate(ctx, "500", "x", "ok"); return err },
		"like_id":          func() error { return svc.LikeUpdate(ctx, "500", "") },
		"notify_type":      func() error { return svc.Notify(ctx, "1", "500", "Board", "hi") },
		"tags_ids":         func() error { _, err := svc.ListTags(ctx, []string{"x"}); return err },
		"tag_name":         func() error { _, err := svc.CreateOrGetTag(ctx, "", ""); return err },
		"tag_board":        func() error { _, err := svc.CreateOrGetTag(ctx, "x", "ops"); return err },
		"workspace_create": func() error { _, err := svc.CreateWorkspace(ctx, "", "", ""); return err },
	}
	for name, call := range calls {
		var input *application.InputError
		if err := call(); !errors.As(err, &input) {
			t.Errorf("%s: err = %v, want *InputError", name, err)
		}
	}
	if port.MutationCount() != 0 {
		t.Fatalf("invalid calls mutated: %v", port.Mutations)
	}
}

func TestServiceAccessors(t *testing.T) {
	svc, _ := newWith(application.Options{WorkspaceScope: "7", AllowDelete: true})
	if svc.WorkspaceScope() != "7" || !svc.AllowsDelete() || svc.Guard().ReadOnly() || svc.Now().IsZero() {
		t.Fatal("accessors mismatch")
	}
	scoped := application.NewScopedPort(nil, "7")
	if scoped.Workspace() != "7" {
		t.Fatal("scoped workspace")
	}
	if _, err := scoped.CreateWorkspace(ctx, "x", "open", ""); err == nil {
		t.Fatal("scoped CreateWorkspace allowed")
	}
}
