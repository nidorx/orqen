package cmd

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleProject_NoProject(t *testing.T) {
	resp, err := projectCommandHandler(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("handleProject: %v", err)
	}
	if resp != "**Error:** No project loaded." {
		t.Errorf("expected '**Error:** No project loaded.', got %q", resp)
	}
}

func TestHandleProject_WithProject(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		DirAbs:  "/tmp/test",
		Modules: []*engine.Module{},
	}
	resp, err := projectCommandHandler(context.Background(), &Request{Project: proj})
	if err != nil {
		t.Fatalf("handleProject: %v", err)
	}
	if !containsAny(resp, "Project:", "test-project", "Modules:") {
		t.Errorf("response missing expected content: %s", resp)
	}
}

func TestHandleProject_Registered(t *testing.T) {
	cmd, ok := Get("project")
	if !ok {
		t.Fatal("command 'project' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
