package mcpserver_test

import (
	"testing"
)

// TestEveryToolIsCallable invokes every registered tool with valid arguments
// through the MCP protocol against the in-memory port. It fails if a tool is
// added without a case here, so new handlers always get an end-to-end check.
func TestEveryToolIsCallable(t *testing.T) {
	yes := true
	no := false
	cases := map[string]map[string]any{
		"server_info":       nil,
		"list_tool_catalog": {"category": "bulk"},
		"get_me":            nil,
		"get_api_status":    nil,

		"list_workspaces":  {"limit": 5, "kind": "closed"},
		"get_workspace":    {"workspace_id": "7"},
		"create_workspace": {"name": "Sandbox", "kind": "closed"},
		"list_folders":     {"workspace_id": "7"},
		"create_folder":    {"workspace_id": "7", "name": "MCP"},

		"list_boards":                   {"workspace_ids": []any{"7"}, "order_by": "used_at"},
		"get_board":                     {"board_id": "100"},
		"get_board_schema":              {"board_id": "100"},
		"create_board":                  {"name": "New", "workspace_id": "7", "empty": true},
		"update_board":                  {"board_id": "100", "attribute": "description", "value": "d"},
		"archive_board":                 {"board_id": "100", "confirm": yes},
		"duplicate_board":               {"board_id": "100", "name": "Copy"},
		"add_board_subscribers":         {"board_id": "100", "user_ids": []any{"1"}},
		"list_board_templates":          nil,
		"provision_board_from_template": {"template": "incident", "workspace_id": "7"},

		"list_board_groups": {"board_id": "100"},
		"create_group":      {"board_id": "100", "name": "Next"},
		"update_group":      {"board_id": "100", "group_id": "todo", "attribute": "title", "value": "Doing"},
		"duplicate_group":   {"board_id": "100", "group_id": "todo"},
		"archive_group":     {"board_id": "100", "group_id": "todo", "confirm": yes},

		"list_board_columns":      {"board_id": "100"},
		"create_column":           {"board_id": "100", "title": "Risk", "column_type": "status", "labels": []any{"Low", "High"}},
		"update_column":           {"board_id": "100", "column_id": "est", "title": "Points"},
		"describe_column_formats": nil,

		"list_items":                  {"board_id": "100", "limit": 1},
		"list_group_items":            {"board_id": "100", "group_id": "todo"},
		"search_items":                {"board_id": "100", "text": "Seed"},
		"find_items_by_column_values": {"board_id": "100", "columns": []any{map[string]any{"column_id": "status", "column_values": []any{"Done"}}}},
		"get_item":                    {"item_id": "500"},
		"get_items":                   {"item_ids": []any{"500"}},
		"list_subitems":               {"item_id": "500"},
		"validate_column_values":      {"board_id": "100", "column_values": map[string]any{"status": "Done"}},

		"create_item":               {"board_id": "100", "name": "x", "column_values": map[string]any{"due": "2026-10-01"}},
		"create_subitem":            {"parent_item_id": "500", "name": "child"},
		"update_item_column_values": {"board_id": "100", "item_id": "500", "column_values": map[string]any{"est": 2}},
		"set_item_status":           {"board_id": "100", "item_id": "500", "label": "Stuck"},
		"set_item_date":             {"board_id": "100", "item_id": "500", "date": "2026-10-02"},
		"assign_item_people":        {"board_id": "100", "item_id": "500", "user_ids": []any{"1"}},
		"rename_item":               {"board_id": "100", "item_id": "500", "name": "Renamed"},
		"move_item":                 {"item_id": "500", "group_id": "todo"},
		"move_item_to_board":        {"item_id": "500", "board_id": "100", "group_id": "todo"},
		"duplicate_item":            {"board_id": "100", "item_id": "500"},
		"archive_item":              {"item_id": "500"},

		"bulk_update_items":  {"board_id": "100", "updates": []any{map[string]any{"item_id": "500", "column_values": map[string]any{"est": 1}}}, "dry_run": no},
		"bulk_move_items":    {"item_ids": []any{"500"}, "group_id": "todo"},
		"bulk_archive_items": {"item_ids": []any{"500"}},

		"list_users":   {"limit": 5},
		"search_users": {"name": "Tester"},
		"get_user":     {"user_id": "1"},
		"list_teams":   nil,

		"list_item_updates":  {"item_id": "500"},
		"list_board_updates": {"board_id": "100"},
		"create_update":      {"item_id": "500", "body": "hi"},
		"reply_to_update":    {"item_id": "500", "update_id": "77", "body": "ok"},
		"like_update":        {"item_id": "500", "update_id": "77"},
		"notify_user":        {"user_id": "1", "target_id": "500", "text": "ping"},

		"list_tags":         nil,
		"create_or_get_tag": {"name": "ops"},

		"board_summary":         {"board_id": "100"},
		"column_distribution":   {"board_id": "100", "column_id": "status"},
		"workload_report":       {"board_id": "100"},
		"overdue_items":         {"board_id": "100"},
		"stale_items":           {"board_id": "100", "days": 7},
		"daily_standup":         {"board_id": "100", "hours": 48},
		"board_health_report":   {"board_id": "100", "stale_days": 7},
		"export_board_markdown": {"board_id": "100"},
		"export_board_csv":      {"board_id": "100"},
		"workspace_overview":    {"workspace_id": "7"},
	}
	session, catalog, _ := connect(t, false)
	for _, spec := range catalog {
		args, ok := cases[spec.Name]
		if !ok {
			t.Errorf("tool %s has no callable test case", spec.Name)
			continue
		}
		result, payload := call(t, session, spec.Name, args)
		if result.IsError {
			t.Errorf("%s returned an error: %s", spec.Name, errorText(result))
			continue
		}
		if payload == nil {
			t.Errorf("%s returned no structured content", spec.Name)
		}
	}
	if len(cases) != len(catalog) {
		t.Errorf("cases = %d, catalog = %d", len(cases), len(catalog))
	}
}

// TestInvalidArgumentsAreRejected spot-checks schema and service validation.
func TestInvalidArgumentsAreRejected(t *testing.T) {
	session, _, port := connect(t, false)
	cases := map[string]map[string]any{
		"get_board":                     {"board_id": "not-a-number"},
		"list_items":                    {"board_id": "100", "limit": 1000},
		"list_group_items":              {"board_id": "100"},
		"column_distribution":           {"board_id": "100"},
		"stale_items":                   {"board_id": "100", "days": 9999},
		"search_users":                  {},
		"bulk_move_items":               {"item_ids": []any{"500"}, "group_id": "", "dry_run": false},
		"update_board":                  {"board_id": "100", "attribute": "owner", "value": "x"},
		"provision_board_from_template": {"template": "unknown"},
	}
	for name, args := range cases {
		if result, _ := call(t, session, name, args); !result.IsError {
			t.Errorf("%s accepted invalid arguments %v", name, args)
		}
	}
	if port.MutationCount() != 0 {
		t.Fatalf("invalid calls mutated: %v", port.Mutations)
	}
}
