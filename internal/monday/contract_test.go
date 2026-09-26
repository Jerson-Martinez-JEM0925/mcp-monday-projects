package monday

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/config"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
)

// contract captures one GraphQL request and replies with a canned body.
type contract struct {
	t         *testing.T
	operation string
	reply     string
	query     string
	variables map[string]any
	calls     int32
}

func newContractClient(t *testing.T, operation, reply string) (*Client, *contract) {
	t.Helper()
	c := &contract{t: t, operation: operation, reply: reply}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&c.calls, 1)
		body, _ := io.ReadAll(r.Body)
		var request GraphQLRequest
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("request is not JSON: %v", err)
		}
		c.query, c.variables = request.Query, request.Variables
		if !strings.Contains(request.Query, c.operation) {
			t.Errorf("query does not contain %q: %s", c.operation, request.Query)
		}
		_, _ = w.Write([]byte(c.reply))
	}))
	t.Cleanup(server.Close)
	client := NewClient(config.Config{APIToken: "token", APIVersion: "2026-07", APIURL: server.URL, HTTPTimeout: time.Second, MaxResponseBytes: 1 << 20})
	return client, c
}

const itemJSON = `{"id":"11","name":"Task","state":"active","url":"https://x/11","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z",
 "board":{"id":"7"},"group":{"id":"topics","title":"Topics"},"creator":{"id":"5","name":"Ana"},"parent_item":{"id":"9"},
 "column_values":[{"id":"status","text":"Done","value":"{\"index\":1}","type":"status"},{"id":"empty","text":null,"value":null,"type":"text"}],
 "subitems":[{"id":"12","name":"Child","column_values":[]}]}`

func TestWireItemMapping(t *testing.T) {
	client, _ := newContractClient(t, "GetItems", `{"data":{"items":[`+itemJSON+`]}}`)
	item, err := client.GetItem(context.Background(), "11")
	if err != nil {
		t.Fatal(err)
	}
	if item.BoardID != "7" || item.ParentItemID != "9" || item.Creator.Name != "Ana" || item.Group.Title != "Topics" {
		t.Fatalf("item = %+v", item)
	}
	if item.ColumnValues[0].Value != `{"index":1}` || item.ColumnValues[1].Text != "" || item.ColumnValues[1].Value != "" {
		t.Fatalf("column values = %+v", item.ColumnValues)
	}
	if len(item.Subitems) != 1 || item.Subitems[0].ID != "12" {
		t.Fatalf("subitems = %+v", item.Subitems)
	}
}

func TestGetItemNotFound(t *testing.T) {
	client, _ := newContractClient(t, "GetItems", `{"data":{"items":[]}}`)
	if _, err := client.GetItem(context.Background(), "1"); !IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestCreateItemSendsColumnValuesAsJSONString(t *testing.T) {
	client, c := newContractClient(t, "CreateItem", `{"data":{"create_item":`+itemJSON+`}}`)
	_, err := client.CreateItem(context.Background(), "7", "", "Task", map[string]any{"status": map[string]any{"label": "Done"}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, ok := c.variables["columnValues"].(string)
	if !ok || encoded != `{"status":{"label":"Done"}}` {
		t.Fatalf("columnValues = %#v, want JSON string", c.variables["columnValues"])
	}
	if _, present := c.variables["groupID"]; present {
		t.Fatal("empty groupID must be omitted, not sent as null")
	}
}

func TestUpdateItemValuesEncodesJSON(t *testing.T) {
	client, c := newContractClient(t, "change_multiple_column_values", `{"data":{"change_multiple_column_values":`+itemJSON+`}}`)
	if _, err := client.UpdateItemValues(context.Background(), "7", "11", map[string]any{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if c.variables["columnValues"] != `{"text":"hi"}` {
		t.Fatalf("columnValues = %#v", c.variables["columnValues"])
	}
}

func TestListItemsPageUsesCursorAndFilters(t *testing.T) {
	client, c := newContractClient(t, "ListItems", `{"data":{"boards":[{"items_page":{"cursor":"abc","items":[`+itemJSON+`]}}]}}`)
	filter := &domain.ItemFilter{Operator: "and", Rules: []domain.ItemFilterRule{{ColumnID: "name", CompareValue: []any{"x"}, Operator: "contains_text"}}}
	page, err := client.ListItemsPage(context.Background(), ItemPageQuery{BoardID: "7", Limit: 5, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	if page.Cursor != "abc" || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	query := c.variables["query"].(map[string]any)
	if query["operator"] != "and" || len(query["rules"].([]any)) != 1 {
		t.Fatalf("query variable = %#v", query)
	}

	next, c2 := newContractClient(t, "next_items_page", `{"data":{"next_items_page":{"cursor":null,"items":[]}}}`)
	page, err = next.ListItemsPage(context.Background(), ItemPageQuery{Limit: 5, Cursor: "abc"})
	if err != nil || page.Cursor != "" || c2.variables["cursor"] != "abc" {
		t.Fatalf("next page = %+v err=%v vars=%v", page, err, c2.variables)
	}
}

func TestListGroupItemsNotFound(t *testing.T) {
	client, _ := newContractClient(t, "ListGroupItems", `{"data":{"boards":[{"groups":[]}]}}`)
	_, err := client.ListItemsPage(context.Background(), ItemPageQuery{BoardID: "7", GroupID: "g", Limit: 5})
	var nf *NotFoundError
	if !errors.As(err, &nf) || nf.Resource != "group" {
		t.Fatalf("err = %v", err)
	}
}

func TestColumnSettingsAcceptObjectOrString(t *testing.T) {
	client, _ := newContractClient(t, "ListColumns", `{"data":{"boards":[{"columns":[
	  {"id":"a","title":"A","type":"status","settings":{"labels":[{"id":1,"label":"Done","is_done":true}]}},
	  {"id":"b","title":"B","type":"status","settings":"{\"labels\":{\"0\":\"Working\"}}"},
	  {"id":"c","title":"C","type":"text","settings":null}]}]}}`)
	columns, err := client.ListColumns(context.Background(), "7")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns[0].Labels()) != 1 || columns[1].Labels()[0].Label != "Working" || columns[2].Settings != nil {
		t.Fatalf("columns = %+v", columns)
	}
}

func TestCreateColumnEncodesDefaults(t *testing.T) {
	client, c := newContractClient(t, "CreateColumn", `{"data":{"create_column":{"id":"status","title":"Status","type":"status"}}}`)
	_, err := client.CreateColumn(context.Background(), CreateColumnInput{BoardID: "7", ID: "status", Title: "Status", Type: "status", Defaults: map[string]any{"labels": map[string]any{"1": "Done"}}})
	if err != nil {
		t.Fatal(err)
	}
	if c.variables["defaults"] != `{"labels":{"1":"Done"}}` {
		t.Fatalf("defaults = %#v", c.variables["defaults"])
	}
}

func TestBoardSchemaAndWorkspaceQueries(t *testing.T) {
	client, _ := newContractClient(t, "GetBoardSchema", `{"data":{"boards":[{"id":"7","name":"B","columns":[{"id":"name","title":"Name","type":"name"}],"groups":[{"id":"g","title":"G"}],"owners":[{"id":"1","name":"A"}],"tags":[]}]}}`)
	schema, err := client.GetBoardSchema(context.Background(), "7")
	if err != nil || schema.Name != "B" || len(schema.Columns) != 1 || schema.Owners[0].Name != "A" {
		t.Fatalf("schema = %+v err = %v", schema, err)
	}
	ws, c := newContractClient(t, "ListWorkspaces", `{"data":{"workspaces":[{"id":"1","name":"DevOps","kind":"closed"}]}}`)
	list, err := ws.ListWorkspacesPage(context.Background(), 10, 0, "closed", "")
	if err != nil || len(list) != 1 || c.variables["kind"] != "closed" {
		t.Fatalf("workspaces = %+v vars=%v err=%v", list, c.variables, err)
	}
	if _, present := c.variables["page"]; present {
		t.Fatal("page=0 must be omitted")
	}
}

func TestPeopleAndCollaborationQueries(t *testing.T) {
	users, c := newContractClient(t, "ListUsers", `{"data":{"users":[{"id":"1","name":"Ana","time_zone_identifier":"America/Managua","teams":[{"id":"3","name":"Ops"}]}]}}`)
	list, err := users.ListUsers(context.Background(), UserQuery{Limit: 5, Name: "Ana"})
	if err != nil || list[0].TimeZone != "America/Managua" || list[0].Teams[0].Name != "Ops" || c.variables["name"] != "Ana" {
		t.Fatalf("users = %+v err=%v", list, err)
	}
	me, _ := newContractClient(t, "query Me", `{"data":{"me":{"id":"1","name":"Ana","kind":"admin","account":{"id":"9","name":"Acme","slug":"acme"}}}}`)
	identity, err := me.Me(context.Background())
	if err != nil || !identity.IsAdmin || identity.Account.Slug != "acme" {
		t.Fatalf("me = %+v err=%v", identity, err)
	}
	updates, _ := newContractClient(t, "ItemUpdates", `{"data":{"items":[{"updates":[{"id":"u1","text_body":"hi","replies":[{"id":"r1","text_body":"ok"}]}]}]}}`)
	list2, err := updates.ListItemUpdates(context.Background(), "11", 5)
	if err != nil || list2[0].Replies[0].ID != "r1" {
		t.Fatalf("updates = %+v err=%v", list2, err)
	}
	reply, c3 := newContractClient(t, "CreateUpdate", `{"data":{"create_update":{"id":"u2","body":"x"}}}`)
	if _, err := reply.CreateUpdate(context.Background(), "", "x", "u1"); err != nil || c3.variables["parentID"] != "u1" {
		t.Fatalf("reply vars = %v err=%v", c3.variables, err)
	}
	if _, present := c3.variables["itemID"]; present {
		t.Fatal("empty itemID must be omitted for replies")
	}
	status, _ := newContractClient(t, "APIStatus", `{"data":{"complexity":{"before":100,"after":90,"query":10,"reset_in_x_seconds":30},"version":{"value":"2026-07","kind":"current"}}}`)
	complexity, version, err := status.APIStatus(context.Background())
	if err != nil || complexity.Query != 10 || version.Value != "2026-07" {
		t.Fatalf("status = %+v %+v err=%v", complexity, version, err)
	}
}

func TestUpdateBoardDetectsRejectedMutation(t *testing.T) {
	client, _ := newContractClient(t, "UpdateBoard", `{"data":{"update_board":"{\"success\":false,\"error\":\"nope\"}"}}`)
	if err := client.UpdateBoard(context.Background(), "7", "name", "x"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v", err)
	}
	ok, _ := newContractClient(t, "UpdateBoard", `{"data":{"update_board":"{\"success\":true}"}}`)
	if err := ok.UpdateBoard(context.Background(), "7", "name", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestRequestErrorIsTypedAndComplexityIsRetried(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			_, _ = w.Write([]byte(`{"errors":[{"message":"budget","extensions":{"code":"ComplexityException","retry_in_seconds":0}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer server.Close()
	client := NewClient(config.Config{APIToken: "t", APIURL: server.URL, HTTPTimeout: time.Second, MaxResponseBytes: 4096, MaxRetries: 1})
	if err := client.Do(context.Background(), "query { ok }", nil, nil); err != nil || attempts != 2 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}

	permanent, _ := newContractClient(t, "q", `{"errors":[{"message":"denied","path":["boards",0],"extensions":{"code":"USER_UNAUTHORIZED"}}]}`)
	err := permanent.Do(context.Background(), "query q { boards { id } }", nil, nil)
	var reqErr *RequestError
	if !errors.As(err, &reqErr) || reqErr.Code != "USER_UNAUTHORIZED" || reqErr.Path != "boards.0" || reqErr.Temporary() {
		t.Fatalf("err = %#v", err)
	}
	legacy, _ := newContractClient(t, "q", `{"error_code":"InvalidBoardIdException","error_message":"bad board"}`)
	if err := legacy.Do(context.Background(), "query q { x }", nil, nil); !errors.As(err, &reqErr) || reqErr.Code != "InvalidBoardIdException" {
		t.Fatalf("legacy err = %v", err)
	}
}

func TestCompactVariablesDropsNil(t *testing.T) {
	got := compactVariables(map[string]any{"a": 1, "b": nil})
	if len(got) != 1 || got["a"] != 1 {
		t.Fatalf("got %v", got)
	}
	if compactVariables(nil) != nil {
		t.Fatal("nil map must stay nil")
	}
}

func TestResponseLimitIsEnforced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"x":"` + strings.Repeat("a", 5000) + `"}}`))
	}))
	defer server.Close()
	client := NewClient(config.Config{APIToken: "t", APIURL: server.URL, HTTPTimeout: time.Second, MaxResponseBytes: 1024})
	if err := client.Do(context.Background(), "query { x }", nil, nil); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v", err)
	}
}
