package monday

import (
	"context"
	"fmt"
)

// Permanent-delete mutations. Signatures verified by introspection against
// API version 2026-07. They are only reachable at MCP_ACCESS_LEVEL=full.
const (
	deleteItemMutation = `mutation DeleteItem($itemID: ID!) {
  delete_item(item_id: $itemID) { id }
}`
	deleteGroupMutation = `mutation DeleteGroup($boardID: ID!, $groupID: String!) {
  delete_group(board_id: $boardID, group_id: $groupID) { id }
}`
	deleteBoardMutation = `mutation DeleteBoard($boardID: ID!) {
  delete_board(board_id: $boardID) { id }
}`
	deleteColumnMutation = `mutation DeleteColumn($boardID: ID!, $columnID: String!) {
  delete_column(board_id: $boardID, column_id: $columnID) { id }
}`
	deleteUpdateMutation = `mutation DeleteUpdate($updateID: ID!) {
  delete_update(id: $updateID) { id }
}`
	deleteFolderMutation = `mutation DeleteFolder($folderID: ID!) {
  delete_folder(folder_id: $folderID) { id }
}`
	deleteWorkspaceMutation = `mutation DeleteWorkspace($workspaceID: ID!) {
  delete_workspace(workspace_id: $workspaceID) { id }
}`
)

// deleted decodes `{ "<field>": { "id": ... } }` and fails when monday
// returns no object, which it does for targets that no longer exist.
func (c *Client) deleted(ctx context.Context, query, field, resource, id string, vars map[string]any) error {
	var data map[string]*struct {
		ID any `json:"id"`
	}
	if err := c.Do(ctx, query, vars, &data); err != nil {
		return err
	}
	if data[field] == nil || data[field].ID == nil || fmt.Sprint(data[field].ID) == "" {
		return &NotFoundError{Resource: resource, ID: id}
	}
	return nil
}

// DeleteItem permanently deletes an item or subitem.
func (c *Client) DeleteItem(ctx context.Context, itemID string) error {
	return c.deleted(ctx, deleteItemMutation, "delete_item", "item", itemID, map[string]any{"itemID": itemID})
}

// DeleteGroup permanently deletes a group and its items.
func (c *Client) DeleteGroup(ctx context.Context, boardID, groupID string) error {
	return c.deleted(ctx, deleteGroupMutation, "delete_group", "group", groupID, map[string]any{"boardID": boardID, "groupID": groupID})
}

// DeleteBoard permanently deletes a board.
func (c *Client) DeleteBoard(ctx context.Context, boardID string) error {
	return c.deleted(ctx, deleteBoardMutation, "delete_board", "board", boardID, map[string]any{"boardID": boardID})
}

// DeleteColumn permanently deletes a column and its values.
func (c *Client) DeleteColumn(ctx context.Context, boardID, columnID string) error {
	return c.deleted(ctx, deleteColumnMutation, "delete_column", "column", columnID, map[string]any{"boardID": boardID, "columnID": columnID})
}

// DeleteUpdate permanently deletes an update (or reply).
func (c *Client) DeleteUpdate(ctx context.Context, updateID string) error {
	return c.deleted(ctx, deleteUpdateMutation, "delete_update", "update", updateID, map[string]any{"updateID": updateID})
}

// DeleteFolder permanently deletes a folder.
func (c *Client) DeleteFolder(ctx context.Context, folderID string) error {
	return c.deleted(ctx, deleteFolderMutation, "delete_folder", "folder", folderID, map[string]any{"folderID": folderID})
}

// DeleteWorkspace permanently deletes a workspace.
func (c *Client) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	return c.deleted(ctx, deleteWorkspaceMutation, "delete_workspace", "workspace", workspaceID, map[string]any{"workspaceID": workspaceID})
}
