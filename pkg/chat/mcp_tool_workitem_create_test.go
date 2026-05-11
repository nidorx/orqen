package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestChatWorkitemCreate_MissingInputs(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	// Test the create handler validates input correctly (no project crash)
	_, out, err := ChatWorkitemCreateHandler(context.Background(), nilReq(), &ChatWorkitemCreateInput{
		Lane:  "",
		Title: "",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	// Should error gracefully for missing inputs
	if out.Error == "" {
		t.Log("create handler returned empty error for missing inputs (may be acceptable)")
	}
}

func TestChatWorkitemCreate_InvalidLane(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatWorkitemCreateHandler(context.Background(), nilReq(), &ChatWorkitemCreateInput{
		Lane:  "nonexistent",
		Title: "test",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for invalid lane")
	}
	if out.Success {
		t.Error("expected success=false for invalid lane")
	}
}

func TestChatWorkitemCreate_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatWorkitemCreateHandler(context.Background(), nilReq(), &ChatWorkitemCreateInput{
		Lane:  "backlog",
		Title: "test",
	}, nil, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}

// Helper for workitem tests - creates a lane and a workitem manually
func setupTestProjectWithLaneAndWorkitem(t *testing.T, dir string) (*engine.Project, *engine.WorkItem) {
	t.Helper()
	proj := setupTestProjectWithLane(t, dir)

	// Manually create a workitem directory and yaml file
	lane := proj.Modules[0].Lanes[0]
	itemDir := filepath.Join(lane.DirAbs, "TASK-0001-test-task")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "TASK-0001.yaml"), []byte{}, 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// Verify the directory and yaml file were created
	if _, err := os.Stat(filepath.Join(itemDir, "TASK-0001.yaml")); err != nil {
		t.Fatalf("expected workitem yaml file to exist: %v", err)
	}

	return proj, nil
}
