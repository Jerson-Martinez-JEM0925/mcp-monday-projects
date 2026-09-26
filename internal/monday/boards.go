package monday

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
)

// Provider-neutral types re-exported for adapter callers and existing tests.
type (
	Workspace = domain.Workspace
	Board     = domain.Board
	Folder    = domain.Folder
)

const boardFields = `id name description state board_kind workspace_id items_count url updated_at`

const listWorkspacesQuery = `query ListWorkspaces($limit: Int!, $page: Int, $kind: WorkspaceKind, $state: State) {
  workspaces(limit: $limit, page: $page, kind: $kind, state: $state) { id name kind description state }
}`

const getWorkspaceQuery = `query GetWorkspace($id: ID!) {
  workspaces(ids: [$id]) { id name kind description state }
}`

const createWorkspaceMutation = `mutation CreateWorkspace($name: String!, $kind: WorkspaceKind!, $description: String) {
  create_workspace(name: $name, kind: $kind, description: $description) { id name kind description state }
}`

const listFoldersQuery = `query ListFolders($workspaceIDs: [ID], $limit: Int!, $page: Int) {
  folders(workspace_ids: $workspaceIDs, limit: $limit, page: $page) { id name color workspace { id name } }
}`

const createFolderMutation = `mutation CreateFolder($workspaceID: ID, $name: String!) {
  create_folder(workspace_id: $workspaceID, name: $name) { id name color workspace { id name } }
}`

const listBoardsQuery = `query ListBoards($limit: Int!, $page: Int, $workspaceIDs: [ID], $state: State, $kind: BoardKind, $order: BoardsOrderBy) {
  boards(limit: $limit, page: $page, workspace_ids: $workspaceIDs, state: $state, board_kind: $kind, order_by: $order) { ` + boardFields + ` }
}`

const getBoardQuery = `query GetBoard($id: ID!) {
  boards(ids: [$id]) { ` + boardFields + ` }
}`

const getBoardSchemaQuery = `query GetBoardSchema($id: ID!) {
  boards(ids: [$id]) {
    ` + boardFields + `
    columns { id title type description archived settings }
    groups { id title color position archived }
    owners { id name }
    subscribers { id name }
    tags { id name color }
  }
}`

const createBoardMutation = `mutation CreateBoard($name: String!, $kind: BoardKind!, $workspaceID: ID, $folderID: ID, $description: String, $empty: Boolean, $ownerIDs: [ID!], $subscriberIDs: [ID!], $templateID: ID) {
  create_board(board_name: $name, board_kind: $kind, workspace_id: $workspaceID, folder_id: $folderID, description: $description, empty: $empty, board_owner_ids: $ownerIDs, board_subscriber_ids: $subscriberIDs, template_id: $templateID) { ` + boardFields + ` }
}`

const updateBoardMutation = `mutation UpdateBoard($id: ID!, $attribute: BoardAttributes!, $value: String!) {
  update_board(board_id: $id, board_attribute: $attribute, new_value: $value)
}`

const archiveBoardMutation = `mutation ArchiveBoard($id: ID!) {
  archive_board(board_id: $id) { ` + boardFields + ` }
}`

const duplicateBoardMutation = `mutation DuplicateBoard($id: ID!, $type: DuplicateBoardType!, $name: String, $workspaceID: ID, $folderID: ID, $keepSubscribers: Boolean) {
  duplicate_board(board_id: $id, duplicate_type: $type, board_name: $name, workspace_id: $workspaceID, folder_id: $folderID, keep_subscribers: $keepSubscribers) { board { ` + boardFields + ` } }
}`

const addUsersToBoardMutation = `mutation AddUsersToBoard($id: ID!, $userIDs: [ID!]!, $kind: BoardSubscriberKind) {
  add_users_to_board(board_id: $id, user_ids: $userIDs, kind: $kind) { id name }
}`

// BoardQuery filters board listings.
type BoardQuery struct {
	Limit        int
	Page         int
	WorkspaceIDs []string
	State        string
	Kind         string
	OrderBy      string
}

// BoardSchema is a board with its full structure.
type BoardSchema struct {
	domain.Board
	Columns     []domain.Column  `json:"columns"`
	Groups      []domain.Group   `json:"groups"`
	Owners      []domain.UserRef `json:"owners,omitempty"`
	Subscribers []domain.UserRef `json:"subscribers,omitempty"`
	Tags        []domain.Tag     `json:"tags,omitempty"`
}

// CreateBoardInput describes a new board.
type CreateBoardInput struct {
	Name          string
	Kind          string
	WorkspaceID   string
	FolderID      string
	Description   string
	Empty         bool
	OwnerIDs      []string
	SubscriberIDs []string
	TemplateID    string
}

func optional(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalList(values []string) any {
	if len(values) == 0 {
		return nil
	}
	return values
}

func optionalInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

// ListWorkspaces returns visible workspaces up to the requested limit.
func (c *Client) ListWorkspaces(ctx context.Context, limit int) ([]Workspace, error) {
	return c.ListWorkspacesPage(ctx, limit, 0, "", "")
}

// ListWorkspacesPage returns one page of workspaces with optional filters.
func (c *Client) ListWorkspacesPage(ctx context.Context, limit, page int, kind, state string) ([]Workspace, error) {
	var data struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	vars := map[string]any{"limit": limit, "page": optionalInt(page), "kind": optional(kind), "state": optional(state)}
	if err := c.Do(ctx, listWorkspacesQuery, vars, &data); err != nil {
		return nil, err
	}
	return data.Workspaces, nil
}

// GetWorkspace returns one workspace.
func (c *Client) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	var data struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	if err := c.Do(ctx, getWorkspaceQuery, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	if len(data.Workspaces) == 0 || data.Workspaces[0].ID == "" {
		return nil, &NotFoundError{Resource: "workspace", ID: id}
	}
	return &data.Workspaces[0], nil
}

// CreateWorkspace creates a workspace.
func (c *Client) CreateWorkspace(ctx context.Context, name, kind, description string) (*Workspace, error) {
	var data struct {
		Workspace Workspace `json:"create_workspace"`
	}
	vars := map[string]any{"name": name, "kind": kind, "description": optional(description)}
	if err := c.Do(ctx, createWorkspaceMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Workspace, nil
}

// ListFolders lists folders, optionally for one workspace.
func (c *Client) ListFolders(ctx context.Context, workspaceID string, limit, page int) ([]Folder, error) {
	var data struct {
		Folders []Folder `json:"folders"`
	}
	vars := map[string]any{"limit": limit, "page": optionalInt(page)}
	if workspaceID != "" {
		vars["workspaceIDs"] = []string{workspaceID}
	}
	if err := c.Do(ctx, listFoldersQuery, vars, &data); err != nil {
		return nil, err
	}
	return data.Folders, nil
}

// CreateFolder creates a folder inside a workspace.
func (c *Client) CreateFolder(ctx context.Context, workspaceID, name string) (*Folder, error) {
	var data struct {
		Folder Folder `json:"create_folder"`
	}
	if err := c.Do(ctx, createFolderMutation, map[string]any{"workspaceID": optional(workspaceID), "name": name}, &data); err != nil {
		return nil, err
	}
	return &data.Folder, nil
}

// ListBoards returns visible boards up to the requested limit.
func (c *Client) ListBoards(ctx context.Context, limit int) ([]Board, error) {
	return c.ListBoardsPage(ctx, BoardQuery{Limit: limit})
}

// ListBoardsPage returns one page of boards with filters.
func (c *Client) ListBoardsPage(ctx context.Context, query BoardQuery) ([]Board, error) {
	var data struct {
		Boards []Board `json:"boards"`
	}
	vars := map[string]any{
		"limit": query.Limit, "page": optionalInt(query.Page), "workspaceIDs": optionalList(query.WorkspaceIDs),
		"state": optional(query.State), "kind": optional(query.Kind), "order": optional(query.OrderBy),
	}
	if err := c.Do(ctx, listBoardsQuery, vars, &data); err != nil {
		return nil, err
	}
	return data.Boards, nil
}

// GetBoard returns a board by its Monday identifier.
func (c *Client) GetBoard(ctx context.Context, id string) (*Board, error) {
	var data struct {
		Boards []Board `json:"boards"`
	}
	if err := c.Do(ctx, getBoardQuery, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	if len(data.Boards) == 0 {
		return nil, &NotFoundError{Resource: "board", ID: id}
	}
	return &data.Boards[0], nil
}

// GetBoardSchema returns a board with its columns, groups, people, and tags.
func (c *Client) GetBoardSchema(ctx context.Context, id string) (*BoardSchema, error) {
	var data struct {
		Boards []BoardSchema `json:"boards"`
	}
	if err := c.Do(ctx, getBoardSchemaQuery, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	if len(data.Boards) == 0 {
		return nil, &NotFoundError{Resource: "board", ID: id}
	}
	return &data.Boards[0], nil
}

// CreateBoard creates a board.
func (c *Client) CreateBoard(ctx context.Context, input CreateBoardInput) (*Board, error) {
	var data struct {
		Board Board `json:"create_board"`
	}
	vars := map[string]any{
		"name": input.Name, "kind": input.Kind, "workspaceID": optional(input.WorkspaceID), "folderID": optional(input.FolderID),
		"description": optional(input.Description), "empty": input.Empty, "ownerIDs": optionalList(input.OwnerIDs),
		"subscriberIDs": optionalList(input.SubscriberIDs), "templateID": optional(input.TemplateID),
	}
	if err := c.Do(ctx, createBoardMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Board, nil
}

// UpdateBoard changes one board attribute (name, description, communication).
func (c *Client) UpdateBoard(ctx context.Context, id, attribute, value string) error {
	var data struct {
		Result json.RawMessage `json:"update_board"`
	}
	if err := c.Do(ctx, updateBoardMutation, map[string]any{"id": id, "attribute": attribute, "value": value}, &data); err != nil {
		return err
	}
	return decodeMutationResult(data.Result)
}

// decodeMutationResult inspects monday's JSON mutation results for failures.
func decodeMutationResult(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		raw = json.RawMessage(text)
	}
	var result struct {
		Success *bool  `json:"success"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(raw, &result) == nil && result.Success != nil && !*result.Success {
		return fmt.Errorf("monday rejected the mutation: %s", result.Error)
	}
	return nil
}

// ArchiveBoard archives (never deletes) a board.
func (c *Client) ArchiveBoard(ctx context.Context, id string) (*Board, error) {
	var data struct {
		Board Board `json:"archive_board"`
	}
	if err := c.Do(ctx, archiveBoardMutation, map[string]any{"id": id}, &data); err != nil {
		return nil, err
	}
	return &data.Board, nil
}

// DuplicateBoard copies a board structure (optionally with items/updates).
func (c *Client) DuplicateBoard(ctx context.Context, id, duplicateType, name, workspaceID, folderID string, keepSubscribers bool) (*Board, error) {
	var data struct {
		Result struct {
			Board Board `json:"board"`
		} `json:"duplicate_board"`
	}
	vars := map[string]any{"id": id, "type": duplicateType, "name": optional(name), "workspaceID": optional(workspaceID), "folderID": optional(folderID), "keepSubscribers": keepSubscribers}
	if err := c.Do(ctx, duplicateBoardMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Result.Board, nil
}

// AddUsersToBoard subscribes users to a board as subscribers or owners.
func (c *Client) AddUsersToBoard(ctx context.Context, boardID string, userIDs []string, kind string) ([]domain.UserRef, error) {
	var data struct {
		Users []domain.UserRef `json:"add_users_to_board"`
	}
	if err := c.Do(ctx, addUsersToBoardMutation, map[string]any{"id": boardID, "userIDs": userIDs, "kind": optional(kind)}, &data); err != nil {
		return nil, err
	}
	return data.Users, nil
}
