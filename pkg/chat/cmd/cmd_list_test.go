package cmd

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleList_NoProject(t *testing.T) {
	resp, err := listCommandHandler(context.Background(), &Request{ExtId: "user1"})
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if resp != "**Error:** No project loaded." {
		t.Errorf("expected '**Error:** No project loaded.', got %q", resp)
	}
}

func TestHandleList_EmptyProject(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	resp, err := listCommandHandler(context.Background(), &Request{
		ExtId:   "user1",
		Project: proj,
	})
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if !containsAny(resp, "No workitems found") {
		t.Errorf("expected 'No workitems found', got %q", resp)
	}
}

func TestHandleList_LaneNotFound(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	resp, err := listCommandHandler(context.Background(), &Request{
		ExtId:   "user1",
		Project: proj,
		Content: "nonexistent",
	})
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if resp != `**Error:** Lane "nonexistent" not found.` {
		t.Errorf("expected lane not found message, got %q", resp)
	}
}
