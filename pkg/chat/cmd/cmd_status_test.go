package cmd

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleStatus_NoProject(t *testing.T) {
	resp, err := statusCommandHandler(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	if resp != "**Error:** No project loaded." {
		t.Errorf("expected '**Error:** No project loaded.', got %q", resp)
	}
}

func TestHandleStatus_WithProject(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	resp, err := statusCommandHandler(context.Background(), &Request{Project: proj})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	if !containsAny(resp, "Project:", "test-project", "**Status:**") {
		t.Errorf("response missing expected content: %s", resp)
	}
}
