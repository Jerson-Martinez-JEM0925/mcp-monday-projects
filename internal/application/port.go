// Package application implements use cases on top of a monday port. It owns
// validation, write guards, dry-run planning, pagination budgets, and report
// orchestration. It never builds GraphQL.
package application

import (
	"context"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

// Port is the monday capability surface required by the use cases.
// *monday.Client implements it; tests use an in-memory fake.
type Port interface {
	Me(ctx context.Context) (*domain.Me, error)
	APIStatus(ctx context.Context) (domain.Complexity, domain.APIVersion, error)

	ListWorkspacesPage(ctx context.Context, limit, page int, kind, state string) ([]domain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name, kind, description string) (*domain.Workspace, error)
	ListFolders(ctx context.Context, workspaceID string, limit, page int) ([]domain.Folder, error)
	CreateFolder(ctx context.Context, workspaceID, name string) (*domain.Folder, error)

	ListBoardsPage(ctx context.Context, query monday.BoardQuery) ([]domain.Board, error)
	GetBoard(ctx context.Context, id string) (*domain.Board, error)
	GetBoardSchema(ctx context.Context, id string) (*monday.BoardSchema, error)
	CreateBoard(ctx context.Context, input monday.CreateBoardInput) (*domain.Board, error)
	UpdateBoard(ctx context.Context, id, attribute, value string) error
	ArchiveBoard(ctx context.Context, id string) (*domain.Board, error)
	DuplicateBoard(ctx context.Context, id, duplicateType, name, workspaceID, folderID string, keepSubscribers bool) (*domain.Board, error)
	AddUsersToBoard(ctx context.Context, boardID string, userIDs []string, kind string) ([]domain.UserRef, error)

	ListGroups(ctx context.Context, boardID string) ([]domain.Group, error)
	CreateGroup(ctx context.Context, boardID, name, color, relativeTo, method string) (*domain.Group, error)
	UpdateGroup(ctx context.Context, boardID, groupID, attribute, value string) (*domain.Group, error)
	DuplicateGroup(ctx context.Context, boardID, groupID, title string, addToTop bool) (*domain.Group, error)
	ArchiveGroup(ctx context.Context, boardID, groupID string) (*domain.Group, error)

	ListColumns(ctx context.Context, boardID string) ([]domain.Column, error)
	CreateColumn(ctx context.Context, input monday.CreateColumnInput) (*domain.Column, error)
	ChangeColumnTitle(ctx context.Context, boardID, columnID, title string) (*domain.Column, error)
	ChangeColumnDescription(ctx context.Context, boardID, columnID, description string) (*domain.Column, error)

	ListItemsPage(ctx context.Context, query monday.ItemPageQuery) (domain.ItemPage, error)
	ItemsByColumnValues(ctx context.Context, boardID string, matches []monday.ColumnMatch, limit int, cursor string) (domain.ItemPage, error)
	GetItems(ctx context.Context, ids []string) ([]domain.Item, error)
	CreateItem(ctx context.Context, boardID, groupID, name string, values map[string]any) (*domain.Item, error)
	CreateSubitem(ctx context.Context, parentID, name string, values map[string]any) (*domain.Item, error)
	UpdateItemValues(ctx context.Context, boardID, itemID string, values map[string]any) (*domain.Item, error)
	MoveItem(ctx context.Context, itemID, groupID string) (*domain.Item, error)
	MoveItemToBoard(ctx context.Context, itemID, boardID, groupID string) (*domain.Item, error)
	DuplicateItem(ctx context.Context, boardID, itemID string, withUpdates bool) (*domain.Item, error)
	ArchiveItem(ctx context.Context, itemID string) (*domain.Item, error)

	ListUsers(ctx context.Context, query monday.UserQuery) ([]domain.User, error)
	ListTeams(ctx context.Context, ids []string) ([]domain.Team, error)
	ListItemUpdates(ctx context.Context, itemID string, limit int) ([]domain.Update, error)
	ListBoardUpdates(ctx context.Context, boardID string, limit int) ([]domain.Update, error)
	CreateUpdate(ctx context.Context, itemID, body, parentID string) (*domain.Update, error)
	LikeUpdate(ctx context.Context, updateID string) error
	CreateNotification(ctx context.Context, userID, targetID, targetType, text string) error
	ListTags(ctx context.Context, ids []string) ([]domain.Tag, error)
	CreateOrGetTag(ctx context.Context, boardID, name string) (*domain.Tag, error)
}

var _ Port = (*monday.Client)(nil)
