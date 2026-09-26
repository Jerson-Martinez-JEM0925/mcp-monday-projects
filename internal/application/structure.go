package application

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

var idRe = regexp.MustCompile(`^[0-9]+$`)

// InputError reports an invalid tool argument before any API call.
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &InputError{Message: fmt.Sprintf(format, args...)}
}

func requireID(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalid("%s is required", name)
	}
	if !idRe.MatchString(value) {
		return invalid("%s must be a numeric monday ID, got %q", name, value)
	}
	return nil
}

func requireText(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalid("%s is required", name)
	}
	return nil
}

func requireIDs(name string, values []string, max int) error {
	if len(values) == 0 {
		return invalid("%s must contain at least one ID", name)
	}
	if len(values) > max {
		return invalid("%s accepts at most %d IDs", name, max)
	}
	for _, value := range values {
		if err := requireID(name, value); err != nil {
			return err
		}
	}
	return nil
}

func oneOf(name, value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return invalid("%s must be one of %s", name, strings.Join(allowed, ", "))
}

// BoundLimit applies a default and validates an upper bound.
func BoundLimit(limit, fallback, max int) (int, error) {
	if limit == 0 {
		return fallback, nil
	}
	if limit < 1 || limit > max {
		return 0, invalid("limit must be between 1 and %d", max)
	}
	return limit, nil
}

// ---- account ---------------------------------------------------------------

// Me returns the authenticated user and account.
func (s *Service) Me(ctx context.Context) (*domain.Me, error) { return s.port.Me(ctx) }

// APIStatus returns the complexity budget and API version.
func (s *Service) APIStatus(ctx context.Context) (domain.Complexity, domain.APIVersion, error) {
	return s.port.APIStatus(ctx)
}

// ---- workspaces & folders --------------------------------------------------

// ListWorkspaces lists workspaces with pagination.
func (s *Service) ListWorkspaces(ctx context.Context, limit, page int, kind, state string) ([]domain.Workspace, error) {
	limit, err := BoundLimit(limit, 25, 100)
	if err != nil {
		return nil, err
	}
	if err := oneOf("kind", kind, "open", "closed", "template"); err != nil {
		return nil, err
	}
	if err := oneOf("state", state, "active", "archived", "deleted", "all"); err != nil {
		return nil, err
	}
	return s.port.ListWorkspacesPage(ctx, limit, page, kind, state)
}

// GetWorkspace returns one workspace.
func (s *Service) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	if err := requireID("workspace_id", id); err != nil {
		return nil, err
	}
	return s.port.GetWorkspace(ctx, id)
}

// CreateWorkspace creates a workspace (requires no allowlist or a write-enabled server).
func (s *Service) CreateWorkspace(ctx context.Context, name, kind, description string) (*domain.Workspace, error) {
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	if kind == "" {
		kind = "open"
	}
	if err := oneOf("kind", kind, "open", "closed"); err != nil {
		return nil, err
	}
	if err := s.guard.CheckWrite(); err != nil {
		return nil, err
	}
	if s.guard.Restricted() {
		return nil, invalid("creating workspaces is disabled while a write allowlist is configured")
	}
	return s.port.CreateWorkspace(ctx, name, kind, description)
}

// ListFolders lists folders, optionally in one workspace.
func (s *Service) ListFolders(ctx context.Context, workspaceID string, limit, page int) ([]domain.Folder, error) {
	if workspaceID != "" {
		if err := requireID("workspace_id", workspaceID); err != nil {
			return nil, err
		}
	}
	limit, err := BoundLimit(limit, 25, 100)
	if err != nil {
		return nil, err
	}
	return s.port.ListFolders(ctx, workspaceID, limit, page)
}

// CreateFolder creates a folder in a workspace.
func (s *Service) CreateFolder(ctx context.Context, workspaceID, name string) (*domain.Folder, error) {
	if err := requireID("workspace_id", workspaceID); err != nil {
		return nil, err
	}
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	if err := s.guard.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	return s.port.CreateFolder(ctx, workspaceID, name)
}

// ---- boards ----------------------------------------------------------------

// ListBoards lists boards with filters.
func (s *Service) ListBoards(ctx context.Context, query monday.BoardQuery) ([]domain.Board, error) {
	limit, err := BoundLimit(query.Limit, 25, 100)
	if err != nil {
		return nil, err
	}
	query.Limit = limit
	for _, id := range query.WorkspaceIDs {
		if err := requireID("workspace_ids", id); err != nil {
			return nil, err
		}
	}
	if err := oneOf("state", query.State, "active", "archived", "deleted", "all"); err != nil {
		return nil, err
	}
	if err := oneOf("board_kind", query.Kind, "public", "private", "share"); err != nil {
		return nil, err
	}
	if err := oneOf("order_by", query.OrderBy, "created_at", "used_at"); err != nil {
		return nil, err
	}
	return s.port.ListBoardsPage(ctx, query)
}

// GetBoard returns one board.
func (s *Service) GetBoard(ctx context.Context, id string) (*domain.Board, error) {
	if err := requireID("board_id", id); err != nil {
		return nil, err
	}
	return s.port.GetBoard(ctx, id)
}

// GetBoardSchema returns a board with columns, groups, people, and tags.
func (s *Service) GetBoardSchema(ctx context.Context, id string) (*monday.BoardSchema, error) {
	if err := requireID("board_id", id); err != nil {
		return nil, err
	}
	return s.port.GetBoardSchema(ctx, id)
}

// CreateBoard creates a board inside an (allowed) workspace.
func (s *Service) CreateBoard(ctx context.Context, input monday.CreateBoardInput) (*domain.Board, error) {
	if err := requireText("name", input.Name); err != nil {
		return nil, err
	}
	if input.Kind == "" {
		input.Kind = "public"
	}
	if err := oneOf("board_kind", input.Kind, "public", "private", "share"); err != nil {
		return nil, err
	}
	if input.WorkspaceID != "" {
		if err := requireID("workspace_id", input.WorkspaceID); err != nil {
			return nil, err
		}
	}
	if err := s.guard.CheckWorkspace(input.WorkspaceID); err != nil {
		return nil, err
	}
	board, err := s.port.CreateBoard(ctx, input)
	if err != nil {
		return nil, err
	}
	s.guard.TrustBoard(board.ID)
	return board, nil
}

// UpdateBoard renames a board or changes its description.
func (s *Service) UpdateBoard(ctx context.Context, id, attribute, value string) error {
	if err := requireID("board_id", id); err != nil {
		return err
	}
	if err := oneOf("attribute", attribute, "name", "description"); err != nil || attribute == "" {
		return invalid("attribute must be name or description")
	}
	if attribute == "name" && strings.TrimSpace(value) == "" {
		return invalid("board name cannot be empty")
	}
	if err := s.guard.CheckBoard(id); err != nil {
		return err
	}
	return s.port.UpdateBoard(ctx, id, attribute, value)
}

// ArchiveBoard archives a board. It requires an explicit confirmation.
func (s *Service) ArchiveBoard(ctx context.Context, id string, confirm bool) (*domain.Board, error) {
	if err := requireID("board_id", id); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, invalid("archiving a board requires confirm=true; the board can be restored from monday's archive")
	}
	if err := s.guard.CheckBoard(id); err != nil {
		return nil, err
	}
	return s.port.ArchiveBoard(ctx, id)
}

// DuplicateBoard duplicates a board into an (allowed) workspace.
func (s *Service) DuplicateBoard(ctx context.Context, id, duplicateType, name, workspaceID, folderID string, keepSubscribers bool) (*domain.Board, error) {
	if err := requireID("board_id", id); err != nil {
		return nil, err
	}
	if duplicateType == "" {
		duplicateType = "duplicate_board_with_structure"
	}
	if err := oneOf("duplicate_type", duplicateType, "duplicate_board_with_structure", "duplicate_board_with_pulses", "duplicate_board_with_pulses_and_updates"); err != nil {
		return nil, err
	}
	if err := s.guard.CheckWorkspace(workspaceID); err != nil {
		return nil, err
	}
	board, err := s.port.DuplicateBoard(ctx, id, duplicateType, name, workspaceID, folderID, keepSubscribers)
	if err != nil {
		return nil, err
	}
	if board.ID != "" {
		s.guard.TrustBoard(board.ID)
	}
	return board, nil
}

// AddBoardSubscribers subscribes users to a board.
func (s *Service) AddBoardSubscribers(ctx context.Context, boardID string, userIDs []string, kind string) ([]domain.UserRef, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireIDs("user_ids", userIDs, 50); err != nil {
		return nil, err
	}
	if kind == "" {
		kind = "subscriber"
	}
	if err := oneOf("kind", kind, "subscriber", "owner"); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.AddUsersToBoard(ctx, boardID, userIDs, kind)
}

// ---- groups ----------------------------------------------------------------

// ListGroups lists the groups of a board.
func (s *Service) ListGroups(ctx context.Context, boardID string) ([]domain.Group, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	return s.port.ListGroups(ctx, boardID)
}

// CreateGroup creates a group.
func (s *Service) CreateGroup(ctx context.Context, boardID, name, color, relativeTo, method string) (*domain.Group, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	if err := oneOf("position_relative_method", method, "before_at", "after_at"); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.CreateGroup(ctx, boardID, name, color, relativeTo, method)
}

// UpdateGroup changes a group's title or color.
func (s *Service) UpdateGroup(ctx context.Context, boardID, groupID, attribute, value string) (*domain.Group, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("group_id", groupID); err != nil {
		return nil, err
	}
	if attribute == "" {
		return nil, invalid("attribute is required")
	}
	if err := oneOf("attribute", attribute, "title", "color", "position", "relative_position_after", "relative_position_before"); err != nil {
		return nil, err
	}
	if err := requireText("value", value); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.UpdateGroup(ctx, boardID, groupID, attribute, value)
}

// DuplicateGroup duplicates a group with its items.
func (s *Service) DuplicateGroup(ctx context.Context, boardID, groupID, title string, addToTop bool) (*domain.Group, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("group_id", groupID); err != nil {
		return nil, err
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.DuplicateGroup(ctx, boardID, groupID, title, addToTop)
}

// ArchiveGroup archives a group (with its items). Requires confirmation.
func (s *Service) ArchiveGroup(ctx context.Context, boardID, groupID string, confirm bool) (*domain.Group, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("group_id", groupID); err != nil {
		return nil, err
	}
	if !confirm {
		return nil, invalid("archiving a group archives all of its items; pass confirm=true")
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	return s.port.ArchiveGroup(ctx, boardID, groupID)
}

// ---- columns ---------------------------------------------------------------

// ListColumns lists board columns.
func (s *Service) ListColumns(ctx context.Context, boardID string) ([]domain.Column, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	return s.port.ListColumns(ctx, boardID)
}

var columnIDRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,19}$`)

// CreateColumn creates a column. labels seed status/dropdown columns.
func (s *Service) CreateColumn(ctx context.Context, boardID, id, title, columnType, description, after string, labels []string) (*domain.Column, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("title", title); err != nil {
		return nil, err
	}
	if err := requireText("column_type", columnType); err != nil {
		return nil, err
	}
	if id != "" && !columnIDRe.MatchString(id) {
		return nil, invalid("column id must match %s", columnIDRe.String())
	}
	if len(labels) > 0 && columnType != "status" && columnType != "dropdown" {
		return nil, invalid("labels are only supported for status and dropdown columns")
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	template := domain.TemplateColumn{ID: id, Title: title, Type: columnType, Labels: labels}
	return s.port.CreateColumn(ctx, monday.CreateColumnInput{
		BoardID: boardID, ID: id, Title: title, Type: columnType, Description: description,
		Defaults: domain.ColumnDefaults(template), AfterColumnID: after,
	})
}

// ChangeColumn renames a column and/or changes its description.
func (s *Service) ChangeColumn(ctx context.Context, boardID, columnID, title, description string) (*domain.Column, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	if err := requireText("column_id", columnID); err != nil {
		return nil, err
	}
	if title == "" && description == "" {
		return nil, invalid("provide title and/or description")
	}
	if err := s.guard.CheckBoard(boardID); err != nil {
		return nil, err
	}
	var column *domain.Column
	var err error
	if title != "" {
		if column, err = s.port.ChangeColumnTitle(ctx, boardID, columnID, title); err != nil {
			return nil, err
		}
	}
	if description != "" {
		if column, err = s.port.ChangeColumnDescription(ctx, boardID, columnID, description); err != nil {
			return nil, err
		}
	}
	return column, nil
}
