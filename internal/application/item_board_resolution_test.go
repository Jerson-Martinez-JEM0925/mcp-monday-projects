package application_test

import (
	"strings"
	"testing"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
)

// TestUpdateItemValuesValidatesAgainstItemBoard verifies that column values are
// validated against the board the item actually lives on, not the board_id the
// caller passed. monday accepts a mutation against the wrong board and silently
// discards it (a no-op that still returns success); this is most common with
// subitems, whose board has a different set of columns than the parent board.
// Resolving the real board makes an unknown column fail with a clear error
// instead of a false "ok".
func TestUpdateItemValuesValidatesAgainstItemBoard(t *testing.T) {
	svc, port := newService(application.NewWriteGuard(false, nil, nil))

	// Board "200" (e.g. a subitems board) has NO "est" column; board "100" does.
	port.Columns["200"] = []domain.Column{
		{ID: "name", Title: "Name", Type: "name"},
		{ID: "owner", Title: "Owner", Type: "people"},
	}
	// Item "600" lives on board "200", not on "100".
	port.Items["600"] = domain.Item{ID: "600", Name: "Subitem", BoardID: "200"}

	// Caller passes board "100" (which HAS "est") but the item is on "200".
	_, err := svc.UpdateItemValues(ctx, "100", "600", map[string]any{"est": 1})
	if err == nil {
		t.Fatalf("expected an error: 'est' does not exist on the item's real board 200")
	}
	if !strings.Contains(err.Error(), "unknown column") {
		t.Fatalf("expected an 'unknown column' error, got: %v", err)
	}
	// And nothing must have been written.
	for _, m := range port.Mutations {
		if strings.HasPrefix(m, "update_item_values:") {
			t.Fatalf("no write should reach the port for an invalid column, saw: %s", m)
		}
	}
}

// TestUpdateItemValuesAppliesOnItemBoard verifies the happy path: a column that
// exists on the item's real board is accepted and written against that board.
func TestUpdateItemValuesAppliesOnItemBoard(t *testing.T) {
	svc, port := newService(application.NewWriteGuard(false, nil, nil))

	port.Columns["200"] = []domain.Column{
		{ID: "name", Title: "Name", Type: "name"},
		{ID: "owner", Title: "Owner", Type: "people"},
	}
	port.Items["600"] = domain.Item{ID: "600", Name: "Subitem", BoardID: "200"}

	// "owner" exists on board 200; pass the parent board 100 as board_id.
	if _, err := svc.UpdateItemValues(ctx, "100", "600", map[string]any{"owner": []any{1}}); err != nil {
		t.Fatalf("unexpected error writing a valid column on the item's board: %v", err)
	}
	wrote := false
	for _, m := range port.Mutations {
		if m == "update_item_values:600" {
			wrote = true
		}
	}
	if !wrote {
		t.Fatalf("expected the write to reach the port for item 600")
	}
}
