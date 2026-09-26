package application

import (
	"context"
	"errors"
	"fmt"
)

// ErrDeletesDisabled is returned when a permanent delete is requested while
// the server does not run at MCP_ACCESS_LEVEL=full.
var ErrDeletesDisabled = errors.New("permanent deletes are disabled; set MCP_ACCESS_LEVEL=full to enable them (archive_* tools remain available)")

// requireDelete gates every permanent delete: the access level must allow it,
// the caller must confirm, and the server must accept writes.
func (s *Service) requireDelete(resource, alternative string, confirm bool) error {
	if !s.allowDelete {
		return ErrDeletesDisabled
	}
	if !confirm {
		hint := ""
		if alternative != "" {
			hint = fmt.Sprintf(", or use %s to keep it restorable", alternative)
		}
		return invalid("deleting a %s is permanent and cannot be undone; pass confirm=true%s", resource, hint)
	}
	return s.guard.CheckWrite()
}

// AllowsDelete reports whether permanent deletes are enabled.
func (s *Service) AllowsDelete() bool { return s.allowDelete }

// DeleteItem permanently deletes an item or subitem.
func (s *Service) DeleteItem(ctx context.Context, itemID string, confirm bool) error {
	if err := requireID("item_id", itemID); err != nil {
		return err
	}
	if err := s.requireDelete("item", "archive_item", confirm); err != nil {
		return err
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return err
	}
	return s.port.DeleteItem(ctx, itemID)
}

// DeleteGroup permanently deletes a group and every item in it.
func (s *Service) DeleteGroup(ctx context.Context, boardID, groupID string, confirm bool) error {
	if err := requireID("board_id", boardID); err != nil {
		return err
	}
	if err := requireText("group_id", groupID); err != nil {
		return err
	}
	if err := s.requireDelete("group (and all of its items)", "archive_group", confirm); err != nil {
		return err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return err
	}
	return s.port.DeleteGroup(ctx, boardID, groupID)
}

// DeleteBoard permanently deletes a board.
func (s *Service) DeleteBoard(ctx context.Context, boardID string, confirm bool) error {
	if err := requireID("board_id", boardID); err != nil {
		return err
	}
	if err := s.requireDelete("board", "archive_board", confirm); err != nil {
		return err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return err
	}
	return s.port.DeleteBoard(ctx, boardID)
}

// DeleteColumn permanently deletes a column and all of its values.
func (s *Service) DeleteColumn(ctx context.Context, boardID, columnID string, confirm bool) error {
	if err := requireID("board_id", boardID); err != nil {
		return err
	}
	if err := requireText("column_id", columnID); err != nil {
		return err
	}
	if columnID == "name" {
		return invalid("the name column cannot be deleted")
	}
	if err := s.requireDelete("column (and all of its values)", "", confirm); err != nil {
		return err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return err
	}
	return s.port.DeleteColumn(ctx, boardID, columnID)
}

// DeleteUpdate permanently deletes an update. The update must belong to
// item_id, which is checked against the allowlists and workspace scope.
func (s *Service) DeleteUpdate(ctx context.Context, itemID, updateID string, confirm bool) error {
	if err := requireID("item_id", itemID); err != nil {
		return err
	}
	if err := requireID("update_id", updateID); err != nil {
		return err
	}
	if err := s.requireDelete("update", "", confirm); err != nil {
		return err
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return err
	}
	updates, err := s.port.ListItemUpdates(ctx, itemID, 100)
	if err != nil {
		return err
	}
	for _, update := range updates {
		if update.ID == updateID {
			return s.port.DeleteUpdate(ctx, updateID)
		}
		for _, reply := range update.Replies {
			if reply.ID == updateID {
				return s.port.DeleteUpdate(ctx, updateID)
			}
		}
	}
	return invalid("update %s is not among the latest 100 updates of item %s", updateID, itemID)
}

// DeleteFolder permanently deletes a folder of a workspace. Its boards are
// deleted with it by monday.
func (s *Service) DeleteFolder(ctx context.Context, workspaceID, folderID string, confirm bool) error {
	workspaceID = s.scopedWorkspace(workspaceID)
	if err := requireID("workspace_id", workspaceID); err != nil {
		return err
	}
	if err := requireID("folder_id", folderID); err != nil {
		return err
	}
	if err := s.requireDelete("folder (and the boards inside it)", "", confirm); err != nil {
		return err
	}
	if err := s.guard.CheckWorkspace(workspaceID); err != nil {
		return err
	}
	folders, err := s.port.ListFolders(ctx, workspaceID, 100, 1)
	if err != nil {
		return err
	}
	for _, folder := range folders {
		if folder.ID == folderID {
			return s.port.DeleteFolder(ctx, folderID)
		}
	}
	return invalid("folder %s is not in workspace %s", folderID, workspaceID)
}

// DeleteWorkspace permanently deletes a workspace. It is refused whenever a
// workspace scope or write allowlist is configured.
func (s *Service) DeleteWorkspace(ctx context.Context, workspaceID string, confirm bool) error {
	if err := requireID("workspace_id", workspaceID); err != nil {
		return err
	}
	if err := s.requireDelete("workspace (and everything in it)", "", confirm); err != nil {
		return err
	}
	if s.scope != "" {
		return &ScopeError{Resource: "workspace", ID: workspaceID, Scope: s.scope}
	}
	if s.guard.Restricted() {
		return invalid("deleting workspaces is disabled while a write allowlist is configured")
	}
	return s.port.DeleteWorkspace(ctx, workspaceID)
}
