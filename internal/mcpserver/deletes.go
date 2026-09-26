package mcpserver

import (
	"context"
	"fmt"
)

// Permanent-delete tools. They are registered only at MCP_ACCESS_LEVEL=full,
// carry destructiveHint=true, and every one requires confirm=true.

// DeleteItemInput identifies an item or subitem to delete.
type DeleteItemInput struct {
	ItemID  string `json:"item_id" jsonschema:"item or subitem to delete permanently"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

// DeleteGroupInput identifies a group to delete.
type DeleteGroupInput struct {
	BoardID string `json:"board_id" jsonschema:"board that owns the group"`
	GroupID string `json:"group_id" jsonschema:"group to delete permanently, with all of its items"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

// DeleteBoardInput identifies a board to delete.
type DeleteBoardInput struct {
	BoardID string `json:"board_id" jsonschema:"board to delete permanently"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

// DeleteColumnInput identifies a column to delete.
type DeleteColumnInput struct {
	BoardID  string `json:"board_id" jsonschema:"board that owns the column"`
	ColumnID string `json:"column_id" jsonschema:"column to delete permanently, with all of its values"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

// DeleteUpdateInput identifies an update to delete.
type DeleteUpdateInput struct {
	ItemID   string `json:"item_id" jsonschema:"item the update belongs to (checked against allowlists and scope)"`
	UpdateID string `json:"update_id" jsonschema:"update or reply to delete permanently"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

// DeleteFolderInput identifies a folder to delete.
type DeleteFolderInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"workspace that owns the folder; defaults to MONDAY_WORKSPACE_ID when the server is scoped"`
	FolderID    string `json:"folder_id" jsonschema:"folder to delete permanently, with the boards inside it"`
	Confirm     bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

// DeleteWorkspaceInput identifies a workspace to delete.
type DeleteWorkspaceInput struct {
	WorkspaceID string `json:"workspace_id" jsonschema:"workspace to delete permanently, with everything in it"`
	Confirm     bool   `json:"confirm,omitempty" jsonschema:"must be true: the delete is permanent and cannot be undone"`
}

func deleted(resource, id string) OKOutput {
	return OKOutput{OK: true, Message: fmt.Sprintf("%s %s permanently deleted", resource, id)}
}

func registerDeleteTools(r *registry) {
	svc := r.svc
	del := func(name, title, capability, description string) ToolSpec {
		return ToolSpec{Name: name, Category: CatDeletes, Title: title, Destructive: true, Permanent: true, Capability: capability, Description: description}
	}

	add(r, del("delete_item", "Delete item", "items.delete",
		"PERMANENTLY delete an item or subitem (MCP_ACCESS_LEVEL=full). Requires confirm=true; prefer archive_item to keep it restorable."),
		func(ctx context.Context, in DeleteItemInput) (OKOutput, error) {
			if err := svc.DeleteItem(ctx, in.ItemID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete item", err)
			}
			return deleted("item", in.ItemID), nil
		})
	add(r, del("delete_group", "Delete group", "groups.delete",
		"PERMANENTLY delete a group and all of its items (MCP_ACCESS_LEVEL=full). Requires confirm=true; prefer archive_group."),
		func(ctx context.Context, in DeleteGroupInput) (OKOutput, error) {
			if err := svc.DeleteGroup(ctx, in.BoardID, in.GroupID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete group", err)
			}
			return deleted("group", in.GroupID), nil
		})
	add(r, del("delete_board", "Delete board", "boards.delete",
		"PERMANENTLY delete a board (MCP_ACCESS_LEVEL=full). Requires confirm=true; prefer archive_board."),
		func(ctx context.Context, in DeleteBoardInput) (OKOutput, error) {
			if err := svc.DeleteBoard(ctx, in.BoardID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete board", err)
			}
			return deleted("board", in.BoardID), nil
		})
	add(r, del("delete_column", "Delete column", "columns.delete",
		"PERMANENTLY delete a column and all of its values (MCP_ACCESS_LEVEL=full). Requires confirm=true. The name column cannot be deleted."),
		func(ctx context.Context, in DeleteColumnInput) (OKOutput, error) {
			if err := svc.DeleteColumn(ctx, in.BoardID, in.ColumnID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete column", err)
			}
			return deleted("column", in.ColumnID), nil
		})
	add(r, del("delete_update", "Delete update", "updates.delete",
		"PERMANENTLY delete an update or reply of an item (MCP_ACCESS_LEVEL=full). Requires confirm=true and the owning item_id."),
		func(ctx context.Context, in DeleteUpdateInput) (OKOutput, error) {
			if err := svc.DeleteUpdate(ctx, in.ItemID, in.UpdateID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete update", err)
			}
			return deleted("update", in.UpdateID), nil
		})
	add(r, del("delete_folder", "Delete folder", "workspaces.delete",
		"PERMANENTLY delete a workspace folder and the boards inside it (MCP_ACCESS_LEVEL=full). Requires confirm=true."),
		func(ctx context.Context, in DeleteFolderInput) (OKOutput, error) {
			if err := svc.DeleteFolder(ctx, in.WorkspaceID, in.FolderID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete folder", err)
			}
			return deleted("folder", in.FolderID), nil
		})
	add(r, del("delete_workspace", "Delete workspace", "workspaces.delete",
		"PERMANENTLY delete a workspace and everything in it (MCP_ACCESS_LEVEL=full). Requires confirm=true. Refused when MONDAY_WORKSPACE_ID or a write allowlist is set."),
		func(ctx context.Context, in DeleteWorkspaceInput) (OKOutput, error) {
			if err := svc.DeleteWorkspace(ctx, in.WorkspaceID, in.Confirm); err != nil {
				return OKOutput{}, wrap("delete workspace", err)
			}
			return deleted("workspace", in.WorkspaceID), nil
		})
}
