package cmd

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleLanes_NoProject(t *testing.T) {
	resp, err := lanesCommandHandler(context.Background(), &Request{ExtId: "user1"})
	if err != nil {
		t.Fatalf("handleLanes: %v", err)
	}
	if resp != "**Error:** No project loaded." {
		t.Errorf("expected '**Error:** No project loaded.', got %q", resp)
	}
}

func TestHandleLanes_WithProject(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	resp, err := lanesCommandHandler(context.Background(), &Request{
		ExtId:   "user1",
		Project: proj,
	})
	if err != nil {
		t.Fatalf("handleLanes: %v", err)
	}
	if !containsAny(resp, "Available Lanes") {
		t.Errorf("response missing expected content: %s", resp)
	}
}

func TestHandleLanes_Registered(t *testing.T) {
	cmd, ok := Get("lanes")
	if !ok {
		t.Fatal("command 'lanes' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
