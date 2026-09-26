package mcpserver

import (
	"context"
	"errors"

	"github.com/jersonmartinez/mcp-monday-projects/internal/domain"
	"github.com/jersonmartinez/mcp-monday-projects/internal/monday"
)

var errGroupRequired = errors.New("group_id is required")

// ListUsersInput lists or searches users.
type ListUsersInput struct {
	Limit  int      `json:"limit,omitempty" jsonschema:"page size 1-200 (default 50)"`
	Page   int      `json:"page,omitempty" jsonschema:"1-based page number"`
	Name   string   `json:"name,omitempty" jsonschema:"filter by (partial) name"`
	Emails []string `json:"emails,omitempty" jsonschema:"filter by exact emails"`
}

// UsersOutput lists users.
type UsersOutput struct {
	Users []domain.User `json:"users"`
	Count int           `json:"count"`
}

// UserIDInput identifies a user.
type UserIDInput struct {
	UserID string `json:"user_id" jsonschema:"monday user identifier"`
}

// UserOutput wraps one user.
type UserOutput struct {
	User domain.User `json:"user"`
}

// TeamsInput selects teams.
type TeamsInput struct {
	TeamIDs []string `json:"team_ids,omitempty" jsonschema:"optional team IDs; empty lists every team"`
}

// TeamsOutput lists teams.
type TeamsOutput struct {
	Teams []domain.Team `json:"teams"`
	Count int           `json:"count"`
}

// ItemUpdatesInput lists item updates.
type ItemUpdatesInput struct {
	ItemID string `json:"item_id" jsonschema:"monday item identifier"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max updates 1-100 (default 25)"`
}

// BoardUpdatesInput lists board updates.
type BoardUpdatesInput struct {
	BoardID string `json:"board_id" jsonschema:"monday board identifier"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max updates 1-100 (default 25)"`
}

// UpdatesOutput lists updates.
type UpdatesOutput struct {
	Updates []domain.Update `json:"updates"`
	Count   int             `json:"count"`
}

// CreateUpdateInput posts an update.
type CreateUpdateInput struct {
	ItemID string `json:"item_id" jsonschema:"monday item identifier"`
	Body   string `json:"body" jsonschema:"update body (HTML or plain text, max 20000 chars)"`
}

// ReplyInput replies to an update.
type ReplyInput struct {
	ItemID   string `json:"item_id" jsonschema:"item that owns the update (used for the write policy)"`
	UpdateID string `json:"update_id" jsonschema:"update to reply to"`
	Body     string `json:"body" jsonschema:"reply body"`
}

// LikeInput likes an update.
type LikeInput struct {
	ItemID   string `json:"item_id" jsonschema:"item that owns the update (used for the write policy)"`
	UpdateID string `json:"update_id" jsonschema:"update to like"`
}

// UpdateOutput wraps one update.
type UpdateOutput struct {
	Update domain.Update `json:"update"`
}

// NotifyInput sends a notification.
type NotifyInput struct {
	UserID     string `json:"user_id" jsonschema:"recipient user"`
	TargetID   string `json:"target_id" jsonschema:"item ID (target_type=Project) or update ID (Post)"`
	TargetType string `json:"target_type,omitempty" jsonschema:"Project (default, an item) or Post (an update)"`
	Text       string `json:"text" jsonschema:"notification text (max 2000 chars)"`
}

// TagsInput selects tags.
type TagsInput struct {
	TagIDs []string `json:"tag_ids,omitempty" jsonschema:"optional tag IDs"`
}

// TagsOutput lists tags.
type TagsOutput struct {
	Tags  []domain.Tag `json:"tags"`
	Count int          `json:"count"`
}

// CreateTagInput creates or gets a tag.
type CreateTagInput struct {
	Name    string `json:"name" jsonschema:"tag name"`
	BoardID string `json:"board_id,omitempty" jsonschema:"board for private/shareable boards"`
}

// TagOutput wraps one tag.
type TagOutput struct {
	Tag domain.Tag `json:"tag"`
}

func registerCollaborationTools(r *registry) {
	svc := r.svc
	add(r, ToolSpec{Name: "list_users", Category: CatPeople, Title: "List users", ReadOnly: true,
		Description: "List account users (name, email, title, kind, status, teams) with pagination."},
		func(ctx context.Context, in ListUsersInput) (UsersOutput, error) {
			users, err := svc.ListUsers(ctx, monday.UserQuery{Limit: in.Limit, Page: in.Page, Name: in.Name, Emails: in.Emails})
			if err != nil {
				return UsersOutput{}, wrap("list users", err)
			}
			return UsersOutput{Users: users, Count: len(users)}, nil
		})
	add(r, ToolSpec{Name: "search_users", Category: CatPeople, Title: "Search users", ReadOnly: true,
		Description: "Find users by partial name or exact email — handy to resolve IDs before assign_item_people."},
		func(ctx context.Context, in ListUsersInput) (UsersOutput, error) {
			if in.Name == "" && len(in.Emails) == 0 {
				return UsersOutput{}, errors.New("search users: provide name or emails")
			}
			users, err := svc.ListUsers(ctx, monday.UserQuery{Limit: in.Limit, Name: in.Name, Emails: in.Emails})
			if err != nil {
				return UsersOutput{}, wrap("search users", err)
			}
			return UsersOutput{Users: users, Count: len(users)}, nil
		})
	add(r, ToolSpec{Name: "get_user", Category: CatPeople, Title: "Get user", ReadOnly: true,
		Description: "Get one user by ID."},
		func(ctx context.Context, in UserIDInput) (UserOutput, error) {
			user, err := svc.GetUser(ctx, in.UserID)
			if err != nil {
				return UserOutput{}, wrap("get user", err)
			}
			return UserOutput{User: *user}, nil
		})
	add(r, ToolSpec{Name: "list_teams", Category: CatPeople, Title: "List teams", ReadOnly: true,
		Description: "List teams with their members."},
		func(ctx context.Context, in TeamsInput) (TeamsOutput, error) {
			teams, err := svc.ListTeams(ctx, in.TeamIDs)
			if err != nil {
				return TeamsOutput{}, wrap("list teams", err)
			}
			return TeamsOutput{Teams: teams, Count: len(teams)}, nil
		})

	add(r, ToolSpec{Name: "list_item_updates", Category: CatCollab, Title: "List item updates", ReadOnly: true,
		Description: "List an item's updates (comments) with replies and authors."},
		func(ctx context.Context, in ItemUpdatesInput) (UpdatesOutput, error) {
			updates, err := svc.ListItemUpdates(ctx, in.ItemID, in.Limit)
			if err != nil {
				return UpdatesOutput{}, wrap("list item updates", err)
			}
			return UpdatesOutput{Updates: updates, Count: len(updates)}, nil
		})
	add(r, ToolSpec{Name: "list_board_updates", Category: CatCollab, Title: "List board updates", ReadOnly: true,
		Description: "List the most recent updates across a board — an activity feed."},
		func(ctx context.Context, in BoardUpdatesInput) (UpdatesOutput, error) {
			updates, err := svc.ListBoardUpdates(ctx, in.BoardID, in.Limit)
			if err != nil {
				return UpdatesOutput{}, wrap("list board updates", err)
			}
			return UpdatesOutput{Updates: updates, Count: len(updates)}, nil
		})
	add(r, ToolSpec{Name: "create_update", Category: CatCollab, Title: "Post update",
		Description: "Post an update (comment) on an item."},
		func(ctx context.Context, in CreateUpdateInput) (UpdateOutput, error) {
			update, err := svc.CreateUpdate(ctx, in.ItemID, in.Body)
			if err != nil {
				return UpdateOutput{}, wrap("create update", err)
			}
			return UpdateOutput{Update: *update}, nil
		})
	add(r, ToolSpec{Name: "reply_to_update", Category: CatCollab, Title: "Reply to update",
		Description: "Reply to an existing update."},
		func(ctx context.Context, in ReplyInput) (UpdateOutput, error) {
			update, err := svc.ReplyToUpdate(ctx, in.ItemID, in.UpdateID, in.Body)
			if err != nil {
				return UpdateOutput{}, wrap("reply to update", err)
			}
			return UpdateOutput{Update: *update}, nil
		})
	add(r, ToolSpec{Name: "like_update", Category: CatCollab, Title: "Like update",
		Description: "Like an update."},
		func(ctx context.Context, in LikeInput) (OKOutput, error) {
			if err := svc.LikeUpdate(ctx, in.ItemID, in.UpdateID); err != nil {
				return OKOutput{}, wrap("like update", err)
			}
			return OKOutput{OK: true}, nil
		})
	add(r, ToolSpec{Name: "notify_user", Category: CatCollab, Title: "Notify user",
		Description: "Send a monday bell notification to a user about an item or update."},
		func(ctx context.Context, in NotifyInput) (OKOutput, error) {
			if err := svc.Notify(ctx, in.UserID, in.TargetID, in.TargetType, in.Text); err != nil {
				return OKOutput{}, wrap("notify user", err)
			}
			return OKOutput{OK: true, Message: "notification sent"}, nil
		})

	add(r, ToolSpec{Name: "list_tags", Category: CatTags, Title: "List tags", ReadOnly: true,
		Description: "List account public tags."},
		func(ctx context.Context, in TagsInput) (TagsOutput, error) {
			tags, err := svc.ListTags(ctx, in.TagIDs)
			if err != nil {
				return TagsOutput{}, wrap("list tags", err)
			}
			return TagsOutput{Tags: tags, Count: len(tags)}, nil
		})
	add(r, ToolSpec{Name: "create_or_get_tag", Category: CatTags, Title: "Create or get tag",
		Description: "Return an existing tag by name or create it; use its ID in a tags column."},
		func(ctx context.Context, in CreateTagInput) (TagOutput, error) {
			tag, err := svc.CreateOrGetTag(ctx, in.BoardID, in.Name)
			if err != nil {
				return TagOutput{}, wrap("create or get tag", err)
			}
			return TagOutput{Tag: *tag}, nil
		})
}
