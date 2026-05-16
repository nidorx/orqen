package cmd

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleItem_NoProject(t *testing.T) {
	resp, err := itemCommandHandler(context.Background(), &Request{ExtId: "user1"})
	if err != nil {
		t.Fatalf("handleItem: %v", err)
	}
	if resp != "**Error:** No project loaded." {
		t.Errorf("expected '**Error:** No project loaded.', got %q", resp)
	}
}

func TestHandleItem_NotFound(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	resp, err := itemCommandHandler(context.Background(), &Request{
		ExtId:   "user1",
		Project: proj,
		Content: "NONEXISTENT",
	})
	if err != nil {
		t.Fatalf("handleItem: %v", err)
	}
	if resp != `**Error:** Workitem "NONEXISTENT" not found.` {
		t.Errorf("expected not found message, got %q", resp)
	}
}

func TestHandleItem_Registered(t *testing.T) {
	cmd, ok := Get("item")
	if !ok {
		t.Fatal("command 'item' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
