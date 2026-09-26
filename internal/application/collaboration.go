package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

func decodeSettings(settings domain.JSONObject, target any) error {
	if len(settings) == 0 {
		return fmt.Errorf("empty settings")
	}
	raw, err := json.Marshal(map[string]any(settings))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// ---- users & teams ---------------------------------------------------------

// ListUsers lists or searches the user directory.
func (s *Service) ListUsers(ctx context.Context, query monday.UserQuery) ([]domain.User, error) {
	limit, err := BoundLimit(query.Limit, 50, 200)
	if err != nil {
		return nil, err
	}
	query.Limit = limit
	for _, id := range query.IDs {
		if err := requireID("ids", id); err != nil {
			return nil, err
		}
	}
	return s.port.ListUsers(ctx, query)
}

// GetUser returns one user.
func (s *Service) GetUser(ctx context.Context, id string) (*domain.User, error) {
	if err := requireID("user_id", id); err != nil {
		return nil, err
	}
	users, err := s.port.ListUsers(ctx, monday.UserQuery{IDs: []string{id}, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, &monday.NotFoundError{Resource: "user", ID: id}
	}
	return &users[0], nil
}

// ListTeams lists teams with members.
func (s *Service) ListTeams(ctx context.Context, ids []string) ([]domain.Team, error) {
	for _, id := range ids {
		if err := requireID("team_ids", id); err != nil {
			return nil, err
		}
	}
	return s.port.ListTeams(ctx, ids)
}

// ---- updates & notifications -----------------------------------------------

// ListItemUpdates lists an item's updates.
func (s *Service) ListItemUpdates(ctx context.Context, itemID string, limit int) ([]domain.Update, error) {
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	limit, err := BoundLimit(limit, 25, 100)
	if err != nil {
		return nil, err
	}
	return s.port.ListItemUpdates(ctx, itemID, limit)
}

// ListBoardUpdates lists recent updates across a board.
func (s *Service) ListBoardUpdates(ctx context.Context, boardID string, limit int) ([]domain.Update, error) {
	if err := requireID("board_id", boardID); err != nil {
		return nil, err
	}
	limit, err := BoundLimit(limit, 25, 100)
	if err != nil {
		return nil, err
	}
	return s.port.ListBoardUpdates(ctx, boardID, limit)
}

const maxUpdateBody = 20000

// CreateUpdate posts an update on an item.
func (s *Service) CreateUpdate(ctx context.Context, itemID, body string) (*domain.Update, error) {
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if err := requireText("body", body); err != nil {
		return nil, err
	}
	if len(body) > maxUpdateBody {
		return nil, invalid("body exceeds %d characters", maxUpdateBody)
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.port.CreateUpdate(ctx, itemID, body, "")
}

// ReplyToUpdate replies to an update. itemID is needed for guard checks.
func (s *Service) ReplyToUpdate(ctx context.Context, itemID, updateID, body string) (*domain.Update, error) {
	if err := requireID("item_id", itemID); err != nil {
		return nil, err
	}
	if err := requireID("update_id", updateID); err != nil {
		return nil, err
	}
	if err := requireText("body", body); err != nil {
		return nil, err
	}
	if len(body) > maxUpdateBody {
		return nil, invalid("body exceeds %d characters", maxUpdateBody)
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.port.CreateUpdate(ctx, "", body, updateID)
}

// LikeUpdate likes an update.
func (s *Service) LikeUpdate(ctx context.Context, itemID, updateID string) error {
	if err := requireID("item_id", itemID); err != nil {
		return err
	}
	if err := requireID("update_id", updateID); err != nil {
		return err
	}
	if _, err := s.checkItem(ctx, itemID); err != nil {
		return err
	}
	return s.port.LikeUpdate(ctx, updateID)
}

// Notify sends a monday bell notification about an item or board.
func (s *Service) Notify(ctx context.Context, userID, targetID, targetType, text string) error {
	if err := requireID("user_id", userID); err != nil {
		return err
	}
	if err := requireID("target_id", targetID); err != nil {
		return err
	}
	if targetType == "" {
		targetType = "Project"
	}
	if err := oneOf("target_type", targetType, "Project", "Post"); err != nil {
		return err
	}
	if err := requireText("text", text); err != nil {
		return err
	}
	if len(text) > 2000 {
		return invalid("text exceeds 2000 characters")
	}
	if targetType == "Project" {
		if _, err := s.checkItem(ctx, targetID); err != nil {
			return err
		}
	} else if err := s.guard.CheckWrite(); err != nil {
		return err
	}
	return s.port.CreateNotification(ctx, userID, targetID, targetType, text)
}

// ---- tags ------------------------------------------------------------------

// ListTags lists account public tags.
func (s *Service) ListTags(ctx context.Context, ids []string) ([]domain.Tag, error) {
	for _, id := range ids {
		if err := requireID("tag_ids", id); err != nil {
			return nil, err
		}
	}
	return s.port.ListTags(ctx, ids)
}

// CreateOrGetTag returns or creates a tag (board-scoped for private boards).
func (s *Service) CreateOrGetTag(ctx context.Context, boardID, name string) (*domain.Tag, error) {
	if err := requireText("name", name); err != nil {
		return nil, err
	}
	if boardID != "" {
		if err := requireID("board_id", boardID); err != nil {
			return nil, err
		}
		if err := s.guard.CheckBoard(boardID); err != nil {
			return nil, err
		}
	} else if err := s.guard.CheckWrite(); err != nil {
		return nil, err
	}
	return s.port.CreateOrGetTag(ctx, boardID, name)
}

// ---- templates -------------------------------------------------------------

// ProvisionResult describes a board created from a template.
type ProvisionResult struct {
	Board    domain.Board    `json:"board"`
	Template string          `json:"template"`
	Groups   []domain.Group  `json:"groups"`
	Columns  []domain.Column `json:"columns"`
	Items    []domain.Item   `json:"items,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

// ProvisionBoard creates a board from a built-in template: board, columns
// (with labels), groups in order, and optional seed items. Each step is
// additive; a failure is reported as a warning and never rolls back by
// deleting anything.
func (s *Service) ProvisionBoard(ctx context.Context, templateKey, name, workspaceID, kind, description string, seed []domain.TemplateItem) (*ProvisionResult, error) {
	template, err := domain.LookupBoardTemplate(templateKey)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}
	if name == "" {
		name = template.Title
	}
	if description == "" {
		description = template.Description
	}
	if len(seed) > MaxBulkItems {
		return nil, invalid("seed_items accepts at most %d items", MaxBulkItems)
	}
	board, err := s.CreateBoard(ctx, monday.CreateBoardInput{Name: name, Kind: kind, WorkspaceID: workspaceID, Description: description, Empty: true})
	if err != nil {
		return nil, fmt.Errorf("create board: %w", err)
	}
	result := &ProvisionResult{Board: *board, Template: template.Key}
	for _, column := range template.Columns {
		created, err := s.port.CreateColumn(ctx, monday.CreateColumnInput{
			BoardID: board.ID, ID: column.ID, Title: column.Title, Type: column.Type,
			Description: column.Description, Defaults: domain.ColumnDefaults(column),
		})
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("column %q: %v", column.Title, err))
			continue
		}
		result.Columns = append(result.Columns, *created)
	}
	existing, _ := s.port.ListGroups(ctx, board.ID)
	groupIDs := map[string]string{}
	previous := ""
	for index, title := range template.Groups {
		var group *domain.Group
		var err error
		if index == 0 && len(existing) > 0 {
			// monday creates a default group on new boards: reuse it as the
			// first template group instead of leaving an empty one behind.
			group, err = s.port.UpdateGroup(ctx, board.ID, existing[0].ID, "title", title)
		} else if previous != "" {
			group, err = s.port.CreateGroup(ctx, board.ID, title, "", previous, "after_at")
		} else {
			group, err = s.port.CreateGroup(ctx, board.ID, title, "", "", "")
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("group %q: %v", title, err))
			continue
		}
		previous = group.ID
		groupIDs[strings.ToLower(title)] = group.ID
		result.Groups = append(result.Groups, *group)
	}
	if len(seed) > 0 {
		columns, err := s.port.ListColumns(ctx, board.ID)
		if err != nil {
			result.Warnings = append(result.Warnings, "seed items skipped: "+err.Error())
			return result, nil
		}
		for _, entry := range seed {
			plan := domain.PlanColumnValues(columns, entry.Values)
			if !plan.Valid {
				result.Warnings = append(result.Warnings, fmt.Sprintf("seed %q: %v", entry.Name, plan.Err()))
				continue
			}
			item, err := s.port.CreateItem(ctx, board.ID, groupIDs[strings.ToLower(entry.Group)], entry.Name, plan.Normalized)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("seed %q: %v", entry.Name, err))
				continue
			}
			result.Items = append(result.Items, *item)
		}
	}
	return result, nil
}

// ---- reports ---------------------------------------------------------------

// Snapshot loads a board schema and up to maxItems items via cursor paging.
func (s *Service) Snapshot(ctx context.Context, boardID string, maxItems int) (domain.BoardSnapshot, error) {
	if err := requireID("board_id", boardID); err != nil {
		return domain.BoardSnapshot{}, err
	}
	if maxItems <= 0 || maxItems > s.reportMaxItems {
		maxItems = s.reportMaxItems
	}
	schema, err := s.port.GetBoardSchema(ctx, boardID)
	if err != nil {
		return domain.BoardSnapshot{}, err
	}
	snapshot := domain.BoardSnapshot{Board: schema.Board, Columns: schema.Columns, Groups: schema.Groups}
	cursor := ""
	for {
		pageSize := min(100, maxItems-len(snapshot.Items))
		page, err := s.port.ListItemsPage(ctx, monday.ItemPageQuery{BoardID: boardID, Limit: pageSize, Cursor: cursor})
		if err != nil {
			return domain.BoardSnapshot{}, err
		}
		snapshot.Items = append(snapshot.Items, page.Items...)
		cursor = page.Cursor
		if cursor == "" {
			break
		}
		if len(snapshot.Items) >= maxItems {
			snapshot.Truncated = true
			break
		}
	}
	return snapshot, nil
}

// Now returns the service clock.
func (s *Service) Now() time.Time { return s.now() }

// WorkspaceOverview summarizes the boards of a workspace.
type WorkspaceOverview struct {
	Workspace  domain.Workspace `json:"workspace"`
	BoardCount int              `json:"board_count"`
	ItemCount  int              `json:"item_count"`
	Boards     []domain.Board   `json:"boards"`
	Truncated  bool             `json:"truncated"`
}

// BuildWorkspaceOverview lists up to 100 active boards and totals their items.
func (s *Service) BuildWorkspaceOverview(ctx context.Context, workspaceID string) (*WorkspaceOverview, error) {
	workspace, err := s.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	boards, err := s.port.ListBoardsPage(ctx, monday.BoardQuery{Limit: 100, WorkspaceIDs: []string{workspaceID}, State: "active"})
	if err != nil {
		return nil, err
	}
	overview := &WorkspaceOverview{Workspace: *workspace, Truncated: len(boards) == 100}
	for _, board := range boards {
		if strings.HasPrefix(board.Name, "Subitems of") || strings.HasPrefix(board.Name, "Subelementos de") {
			continue
		}
		overview.Boards = append(overview.Boards, board)
		overview.ItemCount += board.ItemsCount
	}
	overview.BoardCount = len(overview.Boards)
	return overview, nil
}
