package chat

import (
	"context"
	"testing"
)

func TestChatLaneList_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatLaneListHandler(context.Background(), nil, &ChatLaneListInput{}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Lanes) == 0 {
		t.Fatal("expected at least one lane")
	}
	if out.Lanes[0].Name != "backlog" {
		t.Errorf("first lane = %q, want %q", out.Lanes[0].Name, "backlog")
	}
}

func TestChatLaneList_ModuleFilter(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatLaneListHandler(context.Background(), nil, &ChatLaneListInput{
		Module: "tasks",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Lanes) == 0 {
		t.Fatal("expected lanes filtered by module")
	}
}

func TestChatLaneList_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatLaneListHandler(context.Background(), nil, &ChatLaneListInput{}, nil, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}
