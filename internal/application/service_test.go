package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/application/applicationtest"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

var ctx = context.Background()

func newService(guard *application.WriteGuard) (*application.Service, *applicationtest.FakePort) {
	port := applicationtest.NewFakePort()
	return application.NewService(port, application.Options{Guard: guard, Now: func() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) }, ReportMaxItems: 3}), port
}

func TestReadOnlyRejectsEveryMutationBeforeThePort(t *testing.T) {
	svc, port := newService(application.NewWriteGuard(true, nil, nil))
	calls := []func() error{
		func() error { _, err := svc.CreateItem(ctx, "100", "", "x", nil); return err },
		func() error { _, err := svc.UpdateItemValues(ctx, "100", "500", map[string]any{"est": 1}); return err },
		func() error { _, err := svc.ArchiveItem(ctx, "500"); return err },
		func() error { _, err := svc.MoveItem(ctx, "500", "todo"); return err },
		func() error { _, err := svc.CreateGroup(ctx, "100", "g", "", "", ""); return err },
		func() error { _, err := svc.CreateBoard(ctx, monday.CreateBoardInput{Name: "b"}); return err },
		func() error { _, err := svc.CreateUpdate(ctx, "500", "hi"); return err },
		func() error { return svc.Notify(ctx, "1", "500", "Project", "hi") },
		func() error { _, err := svc.CreateOrGetTag(ctx, "", "t"); return err },
		func() error { _, err := svc.CreateWorkspace(ctx, "w", "open", ""); return err },
	}
	for index, call := range calls {
		if err := call(); !errors.Is(err, application.ErrReadOnly) {
			t.Fatalf("call %d: err = %v, want ErrReadOnly", index, err)
		}
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations reached the port: %v", port.Mutations)
	}
}

func TestAllowlistGuardsBoardsItemsAndWorkspaces(t *testing.T) {
	svc, port := newService(application.NewWriteGuard(false, []string{"100"}, []string{"7"}))
	if _, err := svc.CreateItem(ctx, "100", "", "ok", nil); err != nil {
		t.Fatalf("allowlisted board refused: %v", err)
	}
	var guardErr *application.GuardError
	if _, err := svc.CreateGroup(ctx, "999", "g", "", "", ""); !errors.As(err, &guardErr) || !strings.Contains(err.Error(), "MONDAY_WRITE_BOARD_ALLOWLIST") {
		t.Fatalf("foreign board err = %v", err)
	}
	// Items are resolved to their board before the check.
	port.Items["900"] = domain.Item{ID: "900", BoardID: "999"}
	if _, err := svc.ArchiveItem(ctx, "900"); !errors.As(err, &guardErr) {
		t.Fatalf("foreign item err = %v", err)
	}
	if _, err := svc.CreateBoard(ctx, monday.CreateBoardInput{Name: "x", WorkspaceID: "8"}); !errors.As(err, &guardErr) {
		t.Fatalf("foreign workspace err = %v", err)
	}
	if _, err := svc.CreateWorkspace(ctx, "w", "", ""); err == nil {
		t.Fatal("workspace creation must be refused under an allowlist")
	}
	board, err := svc.CreateBoard(ctx, monday.CreateBoardInput{Name: "x", WorkspaceID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateGroup(ctx, board.ID, "g", "", "", ""); err != nil {
		t.Fatalf("board created in-session must be writable: %v", err)
	}
	policy := svc.Guard().Describe()
	if created := policy["session_created_board"].([]string); len(created) != 1 || created[0] != board.ID {
		t.Fatalf("policy = %v", policy)
	}
}

func TestInputValidationHappensBeforeAPICalls(t *testing.T) {
	svc, port := newService(nil)
	cases := map[string]error{}
	_, cases["non numeric board"] = svc.GetBoard(ctx, "abc")
	_, cases["empty name"] = svc.CreateItem(ctx, "100", "", " ", nil)
	_, cases["limit too high"] = svc.ListWorkspaces(ctx, 500, 0, "", "")
	_, cases["bad kind"] = svc.ListBoards(ctx, monday.BoardQuery{Kind: "secret"})
	_, cases["bad operator"] = svc.ListItems(ctx, monday.ItemPageQuery{BoardID: "100", Filter: &domain.ItemFilter{Rules: []domain.ItemFilterRule{{ColumnID: "x", Operator: "like"}}}})
	_, cases["search needs input"] = svc.SearchItems(ctx, "100", "", nil, 0)
	_, cases["archive board needs confirm"] = svc.ArchiveBoard(ctx, "100", false)
	_, cases["archive group needs confirm"] = svc.ArchiveGroup(ctx, "100", "todo", false)
	_, cases["labels on text column"] = svc.CreateColumn(ctx, "100", "", "t", "text", "", "", []string{"a"})
	_, cases["bad column id"] = svc.CreateColumn(ctx, "100", "Bad-ID", "t", "text", "", "", nil)
	_, cases["too many ids"] = svc.GetItems(ctx, make([]string, 101))
	_, cases["body too long"] = svc.CreateUpdate(ctx, "500", strings.Repeat("x", 20001))
	cases["notify bad type"] = svc.Notify(ctx, "1", "500", "Email", "x")
	for name, err := range cases {
		var inputErr *application.InputError
		if !errors.As(err, &inputErr) {
			t.Errorf("%s: err = %v, want InputError", name, err)
		}
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestWritesValidateColumnValuesFirst(t *testing.T) {
	svc, port := newService(nil)
	_, err := svc.CreateItem(ctx, "100", "todo", "x", map[string]any{"status": "Nope"})
	var verr *domain.ValidationError
	if !errors.As(err, &verr) || port.MutationCount() != 0 {
		t.Fatalf("err = %v mutations = %v", err, port.Mutations)
	}
	if _, err := svc.CreateItem(ctx, "100", "todo", "x", map[string]any{"status": "done", "due": "2026-10-01"}); err != nil {
		t.Fatal(err)
	}
	item, column, err := svc.SetDetectedColumn(ctx, "100", "500", "status", "", "Stuck")
	if err != nil || column != "status" || item.ID != "500" {
		t.Fatalf("set status: %v %s %v", item, column, err)
	}
	if _, column, _ := svc.SetDetectedColumn(ctx, "100", "500", "people", "", []any{"1"}); column != "owner" {
		t.Fatalf("people column = %s", column)
	}
	plan, err := svc.PlanValues(ctx, "100", map[string]any{"est": "x"})
	if err != nil || plan.Valid {
		t.Fatalf("plan = %+v err = %v", plan, err)
	}
}

func TestBulkUpdateIsDryRunAndAllOrNothing(t *testing.T) {
	svc, port := newService(nil)
	rows := []application.BulkUpdate{{ItemID: "500", ColumnValues: map[string]any{"est": 3}}, {ItemID: "501", ColumnValues: map[string]any{"est": 5}}}
	result, err := svc.BulkUpdateItems(ctx, "100", rows, true)
	if err != nil || !result.DryRun || result.Planned != 2 || port.MutationCount() != 0 {
		t.Fatalf("dry run = %+v err=%v mutations=%v", result, err, port.Mutations)
	}
	bad := append(rows, application.BulkUpdate{ItemID: "502", ColumnValues: map[string]any{"status": "Nope"}})
	result, _ = svc.BulkUpdateItems(ctx, "100", bad, false)
	if result.Succeeded != 0 || result.Failed != 1 || port.MutationCount() != 0 {
		t.Fatalf("invalid batch wrote: %+v %v", result, port.Mutations)
	}
	port.FailOn["update_item_values:501"] = errors.New("boom")
	result, _ = svc.BulkUpdateItems(ctx, "100", rows, false)
	if result.Succeeded != 1 || result.Failed != 1 || result.Rows[1].Error != "boom" {
		t.Fatalf("partial failure = %+v", result)
	}
	if _, err := svc.BulkUpdateItems(ctx, "100", make([]application.BulkUpdate, 51), true); err == nil {
		t.Fatal("more than 50 rows must be rejected")
	}
}

func TestBulkItemActions(t *testing.T) {
	svc, port := newService(application.NewWriteGuard(false, []string{"100"}, nil))
	port.Items["900"] = domain.Item{ID: "900", BoardID: "999"}
	result, err := svc.BulkItemAction(ctx, "archive", []string{"500", "900", "404"}, "", true)
	if err != nil || result.Failed != 1 || port.MutationCount() != 0 {
		t.Fatalf("dry run = %+v err=%v", result, err)
	}
	result, _ = svc.BulkItemAction(ctx, "archive", []string{"500", "900"}, "", false)
	if result.Succeeded != 1 || result.Failed != 1 || result.Rows[0].Status != "archived" || !strings.Contains(result.Rows[1].Error, "allowlist") {
		t.Fatalf("archive = %+v", result)
	}
	if _, err := svc.BulkItemAction(ctx, "move", []string{"500"}, "", true); err == nil {
		t.Fatal("move without group must fail")
	}
	if _, err := svc.BulkItemAction(ctx, "delete", []string{"500"}, "", true); err == nil {
		t.Fatal("delete is not a supported action")
	}
}

func TestProvisionBoardBuildsTemplateInOrder(t *testing.T) {
	svc, port := newService(application.NewWriteGuard(false, []string{"1"}, []string{"7"}))
	result, err := svc.ProvisionBoard(ctx, "devops", "", "7", "private", "", []domain.TemplateItem{
		{Group: "In Progress", Name: "Pipeline", Values: map[string]any{"status": "Working on it", "estimate": 5}},
		{Group: "Backlog", Name: "Invalid", Values: map[string]any{"status": "Nope"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	titles := []string{}
	for _, group := range port.Groups[result.Board.ID] {
		titles = append(titles, group.Title)
	}
	if strings.Join(titles, ",") != "Backlog,In Progress,Code Review,Done" {
		t.Fatalf("group order = %v", titles)
	}
	if len(result.Columns) != 8 || len(result.Items) != 1 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "Invalid") {
		t.Fatalf("result = %+v", result)
	}
	if _, err := svc.ProvisionBoard(ctx, "nope", "", "7", "", "", nil); err == nil {
		t.Fatal("unknown template must fail")
	}
}

func TestSnapshotPagesAndTruncates(t *testing.T) {
	svc, port := newService(nil)
	for _, id := range []string{"501", "502", "503", "504"} {
		port.Items[id] = domain.Item{ID: id, BoardID: "100"}
		port.Order = append(port.Order, id)
	}
	snap, err := svc.Snapshot(ctx, "100", 0)
	if err != nil || len(snap.Items) != 3 || !snap.Truncated {
		t.Fatalf("snapshot = %d items truncated=%v err=%v", len(snap.Items), snap.Truncated, err)
	}
	overview, err := svc.BuildWorkspaceOverview(ctx, "7")
	if err != nil || overview.BoardCount != 1 || overview.ItemCount != 1 {
		t.Fatalf("overview = %+v err=%v", overview, err)
	}
}

func TestCollaborationFlows(t *testing.T) {
	svc, port := newService(nil)
	if _, err := svc.CreateUpdate(ctx, "500", "<p>hi</p>"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReplyToUpdate(ctx, "500", "77", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := svc.LikeUpdate(ctx, "500", "77"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Notify(ctx, "1", "500", "", "ping"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetUser(ctx, "404"); !monday.IsNotFound(err) {
		t.Fatalf("missing user err = %v", err)
	}
	if got := strings.Join(port.Mutations, ","); got != "create_update,create_update,like_update,create_notification" {
		t.Fatalf("mutations = %s", got)
	}
}

func TestBoundLimit(t *testing.T) {
	if v, _ := application.BoundLimit(0, 25, 100); v != 25 {
		t.Fatalf("default = %d", v)
	}
	if _, err := application.BoundLimit(-1, 25, 100); err == nil {
		t.Fatal("negative must fail")
	}
}
