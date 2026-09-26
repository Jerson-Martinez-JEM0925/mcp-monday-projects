package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jersonmartinez/mcp-monday-projects/internal/application"
	"github.com/jersonmartinez/mcp-monday-projects/internal/application/applicationtest"
)

func deleteService(t *testing.T, options application.Options) (*application.Service, *applicationtest.FakePort) {
	t.Helper()
	port := applicationtest.NewFakePort()
	port.Columns["200"] = port.Columns["100"]
	port.BoardWorkspace["200"] = "8"
	return application.NewService(port, options), port
}

type deleteCall struct {
	name string
	call func(*application.Service, bool) error
}

func deleteCalls() []deleteCall {
	return []deleteCall{
		{"item", func(s *application.Service, c bool) error { return s.DeleteItem(ctx, "500", c) }},
		{"group", func(s *application.Service, c bool) error { return s.DeleteGroup(ctx, "100", "todo", c) }},
		{"board", func(s *application.Service, c bool) error { return s.DeleteBoard(ctx, "100", c) }},
		{"column", func(s *application.Service, c bool) error { return s.DeleteColumn(ctx, "100", "est", c) }},
		{"update", func(s *application.Service, c bool) error { return s.DeleteUpdate(ctx, "500", "77", c) }},
		{"folder", func(s *application.Service, c bool) error { return s.DeleteFolder(ctx, "7", "301", c) }},
		{"workspace", func(s *application.Service, c bool) error { return s.DeleteWorkspace(ctx, "7", c) }},
	}
}

func TestDeletesAreDisabledBelowFullAccess(t *testing.T) {
	svc, port := deleteService(t, application.Options{})
	for _, tc := range deleteCalls() {
		if err := tc.call(svc, true); !errors.Is(err, application.ErrDeletesDisabled) {
			t.Fatalf("%s: err = %v, want ErrDeletesDisabled", tc.name, err)
		}
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestDeletesRequireConfirm(t *testing.T) {
	svc, port := deleteService(t, application.Options{AllowDelete: true})
	for _, tc := range deleteCalls() {
		err := tc.call(svc, false)
		if err == nil || !strings.Contains(err.Error(), "permanent") || !strings.Contains(err.Error(), "confirm=true") {
			t.Fatalf("%s: err = %v", tc.name, err)
		}
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestConfirmedDeletesReachThePort(t *testing.T) {
	svc, port := deleteService(t, application.Options{AllowDelete: true})
	for _, tc := range deleteCalls() {
		if err := tc.call(svc, true); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	if port.MutationCount() != len(deleteCalls()) {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}

func TestDeletesRespectReadOnlyAllowlistsAndScope(t *testing.T) {
	readOnly, _ := deleteService(t, application.Options{AllowDelete: true, Guard: application.NewWriteGuard(true, nil, nil)})
	if err := readOnly.DeleteItem(ctx, "500", true); !errors.Is(err, application.ErrReadOnly) {
		t.Fatalf("read-only err = %v", err)
	}

	allow, port := deleteService(t, application.Options{AllowDelete: true, Guard: application.NewWriteGuard(false, []string{"999"}, nil)})
	var guardErr *application.GuardError
	if err := allow.DeleteBoard(ctx, "100", true); !errors.As(err, &guardErr) {
		t.Fatalf("allowlist board err = %v", err)
	}
	if err := allow.DeleteItem(ctx, "500", true); !errors.As(err, &guardErr) {
		t.Fatalf("allowlist item err = %v", err)
	}
	if err := allow.DeleteWorkspace(ctx, "7", true); err == nil {
		t.Fatal("workspace delete allowed under an allowlist")
	}

	scoped, scopedPort := deleteService(t, application.Options{AllowDelete: true, WorkspaceScope: "7"})
	for name, call := range map[string]func() error{
		"board":     func() error { return scoped.DeleteBoard(ctx, "200", true) },
		"group":     func() error { return scoped.DeleteGroup(ctx, "200", "todo", true) },
		"column":    func() error { return scoped.DeleteColumn(ctx, "200", "est", true) },
		"workspace": func() error { return scoped.DeleteWorkspace(ctx, "7", true) },
		"folder":    func() error { return scoped.DeleteFolder(ctx, "8", "301", true) },
	} {
		wantScope(t, name, call())
	}
	if err := scoped.DeleteFolder(ctx, "", "301", true); err != nil {
		t.Fatalf("in-scope folder with default workspace: %v", err)
	}
	if port.MutationCount() != 0 || scopedPort.MutationCount() != 1 {
		t.Fatalf("mutations = %v / %v", port.Mutations, scopedPort.Mutations)
	}
}

func TestDeleteUpdateMustBelongToItem(t *testing.T) {
	svc, port := deleteService(t, application.Options{AllowDelete: true})
	if err := svc.DeleteUpdate(ctx, "500", "88", true); err == nil || !strings.Contains(err.Error(), "not among") {
		t.Fatalf("err = %v", err)
	}
	if err := svc.DeleteColumn(ctx, "100", "name", true); err == nil {
		t.Fatal("name column delete allowed")
	}
	if port.MutationCount() != 0 {
		t.Fatalf("mutations = %v", port.Mutations)
	}
}
