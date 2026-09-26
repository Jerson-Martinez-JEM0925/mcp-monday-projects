package application

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// ScopeError reports a request that targets a resource outside the configured
// workspace scope (MONDAY_WORKSPACE_ID).
type ScopeError struct {
	Resource  string
	ID        string
	Workspace string
	Scope     string
}

func (e *ScopeError) Error() string {
	if e.Resource == "workspace" {
		return fmt.Sprintf("workspace %s is outside the configured scope MONDAY_WORKSPACE_ID=%s", e.ID, e.Scope)
	}
	where := "a different workspace"
	if e.Workspace != "" {
		where = "workspace " + e.Workspace
	}
	return fmt.Sprintf("%s %s belongs to %s, outside the configured scope MONDAY_WORKSPACE_ID=%s", e.Resource, e.ID, where, e.Scope)
}

// ScopedPort confines every workspace-, board- and item-level call of the
// wrapped port to a single workspace. Reads and writes are both scoped:
// out-of-scope targets fail with *ScopeError before any mutation reaches
// monday, and list results are filtered to the scope. Account-level calls
// (me, users, teams, tags, API status) pass through unchanged.
type ScopedPort struct {
	Port
	workspace string
	mu        sync.RWMutex
	boards    map[string]string // board ID -> workspace ID
}

// NewScopedPort wraps port so it only reaches the given workspace.
func NewScopedPort(port Port, workspaceID string) *ScopedPort {
	return &ScopedPort{Port: port, workspace: workspaceID, boards: map[string]string{}}
}

// Workspace returns the scoped workspace ID.
func (p *ScopedPort) Workspace() string { return p.workspace }

func (p *ScopedPort) remember(boardID, workspaceID string) {
	if boardID == "" {
		return
	}
	p.mu.Lock()
	p.boards[boardID] = workspaceID
	p.mu.Unlock()
}

// workspaceOr resolves an optional workspace argument against the scope.
func (p *ScopedPort) workspaceOr(id string) (string, error) {
	if id == "" || id == p.workspace {
		return p.workspace, nil
	}
	return "", &ScopeError{Resource: "workspace", ID: id, Workspace: id, Scope: p.workspace}
}

// checkBoard resolves the board's workspace (cached) and rejects foreign ones.
func (p *ScopedPort) checkBoard(ctx context.Context, boardID string) error {
	if boardID == "" {
		return nil
	}
	p.mu.RLock()
	workspaceID, known := p.boards[boardID]
	p.mu.RUnlock()
	if !known {
		board, err := p.Port.GetBoard(ctx, boardID)
		if err != nil {
			return err
		}
		workspaceID = board.WorkspaceID
		p.remember(boardID, workspaceID)
	}
	if workspaceID != p.workspace {
		return &ScopeError{Resource: "board", ID: boardID, Workspace: workspaceID, Scope: p.workspace}
	}
	return nil
}

// checkItems verifies every item's board is inside the scope.
func (p *ScopedPort) checkItems(ctx context.Context, items []domain.Item) error {
	for _, item := range items {
		if item.BoardID == "" {
			continue
		}
		if err := p.checkBoard(ctx, item.BoardID); err != nil {
			var scopeErr *ScopeError
			if errors.As(err, &scopeErr) {
				return &ScopeError{Resource: "item", ID: item.ID, Workspace: scopeErr.Workspace, Scope: p.workspace}
			}
			return err
		}
	}
	return nil
}

func (p *ScopedPort) checkItem(ctx context.Context, itemID string) error {
	if itemID == "" {
		return nil
	}
	items, err := p.Port.GetItems(ctx, []string{itemID})
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return &monday.NotFoundError{Resource: "item", ID: itemID}
	}
	return p.checkItems(ctx, items)
}

func (p *ScopedPort) checkPage(ctx context.Context, page domain.ItemPage, err error) (domain.ItemPage, error) {
	if err != nil {
		return page, err
	}
	if err := p.checkItems(ctx, page.Items); err != nil {
		return domain.ItemPage{}, err
	}
	return page, nil
}

// ---- workspaces & folders --------------------------------------------------

// ListWorkspacesPage returns only the scoped workspace.
func (p *ScopedPort) ListWorkspacesPage(ctx context.Context, _, page int, _, _ string) ([]domain.Workspace, error) {
	if page > 1 {
		return []domain.Workspace{}, nil
	}
	workspace, err := p.Port.GetWorkspace(ctx, p.workspace)
	if err != nil {
		return nil, err
	}
	return []domain.Workspace{*workspace}, nil
}

func (p *ScopedPort) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	id, err := p.workspaceOr(id)
	if err != nil {
		return nil, err
	}
	return p.Port.GetWorkspace(ctx, id)
}

// CreateWorkspace is always refused: a new workspace is outside the scope.
func (p *ScopedPort) CreateWorkspace(context.Context, string, string, string) (*domain.Workspace, error) {
	return nil, &ScopeError{Resource: "workspace", ID: "(new)", Scope: p.workspace}
}

func (p *ScopedPort) ListFolders(ctx context.Context, workspaceID string, limit, page int) ([]domain.Folder, error) {
	workspaceID, err := p.workspaceOr(workspaceID)
	if err != nil {
		return nil, err
	}
	return p.Port.ListFolders(ctx, workspaceID, limit, page)
}

func (p *ScopedPort) CreateFolder(ctx context.Context, workspaceID, name string) (*domain.Folder, error) {
	workspaceID, err := p.workspaceOr(workspaceID)
	if err != nil {
		return nil, err
	}
	return p.Port.CreateFolder(ctx, workspaceID, name)
}

// ---- boards ----------------------------------------------------------------

// ListBoardsPage forces the workspace filter and drops any foreign result.
func (p *ScopedPort) ListBoardsPage(ctx context.Context, query monday.BoardQuery) ([]domain.Board, error) {
	for _, id := range query.WorkspaceIDs {
		if _, err := p.workspaceOr(id); err != nil {
			return nil, err
		}
	}
	query.WorkspaceIDs = []string{p.workspace}
	boards, err := p.Port.ListBoardsPage(ctx, query)
	if err != nil {
		return nil, err
	}
	scoped := make([]domain.Board, 0, len(boards))
	for _, board := range boards {
		if board.WorkspaceID != "" && board.WorkspaceID != p.workspace {
			continue
		}
		p.remember(board.ID, p.workspace)
		scoped = append(scoped, board)
	}
	return scoped, nil
}

func (p *ScopedPort) GetBoard(ctx context.Context, id string) (*domain.Board, error) {
	if err := p.checkBoard(ctx, id); err != nil {
		return nil, err
	}
	return p.Port.GetBoard(ctx, id)
}

func (p *ScopedPort) GetBoardSchema(ctx context.Context, id string) (*monday.BoardSchema, error) {
	if err := p.checkBoard(ctx, id); err != nil {
		return nil, err
	}
	return p.Port.GetBoardSchema(ctx, id)
}

func (p *ScopedPort) CreateBoard(ctx context.Context, input monday.CreateBoardInput) (*domain.Board, error) {
	workspaceID, err := p.workspaceOr(input.WorkspaceID)
	if err != nil {
		return nil, err
	}
	input.WorkspaceID = workspaceID
	board, err := p.Port.CreateBoard(ctx, input)
	if err == nil && board != nil {
		p.remember(board.ID, workspaceID)
	}
	return board, err
}

func (p *ScopedPort) UpdateBoard(ctx context.Context, id, attribute, value string) error {
	if err := p.checkBoard(ctx, id); err != nil {
		return err
	}
	return p.Port.UpdateBoard(ctx, id, attribute, value)
}

func (p *ScopedPort) ArchiveBoard(ctx context.Context, id string) (*domain.Board, error) {
	if err := p.checkBoard(ctx, id); err != nil {
		return nil, err
	}
	return p.Port.ArchiveBoard(ctx, id)
}

func (p *ScopedPort) DuplicateBoard(ctx context.Context, id, duplicateType, name, workspaceID, folderID string, keepSubscribers bool) (*domain.Board, error) {
	if err := p.checkBoard(ctx, id); err != nil {
		return nil, err
	}
	workspaceID, err := p.workspaceOr(workspaceID)
	if err != nil {
		return nil, err
	}
	board, err := p.Port.DuplicateBoard(ctx, id, duplicateType, name, workspaceID, folderID, keepSubscribers)
	if err == nil && board != nil {
		p.remember(board.ID, workspaceID)
	}
	return board, err
}

func (p *ScopedPort) AddUsersToBoard(ctx context.Context, boardID string, userIDs []string, kind string) ([]domain.UserRef, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.AddUsersToBoard(ctx, boardID, userIDs, kind)
}

// ---- groups & columns ------------------------------------------------------

func (p *ScopedPort) ListGroups(ctx context.Context, boardID string) ([]domain.Group, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.ListGroups(ctx, boardID)
}

func (p *ScopedPort) CreateGroup(ctx context.Context, boardID, name, color, relativeTo, method string) (*domain.Group, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.CreateGroup(ctx, boardID, name, color, relativeTo, method)
}

func (p *ScopedPort) UpdateGroup(ctx context.Context, boardID, groupID, attribute, value string) (*domain.Group, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.UpdateGroup(ctx, boardID, groupID, attribute, value)
}

func (p *ScopedPort) DuplicateGroup(ctx context.Context, boardID, groupID, title string, addToTop bool) (*domain.Group, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.DuplicateGroup(ctx, boardID, groupID, title, addToTop)
}

func (p *ScopedPort) ArchiveGroup(ctx context.Context, boardID, groupID string) (*domain.Group, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.ArchiveGroup(ctx, boardID, groupID)
}

func (p *ScopedPort) ListColumns(ctx context.Context, boardID string) ([]domain.Column, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.ListColumns(ctx, boardID)
}

func (p *ScopedPort) CreateColumn(ctx context.Context, input monday.CreateColumnInput) (*domain.Column, error) {
	if err := p.checkBoard(ctx, input.BoardID); err != nil {
		return nil, err
	}
	return p.Port.CreateColumn(ctx, input)
}

func (p *ScopedPort) ChangeColumnTitle(ctx context.Context, boardID, columnID, title string) (*domain.Column, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.ChangeColumnTitle(ctx, boardID, columnID, title)
}

func (p *ScopedPort) ChangeColumnDescription(ctx context.Context, boardID, columnID, description string) (*domain.Column, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.ChangeColumnDescription(ctx, boardID, columnID, description)
}

// ---- items -----------------------------------------------------------------

// ListItemsPage checks the board and every returned item, so a cursor from
// another board cannot leak foreign items.
func (p *ScopedPort) ListItemsPage(ctx context.Context, query monday.ItemPageQuery) (domain.ItemPage, error) {
	if err := p.checkBoard(ctx, query.BoardID); err != nil {
		return domain.ItemPage{}, err
	}
	page, err := p.Port.ListItemsPage(ctx, query)
	return p.checkPage(ctx, page, err)
}

func (p *ScopedPort) ItemsByColumnValues(ctx context.Context, boardID string, matches []monday.ColumnMatch, limit int, cursor string) (domain.ItemPage, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return domain.ItemPage{}, err
	}
	page, err := p.Port.ItemsByColumnValues(ctx, boardID, matches, limit, cursor)
	return p.checkPage(ctx, page, err)
}

func (p *ScopedPort) GetItems(ctx context.Context, ids []string) ([]domain.Item, error) {
	items, err := p.Port.GetItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	if err := p.checkItems(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (p *ScopedPort) CreateItem(ctx context.Context, boardID, groupID, name string, values map[string]any) (*domain.Item, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.CreateItem(ctx, boardID, groupID, name, values)
}

func (p *ScopedPort) CreateSubitem(ctx context.Context, parentID, name string, values map[string]any) (*domain.Item, error) {
	if err := p.checkItem(ctx, parentID); err != nil {
		return nil, err
	}
	item, err := p.Port.CreateSubitem(ctx, parentID, name, values)
	if err == nil && item != nil {
		// The subitems board hangs off an in-scope parent.
		p.remember(item.BoardID, p.workspace)
	}
	return item, err
}

func (p *ScopedPort) UpdateItemValues(ctx context.Context, boardID, itemID string, values map[string]any) (*domain.Item, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return p.Port.UpdateItemValues(ctx, boardID, itemID, values)
}

func (p *ScopedPort) MoveItem(ctx context.Context, itemID, groupID string) (*domain.Item, error) {
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return p.Port.MoveItem(ctx, itemID, groupID)
}

func (p *ScopedPort) MoveItemToBoard(ctx context.Context, itemID, boardID, groupID string) (*domain.Item, error) {
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.MoveItemToBoard(ctx, itemID, boardID, groupID)
}

func (p *ScopedPort) DuplicateItem(ctx context.Context, boardID, itemID string, withUpdates bool) (*domain.Item, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return p.Port.DuplicateItem(ctx, boardID, itemID, withUpdates)
}

func (p *ScopedPort) ArchiveItem(ctx context.Context, itemID string) (*domain.Item, error) {
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return p.Port.ArchiveItem(ctx, itemID)
}

// ---- updates & tags --------------------------------------------------------

func (p *ScopedPort) ListItemUpdates(ctx context.Context, itemID string, limit int) ([]domain.Update, error) {
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return p.Port.ListItemUpdates(ctx, itemID, limit)
}

func (p *ScopedPort) ListBoardUpdates(ctx context.Context, boardID string, limit int) ([]domain.Update, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.ListBoardUpdates(ctx, boardID, limit)
}

// CreateUpdate checks the item when given. Replies carry only the parent
// update ID; the service resolves and checks their item first.
func (p *ScopedPort) CreateUpdate(ctx context.Context, itemID, body, parentID string) (*domain.Update, error) {
	if err := p.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return p.Port.CreateUpdate(ctx, itemID, body, parentID)
}

// CreateOrGetTag checks the board when the tag is board-scoped; account-level
// public tags pass through.
func (p *ScopedPort) CreateOrGetTag(ctx context.Context, boardID, name string) (*domain.Tag, error) {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return nil, err
	}
	return p.Port.CreateOrGetTag(ctx, boardID, name)
}

// ---- permanent deletes ------------------------------------------------------

func (p *ScopedPort) DeleteItem(ctx context.Context, itemID string) error {
	if err := p.checkItem(ctx, itemID); err != nil {
		return err
	}
	return p.Port.DeleteItem(ctx, itemID)
}

func (p *ScopedPort) DeleteGroup(ctx context.Context, boardID, groupID string) error {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return err
	}
	return p.Port.DeleteGroup(ctx, boardID, groupID)
}

func (p *ScopedPort) DeleteBoard(ctx context.Context, boardID string) error {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return err
	}
	return p.Port.DeleteBoard(ctx, boardID)
}

func (p *ScopedPort) DeleteColumn(ctx context.Context, boardID, columnID string) error {
	if err := p.checkBoard(ctx, boardID); err != nil {
		return err
	}
	return p.Port.DeleteColumn(ctx, boardID, columnID)
}

// DeleteFolder only deletes folders listed in the scoped workspace.
func (p *ScopedPort) DeleteFolder(ctx context.Context, folderID string) error {
	folders, err := p.Port.ListFolders(ctx, p.workspace, 100, 1)
	if err != nil {
		return err
	}
	for _, folder := range folders {
		if folder.ID == folderID {
			return p.Port.DeleteFolder(ctx, folderID)
		}
	}
	return &ScopeError{Resource: "folder", ID: folderID, Scope: p.workspace}
}

// DeleteWorkspace is always refused: deleting the scope (or any other
// workspace) is outside a scoped server's authority.
func (p *ScopedPort) DeleteWorkspace(_ context.Context, workspaceID string) error {
	return &ScopeError{Resource: "workspace", ID: workspaceID, Scope: p.workspace}
}

// DeleteUpdate passes through: the service resolves the update's item and
// checks it (through this port) before calling.

var _ Port = (*ScopedPort)(nil)
