package mcp

import (
	"context"
	"testing"
)

func TestChatProjectGet_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatProjectGetHandler(context.Background(), nil, &ChatProjectGetInput{}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.ModuleCount == 0 {
		t.Error("expected at least one module")
	}
	if out.DirAbs != dir {
		t.Errorf("DirAbs = %q, want %q", out.DirAbs, dir)
	}
}

func TestChatProjectGet_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatProjectGetHandler(context.Background(), nil, &ChatProjectGetInput{}, nil, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}
