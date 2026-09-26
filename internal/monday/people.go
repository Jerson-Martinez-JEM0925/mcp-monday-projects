package monday

import (
	"context"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
)

const userFields = `id name email title kind status time_zone_identifier url teams { id name }`

const meQuery = `query Me {
  me { ` + userFields + ` account { id name slug tier country_code active_members_count } }
}`

const apiStatusQuery = `query APIStatus {
  complexity { before after query reset_in_x_seconds }
  version { value kind }
}`

const listUsersQuery = `query ListUsers($limit: Int, $page: Int, $name: String, $emails: [String!], $ids: [ID!]) {
  users(limit: $limit, page: $page, name: $name, emails: $emails, ids: $ids) { ` + userFields + ` }
}`

const listTeamsQuery = `query ListTeams($ids: [ID!]) {
  teams(ids: $ids) { id name users { id name } }
}`

const updateFields = `id body text_body created_at item_id creator { id name }
  replies { id text_body created_at creator { id name } }`

const itemUpdatesQuery = `query ItemUpdates($itemID: ID!, $limit: Int!) {
  items(ids: [$itemID]) { updates(limit: $limit) { ` + updateFields + ` } }
}`

const boardUpdatesQuery = `query BoardUpdates($boardID: ID!, $limit: Int!) {
  boards(ids: [$boardID]) { updates(limit: $limit) { ` + updateFields + ` } }
}`

const createUpdateMutation = `mutation CreateUpdate($itemID: ID, $body: String!, $parentID: ID) {
  create_update(item_id: $itemID, body: $body, parent_id: $parentID) { id body text_body created_at item_id creator { id name } }
}`

const likeUpdateMutation = `mutation LikeUpdate($updateID: ID!) {
  like_update(update_id: $updateID) { id }
}`

const createNotificationMutation = `mutation CreateNotification($userID: ID!, $targetID: ID!, $targetType: NotificationTargetType!, $text: String!) {
  create_notification(user_id: $userID, target_id: $targetID, target_type: $targetType, text: $text) { text }
}`

const listTagsQuery = `query ListTags($ids: [ID!]) {
  tags(ids: $ids) { id name color }
}`

const createOrGetTagMutation = `mutation CreateOrGetTag($boardID: ID, $name: String) {
  create_or_get_tag(board_id: $boardID, tag_name: $name) { id name color }
}`

type wireUser struct {
	domain.User
	TimeZoneIdentifier string          `json:"time_zone_identifier"`
	Account            *domain.Account `json:"account"`
}

func (w wireUser) toDomain() domain.User {
	user := w.User
	user.TimeZone = w.TimeZoneIdentifier
	return user
}

// UserQuery filters the user directory.
type UserQuery struct {
	Limit  int
	Page   int
	Name   string
	Emails []string
	IDs    []string
}

// Me returns the authenticated user and account.
func (c *Client) Me(ctx context.Context) (*domain.Me, error) {
	var data struct {
		Me wireUser `json:"me"`
	}
	if err := c.Do(ctx, meQuery, nil, &data); err != nil {
		return nil, err
	}
	me := &domain.Me{User: data.Me.toDomain(), Account: data.Me.Account}
	me.IsAdmin = me.Kind == "admin"
	return me, nil
}

// APIStatus returns the complexity budget and the API version that served it.
func (c *Client) APIStatus(ctx context.Context) (domain.Complexity, domain.APIVersion, error) {
	var data struct {
		Complexity domain.Complexity `json:"complexity"`
		Version    domain.APIVersion `json:"version"`
	}
	if err := c.Do(ctx, apiStatusQuery, nil, &data); err != nil {
		return domain.Complexity{}, domain.APIVersion{}, err
	}
	return data.Complexity, data.Version, nil
}

// ListUsers lists users with optional filters.
func (c *Client) ListUsers(ctx context.Context, query UserQuery) ([]domain.User, error) {
	var data struct {
		Users []wireUser `json:"users"`
	}
	vars := map[string]any{
		"limit": optionalInt(query.Limit), "page": optionalInt(query.Page), "name": optional(query.Name),
		"emails": optionalList(query.Emails), "ids": optionalList(query.IDs),
	}
	if err := c.Do(ctx, listUsersQuery, vars, &data); err != nil {
		return nil, err
	}
	users := make([]domain.User, 0, len(data.Users))
	for _, user := range data.Users {
		users = append(users, user.toDomain())
	}
	return users, nil
}

// ListTeams lists teams (all, or by ID) with their members.
func (c *Client) ListTeams(ctx context.Context, ids []string) ([]domain.Team, error) {
	var data struct {
		Teams []domain.Team `json:"teams"`
	}
	if err := c.Do(ctx, listTeamsQuery, map[string]any{"ids": optionalList(ids)}, &data); err != nil {
		return nil, err
	}
	return data.Teams, nil
}

// ListItemUpdates lists updates (with replies) of an item.
func (c *Client) ListItemUpdates(ctx context.Context, itemID string, limit int) ([]domain.Update, error) {
	var data struct {
		Items []struct {
			Updates []domain.Update `json:"updates"`
		} `json:"items"`
	}
	if err := c.Do(ctx, itemUpdatesQuery, map[string]any{"itemID": itemID, "limit": limit}, &data); err != nil {
		return nil, err
	}
	if len(data.Items) == 0 {
		return nil, &NotFoundError{Resource: "item", ID: itemID}
	}
	return data.Items[0].Updates, nil
}

// ListBoardUpdates lists the most recent updates across a board.
func (c *Client) ListBoardUpdates(ctx context.Context, boardID string, limit int) ([]domain.Update, error) {
	var data struct {
		Boards []struct {
			Updates []domain.Update `json:"updates"`
		} `json:"boards"`
	}
	if err := c.Do(ctx, boardUpdatesQuery, map[string]any{"boardID": boardID, "limit": limit}, &data); err != nil {
		return nil, err
	}
	if len(data.Boards) == 0 {
		return nil, &NotFoundError{Resource: "board", ID: boardID}
	}
	return data.Boards[0].Updates, nil
}

// CreateUpdate posts an update on an item, or a reply when parentID is set.
func (c *Client) CreateUpdate(ctx context.Context, itemID, body, parentID string) (*domain.Update, error) {
	var data struct {
		Update domain.Update `json:"create_update"`
	}
	vars := map[string]any{"itemID": optional(itemID), "body": body, "parentID": optional(parentID)}
	if err := c.Do(ctx, createUpdateMutation, vars, &data); err != nil {
		return nil, err
	}
	return &data.Update, nil
}

// LikeUpdate likes an update.
func (c *Client) LikeUpdate(ctx context.Context, updateID string) error {
	var data struct {
		Like struct {
			ID string `json:"id"`
		} `json:"like_update"`
	}
	return c.Do(ctx, likeUpdateMutation, map[string]any{"updateID": updateID}, &data)
}

// CreateNotification sends a monday notification to a user.
func (c *Client) CreateNotification(ctx context.Context, userID, targetID, targetType, text string) error {
	var data struct {
		Notification struct {
			Text string `json:"text"`
		} `json:"create_notification"`
	}
	vars := map[string]any{"userID": userID, "targetID": targetID, "targetType": targetType, "text": text}
	return c.Do(ctx, createNotificationMutation, vars, &data)
}

// ListTags lists account-level public tags.
func (c *Client) ListTags(ctx context.Context, ids []string) ([]domain.Tag, error) {
	var data struct {
		Tags []domain.Tag `json:"tags"`
	}
	if err := c.Do(ctx, listTagsQuery, map[string]any{"ids": optionalList(ids)}, &data); err != nil {
		return nil, err
	}
	return data.Tags, nil
}

// CreateOrGetTag returns an existing tag or creates it.
func (c *Client) CreateOrGetTag(ctx context.Context, boardID, name string) (*domain.Tag, error) {
	var data struct {
		Tag domain.Tag `json:"create_or_get_tag"`
	}
	if err := c.Do(ctx, createOrGetTagMutation, map[string]any{"boardID": optional(boardID), "name": name}, &data); err != nil {
		return nil, err
	}
	return &data.Tag, nil
}
