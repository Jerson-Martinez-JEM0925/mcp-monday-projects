package mcpserver

import (
	"context"
	"fmt"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

const defaultListLimit = 25

func wrap(op string, err error) error { return fmt.Errorf("%s: %w", op, err) }

// ---- inputs & outputs --------------------------------------------------------

// ListWorkspacesInput paginates workspaces.
type ListWorkspacesInput struct {
	Limit int    `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
	Page  int    `json:"page,omitempty" jsonschema:"1-based page number"`
	Kind  string `json:"kind,omitempty" jsonschema:"open, closed, or template"`
	State string `json:"state,omitempty" jsonschema:"active (default), archived, deleted, or all"`
}

// ListWorkspacesOutput is the stable MCP response for workspace discovery.
type ListWorkspacesOutput struct {
	Workspaces []monday.Workspace `json:"workspaces"`
	Count      int                `json:"count"`
}

// WorkspaceIDInput identifies a workspace.
type WorkspaceIDInput struct {
	WorkspaceID string `json:"workspace_id" jsonschema:"monday workspace identifier"`
}

// WorkspaceOutput wraps one workspace.
type WorkspaceOutput struct {
	Workspace monday.Workspace `json:"workspace"`
}

// CreateWorkspaceInput creates a workspace.
type CreateWorkspaceInput struct {
	Name        string `json:"name" jsonschema:"workspace name"`
	Kind        string `json:"kind,omitempty" jsonschema:"open (default) or closed"`
	Description string `json:"description,omitempty" jsonschema:"optional description"`
}

// ListFoldersInput lists folders.
type ListFoldersInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"optional workspace filter"`
	Limit       int    `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

// FoldersOutput lists folders.
type FoldersOutput struct {
	Folders []domain.Folder `json:"folders"`
	Count   int             `json:"count"`
}

// CreateFolderInput creates a folder.
type CreateFolderInput struct {
	WorkspaceID string `json:"workspace_id" jsonschema:"workspace that will own the folder"`
	Name        string `json:"name" jsonschema:"folder name"`
}

// FolderOutput wraps one folder.
type FolderOutput struct {
	Folder domain.Folder `json:"folder"`
}

// ListBoardsInput filters boards.
type ListBoardsInput struct {
	Limit        int      `json:"limit,omitempty" jsonschema:"page size 1-100 (default 25)"`
	Page         int      `json:"page,omitempty" jsonschema:"1-based page number"`
	WorkspaceIDs []string `json:"workspace_ids,omitempty" jsonschema:"only boards in these workspaces"`
	State        string   `json:"state,omitempty" jsonschema:"active (default), archived, deleted, or all"`
	BoardKind    string   `json:"board_kind,omitempty" jsonschema:"public, private, or share"`
	OrderBy      string   `json:"order_by,omitempty" jsonschema:"created_at or used_at"`
}

// ListBoardsOutput is the stable MCP response for board discovery.
type ListBoardsOutput struct {
	Boards []monday.Board `json:"boards"`
	Count  int            `json:"count"`
}

// GetBoardInput identifies one board.
type GetBoardInput struct {
	BoardID string `json:"board_id" jsonschema:"monday board identifier"`
}

// BoardIDInput identifies one board.
type BoardIDInput = GetBoardInput

// GetBoardOutput is the stable MCP response for one board.
type GetBoardOutput struct {
	Board monday.Board `json:"board"`
}

// BoardSchemaOutput returns a board structure.
type BoardSchemaOutput struct {
	Schema        monday.BoardSchema   `json:"schema"`
	ReportColumns domain.ReportColumns `json:"detected_report_columns"`
}

// CreateBoardInput creates a board.
type CreateBoardInput struct {
	Name        string `json:"name" jsonschema:"board name"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"workspace that will own the board"`
	FolderID    string `json:"folder_id,omitempty" jsonschema:"optional folder"`
	BoardKind   string `json:"board_kind,omitempty" jsonschema:"public (default), private, or share"`
	Description string `json:"description,omitempty" jsonschema:"optional description"`
	Empty       bool   `json:"empty,omitempty" jsonschema:"create without monday's sample columns and items"`
}

// UpdateBoardInput changes a board attribute.
type UpdateBoardInput struct {
	BoardID   string `json:"board_id" jsonschema:"monday board identifier"`
	Attribute string `json:"attribute" jsonschema:"name or description"`
	Value     string `json:"value" jsonschema:"new value"`
}

// OKOutput acknowledges a mutation without a payload.
type OKOutput struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// ArchiveBoardInput archives a board.
type ArchiveBoardInput struct {
	BoardID string `json:"board_id" jsonschema:"monday board identifier"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true; archived boards can be restored from monday"`
}

// DuplicateBoardInput duplicates a board.
type DuplicateBoardInput struct {
	BoardID         string `json:"board_id" jsonschema:"source board"`
	Name            string `json:"name,omitempty" jsonschema:"name for the copy"`
	DuplicateType   string `json:"duplicate_type,omitempty" jsonschema:"duplicate_board_with_structure (default), duplicate_board_with_pulses, duplicate_board_with_pulses_and_updates"`
	WorkspaceID     string `json:"workspace_id,omitempty" jsonschema:"destination workspace"`
	FolderID        string `json:"folder_id,omitempty" jsonschema:"destination folder"`
	KeepSubscribers bool   `json:"keep_subscribers,omitempty" jsonschema:"copy subscribers too"`
}

// AddSubscribersInput subscribes users.
type AddSubscribersInput struct {
	BoardID string   `json:"board_id" jsonschema:"monday board identifier"`
	UserIDs []string `json:"user_ids" jsonschema:"user IDs to add (max 50)"`
	Kind    string   `json:"kind,omitempty" jsonschema:"subscriber (default) or owner"`
}

// UsersRefOutput lists user refs.
type UsersRefOutput struct {
	Users []domain.UserRef `json:"users"`
	Count int              `json:"count"`
}

// TemplatesOutput lists board templates.
type TemplatesOutput struct {
	Templates []domain.BoardTemplate `json:"templates"`
}

// ProvisionInput creates a board from a template.
type ProvisionInput struct {
	Template    string                `json:"template" jsonschema:"template key: devops, incident, release, or project"`
	Name        string                `json:"name,omitempty" jsonschema:"board name (defaults to the template title)"`
	WorkspaceID string                `json:"workspace_id,omitempty" jsonschema:"workspace that will own the board"`
	BoardKind   string                `json:"board_kind,omitempty" jsonschema:"public (default), private, or share"`
	Description string                `json:"description,omitempty" jsonschema:"board description"`
	SeedItems   []domain.TemplateItem `json:"seed_items,omitempty" jsonschema:"optional items to create; values use friendly column formats"`
}

// ProvisionOutput reports a provisioned board.
type ProvisionOutput struct {
	Result application.ProvisionResult `json:"result"`
}

// GroupsOutput lists groups.
type GroupsOutput struct {
	Groups []monday.Group `json:"groups"`
	Count  int            `json:"count"`
}

// GroupOutput wraps one group.
type GroupOutput struct {
	Group monday.Group `json:"group"`
}

// CreateGroupInput creates a group.
type CreateGroupInput struct {
	BoardID                string `json:"board_id" jsonschema:"monday board identifier"`
	Name                   string `json:"name" jsonschema:"group title"`
	Color                  string `json:"color,omitempty" jsonschema:"optional hex color like #00c875"`
	RelativeTo             string `json:"relative_to,omitempty" jsonschema:"group ID to position against"`
	PositionRelativeMethod string `json:"position_relative_method,omitempty" jsonschema:"before_at or after_at"`
}

// UpdateGroupInput changes a group attribute.
type UpdateGroupInput struct {
	BoardID   string `json:"board_id" jsonschema:"monday board identifier"`
	GroupID   string `json:"group_id" jsonschema:"group identifier"`
	Attribute string `json:"attribute" jsonschema:"title, color, position, relative_position_after, or relative_position_before"`
	Value     string `json:"value" jsonschema:"new value"`
}

// DuplicateGroupInput duplicates a group.
type DuplicateGroupInput struct {
	BoardID  string `json:"board_id" jsonschema:"monday board identifier"`
	GroupID  string `json:"group_id" jsonschema:"group to copy"`
	Title    string `json:"title,omitempty" jsonschema:"title for the copy"`
	AddToTop bool   `json:"add_to_top,omitempty" jsonschema:"place the copy at the top"`
}

// ArchiveGroupInput archives a group.
type ArchiveGroupInput struct {
	BoardID string `json:"board_id" jsonschema:"monday board identifier"`
	GroupID string `json:"group_id" jsonschema:"group identifier"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true; archives the group and its items"`
}

// ColumnsOutput lists columns.
type ColumnsOutput struct {
	Columns []monday.Column `json:"columns"`
	Count   int             `json:"count"`
}

// ColumnOutput wraps one column.
type ColumnOutput struct {
	Column monday.Column `json:"column"`
}

// CreateColumnInput creates a column.
type CreateColumnInput struct {
	BoardID       string   `json:"board_id" jsonschema:"monday board identifier"`
	Title         string   `json:"title" jsonschema:"column title"`
	ColumnType    string   `json:"column_type" jsonschema:"monday column type, e.g. status, text, numbers, date, people, dropdown, link, timeline"`
	ColumnID      string   `json:"column_id,omitempty" jsonschema:"optional stable ID (lowercase, max 20 chars)"`
	Description   string   `json:"description,omitempty" jsonschema:"optional description"`
	AfterColumnID string   `json:"after_column_id,omitempty" jsonschema:"insert after this column"`
	Labels        []string `json:"labels,omitempty" jsonschema:"labels for status/dropdown columns"`
}

// UpdateColumnInput renames or describes a column.
type UpdateColumnInput struct {
	BoardID     string `json:"board_id" jsonschema:"monday board identifier"`
	ColumnID    string `json:"column_id" jsonschema:"column identifier"`
	Title       string `json:"title,omitempty" jsonschema:"new title"`
	Description string `json:"description,omitempty" jsonschema:"new description"`
}

// FormatsOutput documents column value formats.
type FormatsOutput struct {
	Formats []domain.ColumnFormat `json:"formats"`
}

// ---- registration ------------------------------------------------------------

func registerStructureTools(r *registry) {
	svc := r.svc
	// Workspaces & folders
	add(r, ToolSpec{Name: "list_workspaces", Category: CatWorkspaces, Title: "List workspaces", ReadOnly: true,
		Description: "List visible monday.com workspaces with pagination and kind/state filters."},
		func(ctx context.Context, in ListWorkspacesInput) (ListWorkspacesOutput, error) {
			list, err := svc.ListWorkspaces(ctx, in.Limit, in.Page, in.Kind, in.State)
			if err != nil {
				return ListWorkspacesOutput{}, wrap("list workspaces", err)
			}
			return ListWorkspacesOutput{Workspaces: list, Count: len(list)}, nil
		})
	add(r, ToolSpec{Name: "get_workspace", Category: CatWorkspaces, Title: "Get workspace", ReadOnly: true,
		Description: "Get one workspace by ID."},
		func(ctx context.Context, in WorkspaceIDInput) (WorkspaceOutput, error) {
			workspace, err := svc.GetWorkspace(ctx, in.WorkspaceID)
			if err != nil {
				return WorkspaceOutput{}, wrap("get workspace", err)
			}
			return WorkspaceOutput{Workspace: *workspace}, nil
		})
	add(r, ToolSpec{Name: "create_workspace", Category: CatWorkspaces, Title: "Create workspace",
		Description: "Create a workspace. Refused while a write allowlist is configured."},
		func(ctx context.Context, in CreateWorkspaceInput) (WorkspaceOutput, error) {
			workspace, err := svc.CreateWorkspace(ctx, in.Name, in.Kind, in.Description)
			if err != nil {
				return WorkspaceOutput{}, wrap("create workspace", err)
			}
			return WorkspaceOutput{Workspace: *workspace}, nil
		})
	add(r, ToolSpec{Name: "list_folders", Category: CatWorkspaces, Title: "List folders", ReadOnly: true,
		Description: "List folders, optionally within one workspace."},
		func(ctx context.Context, in ListFoldersInput) (FoldersOutput, error) {
			list, err := svc.ListFolders(ctx, in.WorkspaceID, in.Limit, in.Page)
			if err != nil {
				return FoldersOutput{}, wrap("list folders", err)
			}
			return FoldersOutput{Folders: list, Count: len(list)}, nil
		})
	add(r, ToolSpec{Name: "create_folder", Category: CatWorkspaces, Title: "Create folder",
		Description: "Create a folder in a workspace (workspace must be allowed by the write policy)."},
		func(ctx context.Context, in CreateFolderInput) (FolderOutput, error) {
			folder, err := svc.CreateFolder(ctx, in.WorkspaceID, in.Name)
			if err != nil {
				return FolderOutput{}, wrap("create folder", err)
			}
			return FolderOutput{Folder: *folder}, nil
		})

	// Boards
	add(r, ToolSpec{Name: "list_boards", Category: CatBoards, Title: "List boards", ReadOnly: true,
		Description: "List boards with pagination and workspace/state/kind filters, including item counts and URLs."},
		func(ctx context.Context, in ListBoardsInput) (ListBoardsOutput, error) {
			list, err := svc.ListBoards(ctx, monday.BoardQuery{Limit: in.Limit, Page: in.Page, WorkspaceIDs: in.WorkspaceIDs, State: in.State, Kind: in.BoardKind, OrderBy: in.OrderBy})
			if err != nil {
				return ListBoardsOutput{}, wrap("list boards", err)
			}
			return ListBoardsOutput{Boards: list, Count: len(list)}, nil
		})
	add(r, ToolSpec{Name: "get_board", Category: CatBoards, Title: "Get board", ReadOnly: true,
		Description: "Get one monday.com board by ID."},
		func(ctx context.Context, in GetBoardInput) (GetBoardOutput, error) {
			board, err := svc.GetBoard(ctx, in.BoardID)
			if err != nil {
				return GetBoardOutput{}, wrap("get board", err)
			}
			return GetBoardOutput{Board: *board}, nil
		})
	add(r, ToolSpec{Name: "get_board_schema", Category: CatBoards, Title: "Get board schema", ReadOnly: true,
		Description: "Get a board's full structure: columns (with status/dropdown labels), groups, owners, subscribers, and tags, plus the status/date/people columns reports will use."},
		func(ctx context.Context, in GetBoardInput) (BoardSchemaOutput, error) {
			schema, err := svc.GetBoardSchema(ctx, in.BoardID)
			if err != nil {
				return BoardSchemaOutput{}, wrap("get board schema", err)
			}
			return BoardSchemaOutput{Schema: *schema, ReportColumns: domain.DetectReportColumns(schema.Columns, domain.ReportColumns{})}, nil
		})
	add(r, ToolSpec{Name: "create_board", Category: CatBoards, Title: "Create board",
		Description: "Create a board in a workspace. Boards created here become writable for this server session even under an allowlist."},
		func(ctx context.Context, in CreateBoardInput) (GetBoardOutput, error) {
			board, err := svc.CreateBoard(ctx, monday.CreateBoardInput{Name: in.Name, Kind: in.BoardKind, WorkspaceID: in.WorkspaceID, FolderID: in.FolderID, Description: in.Description, Empty: in.Empty})
			if err != nil {
				return GetBoardOutput{}, wrap("create board", err)
			}
			return GetBoardOutput{Board: *board}, nil
		})
	add(r, ToolSpec{Name: "update_board", Category: CatBoards, Title: "Update board",
		Description: "Rename a board or change its description."},
		func(ctx context.Context, in UpdateBoardInput) (OKOutput, error) {
			if err := svc.UpdateBoard(ctx, in.BoardID, in.Attribute, in.Value); err != nil {
				return OKOutput{}, wrap("update board", err)
			}
			return OKOutput{OK: true, Message: in.Attribute + " updated"}, nil
		})
	add(r, ToolSpec{Name: "archive_board", Category: CatBoards, Title: "Archive board", Destructive: true,
		Description: "Archive (never delete) a board. Requires confirm=true."},
		func(ctx context.Context, in ArchiveBoardInput) (GetBoardOutput, error) {
			board, err := svc.ArchiveBoard(ctx, in.BoardID, in.Confirm)
			if err != nil {
				return GetBoardOutput{}, wrap("archive board", err)
			}
			return GetBoardOutput{Board: *board}, nil
		})
	add(r, ToolSpec{Name: "duplicate_board", Category: CatBoards, Title: "Duplicate board",
		Description: "Duplicate a board's structure, optionally with items and updates, into a workspace/folder."},
		func(ctx context.Context, in DuplicateBoardInput) (GetBoardOutput, error) {
			board, err := svc.DuplicateBoard(ctx, in.BoardID, in.DuplicateType, in.Name, in.WorkspaceID, in.FolderID, in.KeepSubscribers)
			if err != nil {
				return GetBoardOutput{}, wrap("duplicate board", err)
			}
			return GetBoardOutput{Board: *board}, nil
		})
	add(r, ToolSpec{Name: "add_board_subscribers", Category: CatBoards, Title: "Add board subscribers",
		Description: "Subscribe users to a board as subscribers or owners."},
		func(ctx context.Context, in AddSubscribersInput) (UsersRefOutput, error) {
			users, err := svc.AddBoardSubscribers(ctx, in.BoardID, in.UserIDs, in.Kind)
			if err != nil {
				return UsersRefOutput{}, wrap("add board subscribers", err)
			}
			return UsersRefOutput{Users: users, Count: len(users)}, nil
		})
	add(r, ToolSpec{Name: "list_board_templates", Category: CatBoards, Title: "List board templates", ReadOnly: true, Capability: "boards.read",
		Description: "List the built-in board blueprints (devops, incident, release, project) with their groups and typed columns."},
		func(context.Context, struct{}) (TemplatesOutput, error) {
			return TemplatesOutput{Templates: domain.BoardTemplates()}, nil
		})
	add(r, ToolSpec{Name: "provision_board_from_template", Category: CatBoards, Title: "Provision board from template",
		Description: "Create a ready-to-use board from a template in one call: typed columns with labels, ordered groups, and optional validated seed items. Additive only; nothing is deleted."},
		func(ctx context.Context, in ProvisionInput) (ProvisionOutput, error) {
			result, err := svc.ProvisionBoard(ctx, in.Template, in.Name, in.WorkspaceID, in.BoardKind, in.Description, in.SeedItems)
			if err != nil {
				return ProvisionOutput{}, wrap("provision board", err)
			}
			return ProvisionOutput{Result: *result}, nil
		})

	// Groups
	add(r, ToolSpec{Name: "list_board_groups", Category: CatGroups, Title: "List groups", ReadOnly: true,
		Description: "List groups in a monday.com board."},
		func(ctx context.Context, in BoardIDInput) (GroupsOutput, error) {
			groups, err := svc.ListGroups(ctx, in.BoardID)
			if err != nil {
				return GroupsOutput{}, wrap("list groups", err)
			}
			return GroupsOutput{Groups: groups, Count: len(groups)}, nil
		})
	add(r, ToolSpec{Name: "create_group", Category: CatGroups, Title: "Create group",
		Description: "Create a group, optionally positioned before/after another group."},
		func(ctx context.Context, in CreateGroupInput) (GroupOutput, error) {
			group, err := svc.CreateGroup(ctx, in.BoardID, in.Name, in.Color, in.RelativeTo, in.PositionRelativeMethod)
			if err != nil {
				return GroupOutput{}, wrap("create group", err)
			}
			return GroupOutput{Group: *group}, nil
		})
	add(r, ToolSpec{Name: "update_group", Category: CatGroups, Title: "Update group",
		Description: "Change a group's title, color, or position."},
		func(ctx context.Context, in UpdateGroupInput) (GroupOutput, error) {
			group, err := svc.UpdateGroup(ctx, in.BoardID, in.GroupID, in.Attribute, in.Value)
			if err != nil {
				return GroupOutput{}, wrap("update group", err)
			}
			return GroupOutput{Group: *group}, nil
		})
	add(r, ToolSpec{Name: "duplicate_group", Category: CatGroups, Title: "Duplicate group",
		Description: "Duplicate a group together with its items."},
		func(ctx context.Context, in DuplicateGroupInput) (GroupOutput, error) {
			group, err := svc.DuplicateGroup(ctx, in.BoardID, in.GroupID, in.Title, in.AddToTop)
			if err != nil {
				return GroupOutput{}, wrap("duplicate group", err)
			}
			return GroupOutput{Group: *group}, nil
		})
	add(r, ToolSpec{Name: "archive_group", Category: CatGroups, Title: "Archive group", Destructive: true,
		Description: "Archive (never delete) a group and its items. Requires confirm=true."},
		func(ctx context.Context, in ArchiveGroupInput) (GroupOutput, error) {
			group, err := svc.ArchiveGroup(ctx, in.BoardID, in.GroupID, in.Confirm)
			if err != nil {
				return GroupOutput{}, wrap("archive group", err)
			}
			return GroupOutput{Group: *group}, nil
		})

	// Columns
	add(r, ToolSpec{Name: "list_board_columns", Category: CatColumns, Title: "List columns", ReadOnly: true,
		Description: "List columns in a monday.com board, including settings such as status labels."},
		func(ctx context.Context, in BoardIDInput) (ColumnsOutput, error) {
			columns, err := svc.ListColumns(ctx, in.BoardID)
			if err != nil {
				return ColumnsOutput{}, wrap("list columns", err)
			}
			return ColumnsOutput{Columns: columns, Count: len(columns)}, nil
		})
	add(r, ToolSpec{Name: "create_column", Category: CatColumns, Title: "Create column",
		Description: "Create a typed column; status and dropdown columns accept initial labels."},
		func(ctx context.Context, in CreateColumnInput) (ColumnOutput, error) {
			column, err := svc.CreateColumn(ctx, in.BoardID, in.ColumnID, in.Title, in.ColumnType, in.Description, in.AfterColumnID, in.Labels)
			if err != nil {
				return ColumnOutput{}, wrap("create column", err)
			}
			return ColumnOutput{Column: *column}, nil
		})
	add(r, ToolSpec{Name: "update_column", Category: CatColumns, Title: "Update column",
		Description: "Rename a column and/or change its description."},
		func(ctx context.Context, in UpdateColumnInput) (ColumnOutput, error) {
			column, err := svc.ChangeColumn(ctx, in.BoardID, in.ColumnID, in.Title, in.Description)
			if err != nil {
				return ColumnOutput{}, wrap("update column", err)
			}
			return ColumnOutput{Column: *column}, nil
		})
	add(r, ToolSpec{Name: "describe_column_formats", Category: CatColumns, Title: "Column value formats", ReadOnly: true,
		Description: "Describe the friendly value formats accepted per column type by every write tool, and which types are read-only."},
		func(context.Context, struct{}) (FormatsOutput, error) {
			return FormatsOutput{Formats: domain.ColumnFormats()}, nil
		})
}
