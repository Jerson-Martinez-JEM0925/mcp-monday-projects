package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/application/applicationtest"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// newScoped returns a service scoped to workspace "7" whose fake also holds a
// foreign board "200" (workspace "8") with item "600".
func newScoped(guard *application.WriteGuard) (*application.Service, *applicationtest.FakePort) {
	port := applicationtest.NewFakePort()
	port.Columns["200"] = port.Columns["100"]
	port.Groups["200"] = port.Groups["100"]
	port.BoardWorkspace["200"] = "8"
	port.Items["600"] = domain.Item{ID: "600", Name: "Foreign", BoardID: "200"}
	svc := application.NewService(port, application.Options{
		Guard: guard, WorkspaceScope: "7", ReportMaxItems: 3,
		Now: func() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) },
	})
	return svc, port
}

func wantScope(t *testing.T, name string, err error) {
	t.Helper()
	var scopeErr *application.ScopeError
	if !errors.As(err, &scopeErr) {
		t.Fatalf("%s: err = %v, want *ScopeError", name, err)
	}
}

func TestScopeRejectsForeignBoardsAndItemsForReadsAndWrites(t *testing.T) {
	svc, port := newScoped(nil)
	calls := map[string]func() error{
		"get_board":        func() error { _, err := svc.GetBoard(ctx, "200"); return err },
		"get_board_schema": func() error { _, err := svc.GetBoardSchema(ctx, "200"); return err },
		"list_groups":      func() error { _, err := svc.ListGroups(ctx, "200"); return err },
		"list_items":       func() error { _, err := svc.ListItems(ctx, monday.ItemPageQuery{BoardID: "200"}); return err },
		"get_item":         func() error { _, err := svc.GetItem(ctx, "600"); return err },
		"item_updates":     func() error { _, err := svc.ListItemUpdates(ctx, "600", 5); return err },
		"create_item":      func() error { _, err := svc.CreateItem(ctx, "200", "", "x", nil); return err },
		"update_values":    func() error { _, err := svc.UpdateItemValues(ctx, "200", "600", map[string]any{"est": 1}); return err },
		"archive_item":     func() error { _, err := svc.ArchiveItem(ctx, "600"); return err },
		"move_to_foreign":  func() error { _, err := svc.MoveItemToBoard(ctx, "500", "200", "todo"); return err },
		"create_update":    func() error { _, err := svc.CreateUpdate(ctx, "600", "hi"); return err },
		"notify_item":      func() error { return svc.Notify(ctx, "1", "600", "Project", "hi") },
		"create_group":     func() error { _, err := svc.CreateGroup(ctx, "200", "g", "", "", ""); return err },
		"archive_board":    func() error { _, err := svc.ArchiveBoard(ctx, "200", true); return err },
		"foreign_ws":       func() error { _, err := svc.GetWorkspace(ctx, "8"); return err },
		"list_foreign_ws": func() error {
			_, err := svc.ListBoards(ctx, monday.BoardQuery{WorkspaceIDs: []string{"8"}})
			return err
		},
		"board_in_foreign": func() error {
			_, err := svc.CreateBoard(ctx, monday.CreateBoardInput{Name: "b", WorkspaceID: "8"})
			return err
		},
		"folder_foreign": func() error { _, err := svc.CreateFolder(ctx, "8", "f"); return err },
	}
	for name, call := range calls {
		wantScope(t, name, call())
	}
	if port.MutationCount() != 0 {
		t.Fatalf("out-of-scope mutations reached the port: %v", port.Mutations)
	}
}

func TestScopeAllowsInScopeTargetsAndDefaultsWorkspace(t *testing.T) {
	svc, port := newScoped(nil)
	if _, err := svc.CreateItem(ctx, "100", "", "ok", nil); err != nil {
		t.Fatalf("in-scope create refused: %v", err)
	}
	if _, err := svc.GetItem(ctx, "500"); err != nil {
		t.Fatalf("in-scope read refused: %v", err)
	}
	workspace, err := svc.GetWorkspace(ctx, "")
	if err != nil || workspace.ID != "7" {
		t.Fatalf("default workspace = %+v, %v", workspace, err)
	}
	board, err := svc.CreateBoard(ctx, monday.CreateBoardInput{Name: "scoped"})
	if err != nil || board.WorkspaceID != "7" {
		t.Fatalf("board defaulted to %+v, %v", board, err)
	}
	// A board created through the scoped server is writable right away.
	if _, err := svc.CreateGroup(ctx, board.ID, "g", "", "", ""); err != nil {
		t.Fatalf("created board refused: %v", err)
	}
	if _, err := svc.CreateFolder(ctx, "", "f"); err != nil {
		t.Fatalf("folder with default workspace refused: %v", err)
	}
	list, err := svc.ListWorkspaces(ctx, 0, 1, "", "")
	if err != nil || len(list) != 1 || list[0].ID != "7" {
		t.Fatalf("list_workspaces = %+v, %v", list, err)
	}
	if port.MutationCount() != 4 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestScopeRefusesWorkspaceCreationAndUnverifiableNotifications(t *testing.T) {
	svc, port := newScoped(nil)
	if _, err := svc.CreateWorkspace(ctx, "new", "open", ""); err == nil {
		t.Fatal("workspace creation allowed under scope")
	}
	if err := svc.Notify(ctx, "1", "42", "Post", "hi"); err == nil {
		t.Fatal("Post notification allowed under scope")
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestScopeComposesWithReadOnly(t *testing.T) {
	svc, _ := newScoped(application.NewWriteGuard(true, nil, nil))
	if _, err := svc.CreateItem(ctx, "100", "", "x", nil); !errors.Is(err, application.ErrReadOnly) {
		t.Fatalf("err = %v, want ErrReadOnly", err)
	}
	if _, err := svc.GetBoard(ctx, "100"); err != nil {
		t.Fatalf("in-scope read refused in read-only mode: %v", err)
	}
}
