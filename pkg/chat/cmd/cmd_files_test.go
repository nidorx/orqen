package cmd

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleFiles_NoProject(t *testing.T) {
	resp, err := filesCommandHandler(context.Background(), &Request{ExtId: "user1"})
	if err != nil {
		t.Fatalf("handleFiles: %v", err)
	}
	if resp != "**Error:** No project loaded." {
		t.Errorf("expected '**Error:** No project loaded.', got %q", resp)
	}
}

func TestHandleFiles_BlockedPath(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		DirAbs:  t.TempDir(),
		Modules: []*engine.Module{},
	}
	resp, err := filesCommandHandler(context.Background(), &Request{
		ExtId:   "user1",
		Project: proj,
		Content: ".orqen",
	})
	if err != nil {
		t.Fatalf("handleFiles: %v", err)
	}
	if !containsAny(resp, "Access denied", "protected") {
		t.Errorf("expected access denied message, got %q", resp)
	}
}

func TestHandleFiles_Registered(t *testing.T) {
	cmd, ok := Get("files")
	if !ok {
		t.Fatal("command 'files' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
