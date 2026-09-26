package monday

import (
	"context"
	"testing"
)

func TestDeleteMutationsSendVerifiedArguments(t *testing.T) {
	cases := []struct {
		name      string
		operation string
		field     string
		call      func(*Client) error
		want      map[string]any
	}{
		{"item", "DeleteItem", "delete_item", func(c *Client) error { return c.DeleteItem(context.Background(), "11") }, map[string]any{"itemID": "11"}},
		{"group", "DeleteGroup", "delete_group", func(c *Client) error { return c.DeleteGroup(context.Background(), "7", "topics") }, map[string]any{"boardID": "7", "groupID": "topics"}},
		{"board", "DeleteBoard", "delete_board", func(c *Client) error { return c.DeleteBoard(context.Background(), "7") }, map[string]any{"boardID": "7"}},
		{"column", "DeleteColumn", "delete_column", func(c *Client) error { return c.DeleteColumn(context.Background(), "7", "status") }, map[string]any{"boardID": "7", "columnID": "status"}},
		{"update", "DeleteUpdate", "delete_update", func(c *Client) error { return c.DeleteUpdate(context.Background(), "33") }, map[string]any{"updateID": "33"}},
		{"folder", "DeleteFolder", "delete_folder", func(c *Client) error { return c.DeleteFolder(context.Background(), "44") }, map[string]any{"folderID": "44"}},
		{"workspace", "DeleteWorkspace", "delete_workspace", func(c *Client) error { return c.DeleteWorkspace(context.Background(), "55") }, map[string]any{"workspaceID": "55"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, c := newContractClient(t, tc.operation, `{"data":{"`+tc.field+`":{"id":"1"}}}`)
			if err := tc.call(client); err != nil {
				t.Fatal(err)
			}
			for key, value := range tc.want {
				if c.variables[key] != value {
					t.Fatalf("%s = %#v, want %#v (all: %v)", key, c.variables[key], value, c.variables)
				}
			}
		})
	}
}

func TestDeleteOfMissingTargetIsNotFound(t *testing.T) {
	client, _ := newContractClient(t, "DeleteItem", `{"data":{"delete_item":null}}`)
	if err := client.DeleteItem(context.Background(), "404"); !IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
}
