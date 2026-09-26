package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ErrReadOnly is returned by every mutation when the server runs read-only.
var ErrReadOnly = errors.New("server is running in read-only mode (MCP_READ_ONLY=true); mutations are disabled")

// GuardError explains why a mutation target is outside the allowlist.
type GuardError struct {
	Resource string
	ID       string
}

func (e *GuardError) Error() string {
	return fmt.Sprintf("%s %s is not in the write allowlist; add it to MONDAY_WRITE_%s_ALLOWLIST to allow mutations", e.Resource, e.ID, upper(e.Resource))
}

func upper(s string) string {
	if s == "board" {
		return "BOARD"
	}
	return "WORKSPACE"
}

// WriteGuard enforces read-only mode and board/workspace write allowlists.
// Boards created through this server are trusted for the process lifetime,
// so a provisioned board can be populated even under a strict allowlist.
type WriteGuard struct {
	readOnly   bool
	boards     map[string]bool
	workspaces map[string]bool
	mu         sync.RWMutex
	created    map[string]bool
}

// NewWriteGuard builds a guard. Empty allowlists mean "no restriction".
func NewWriteGuard(readOnly bool, boards, workspaces []string) *WriteGuard {
	guard := &WriteGuard{readOnly: readOnly, boards: map[string]bool{}, workspaces: map[string]bool{}, created: map[string]bool{}}
	for _, id := range boards {
		guard.boards[id] = true
	}
	for _, id := range workspaces {
		guard.workspaces[id] = true
	}
	return guard
}

// ReadOnly reports whether mutations are disabled.
func (g *WriteGuard) ReadOnly() bool { return g.readOnly }

// Restricted reports whether any allowlist is active.
func (g *WriteGuard) Restricted() bool { return len(g.boards) > 0 || len(g.workspaces) > 0 }

// Describe returns the guard's effective policy for diagnostics.
func (g *WriteGuard) Describe() map[string]any {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]any{
		"read_only":             g.readOnly,
		"board_allowlist":       keys(g.boards),
		"workspace_allowlist":   keys(g.workspaces),
		"session_created_board": keys(g.created),
	}
}

func keys(m map[string]bool) []string {
	list := make([]string, 0, len(m))
	for key := range m {
		list = append(list, key)
	}
	sort.Strings(list)
	return list
}

// CheckWrite rejects every mutation in read-only mode.
func (g *WriteGuard) CheckWrite() error {
	if g.readOnly {
		return ErrReadOnly
	}
	return nil
}

// CheckBoard authorizes a mutation on a board.
func (g *WriteGuard) CheckBoard(boardID string) error {
	if err := g.CheckWrite(); err != nil {
		return err
	}
	if !g.Restricted() {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.boards[boardID] || g.created[boardID] {
		return nil
	}
	return &GuardError{Resource: "board", ID: boardID}
}

// CheckWorkspace authorizes creating resources inside a workspace. With only
// a board allowlist configured, creating new boards is refused.
func (g *WriteGuard) CheckWorkspace(workspaceID string) error {
	if err := g.CheckWrite(); err != nil {
		return err
	}
	if !g.Restricted() {
		return nil
	}
	if workspaceID != "" && g.workspaces[workspaceID] {
		return nil
	}
	return &GuardError{Resource: "workspace", ID: orUnset(workspaceID)}
}

func orUnset(id string) string {
	if id == "" {
		return "(none)"
	}
	return id
}

// TrustBoard marks a board created by this process as writable.
func (g *WriteGuard) TrustBoard(boardID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.created[boardID] = true
}

// boardOfItem resolves the board that owns an item, for guard checks.
func (s *Service) checkItem(ctx context.Context, itemID string) (string, error) {
	if err := s.guard.CheckWrite(); err != nil {
		return "", err
	}
	if !s.guard.Restricted() {
		return "", nil
	}
	items, err := s.port.GetItems(ctx, []string{itemID})
	if err != nil {
		return "", fmt.Errorf("resolve item board: %w", err)
	}
	if len(items) == 0 {
		return "", fmt.Errorf("item not found: %s", itemID)
	}
	return items[0].BoardID, s.guard.CheckBoard(items[0].BoardID)
}

// Service is the application facade used by the MCP layer.
type Service struct {
	port           Port
	guard          *WriteGuard
	now            func() time.Time
	reportMaxItems int
}

// Options configures a Service.
type Options struct {
	Guard          *WriteGuard
	Now            func() time.Time
	ReportMaxItems int
}

// NewService builds the application service.
func NewService(port Port, options Options) *Service {
	if options.Guard == nil {
		options.Guard = NewWriteGuard(false, nil, nil)
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.ReportMaxItems <= 0 {
		options.ReportMaxItems = 500
	}
	return &Service{port: port, guard: options.Guard, now: options.Now, reportMaxItems: options.ReportMaxItems}
}

// Guard exposes the write guard for diagnostics.
func (s *Service) Guard() *WriteGuard { return s.guard }
