package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Test Helpers for MoveTo
// ============================================================================

func setupMoveToTest(t *testing.T) (*Project, string, *Module) {
	t.Helper()

	// Create a temporary project with a custom module for testing
	tempDir := t.TempDir()

	// Create .orqen directory
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create orqen.yaml with lanes in order
	orqenYAML := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 1

modules:
  - name: movetest
    dir: "movetest"
    prefix: "MT"
    order: ["inbox", "doing", "review", "done"]
    lanes:
      - name: "inbox"
        purpose: "Tasks waiting to be picked up"
      - name: "doing"
        purpose: "Tasks being implemented"
      - name: "review"
        purpose: "Tasks under review"
      - name: "done"
        purpose: "Completed tasks"
`
	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(orqenYAML), 0644); err != nil {
		t.Fatalf("failed to write orqen.yaml: %v", err)
	}

	// Create module directory
	os.MkdirAll(filepath.Join(tempDir, "movetest", "inbox"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "movetest", "doing"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "movetest", "review"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "movetest", "done"), 0755)

	// Load project
	proj, err := Load(tempDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	mod := proj.GetModule("movetest")
	if mod == nil {
		t.Fatal("movetest module not found")
	}

	return proj, tempDir, mod
}

func createTestWorkItem(t *testing.T, lane *Lane, seq int, name string) *WorkItem {
	t.Helper()

	dirPath := filepath.Join(lane.DirAbs, name)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create work item dir: %v", err)
	}

	// Create YAML file
	yamlPath := filepath.Join(dirPath, lane.Module.Prefix+"-"+fmt.Sprintf("%04d", seq)+".yaml")
	if err := os.WriteFile(yamlPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create yaml file: %v", err)
	}

	// Create work item
	item := &WorkItem{
		Seq:        seq,
		Name:       name,
		Lane:       lane,
		Attributes: make(Attributes),
	}

	// Add to lane cache
	lane.workItemsByID.Set(item.ID, item)
	lane.Module.set(item)

	return item
}

// ============================================================================
// Lane.validateMoveTo Tests
// ============================================================================

func TestValidateMoveTo_AllowedNextExplicit(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	// Configure review lane with explicit allowed_next
	reviewLane := mod.GetLane("review")
	reviewLane.AllowedNext = []string{"done", "doing"} // allow going back to doing for rework

	// Move to allowed lane should succeed
	if err := reviewLane.validateMoveTo("done"); err != nil {
		t.Errorf("expected move to 'done' to be allowed, got error: %v", err)
	}

	if err := reviewLane.validateMoveTo("doing"); err != nil {
		t.Errorf("expected move to 'doing' to be allowed, got error: %v", err)
	}

	// Move to disallowed lane should fail
	if err := reviewLane.validateMoveTo("inbox"); err == nil {
		t.Error("expected move to 'inbox' to be disallowed")
	} else if !strings.Contains(err.Error(), "move not allowed") {
		t.Errorf("expected clear error message, got: %v", err)
	}
}

func TestValidateMoveTo_AllowedNextWildcard(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	// Configure review lane with wildcard
	reviewLane := mod.GetLane("review")
	reviewLane.AllowedNext = []string{"*"}

	// Move to any lane should succeed
	if err := reviewLane.validateMoveTo("done"); err != nil {
		t.Errorf("expected move to 'done' to be allowed with wildcard, got error: %v", err)
	}

	if err := reviewLane.validateMoveTo("inbox"); err != nil {
		t.Errorf("expected move to 'inbox' to be allowed with wildcard, got error: %v", err)
	}
}

func TestValidateMoveTo_DefaultNextLaneInOrder(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	// No allowed_next configured - should use Module.Order
	inboxLane := mod.GetLane("inbox")
	doingLane := mod.GetLane("doing")
	reviewLane := mod.GetLane("review")
	doneLane := mod.GetLane("done")

	// Move from inbox to doing (next in order) should succeed
	if err := inboxLane.validateMoveTo("doing"); err != nil {
		t.Errorf("expected move from 'inbox' to 'doing' to be allowed, got error: %v", err)
	}

	// Move from inbox to review (skip doing) should fail
	if err := inboxLane.validateMoveTo("review"); err == nil {
		t.Error("expected move from 'inbox' to 'review' to be disallowed")
	} else if !strings.Contains(err.Error(), "next lane in order") {
		t.Errorf("expected 'next lane in order' in error message, got: %v", err)
	}

	// Move from doing to review (next in order) should succeed
	if err := doingLane.validateMoveTo("review"); err != nil {
		t.Errorf("expected move from 'doing' to 'review' to be allowed, got error: %v", err)
	}

	// Move from review to done (next in order) should succeed
	if err := reviewLane.validateMoveTo("done"); err != nil {
		t.Errorf("expected move from 'review' to 'done' to be allowed, got error: %v", err)
	}

	// Move from done (last in order) should fail
	if err := doneLane.validateMoveTo("inbox"); err == nil {
		t.Error("expected move from 'done' (last lane) to be disallowed")
	} else if !strings.Contains(err.Error(), "last lane in order") {
		t.Errorf("expected 'last lane in order' in error message, got: %v", err)
	}
}

func TestValidateMoveTo_NoModuleReference(t *testing.T) {
	_, tempDir, _ := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	// Create a lane without module reference
	lane := &Lane{
		Name: "orphan",
	}

	// Should allow all (can't validate without module)
	if err := lane.validateMoveTo("any"); err != nil {
		t.Errorf("expected no error when lane has no module reference, got: %v", err)
	}
}

func TestValidateMoveTo_LaneNotInOrder(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	// Add a lane that's not in Module.Order
	newLane := &Lane{
		Name:   "custom",
		Module: mod,
	}
	mod.Lanes = append(mod.Lanes, newLane)

	// Lane not in Order should allow all (fallback for backward compatibility)
	if err := newLane.validateMoveTo("any"); err != nil {
		t.Errorf("expected no error when lane not in Order, got: %v", err)
	}
}

// ============================================================================
// WorkItem.MoveTo Tests
// ============================================================================

func TestWorkItemMoveTo_ValidationSuccess(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	inboxLane := mod.GetLane("inbox")
	doingLane := mod.GetLane("doing")

	// Create a work item in inbox
	item := createTestWorkItem(t, inboxLane, 1, "MT-0001-test-task")

	// Move to doing (next in order) should succeed
	if err := item.MoveTo("doing"); err != nil {
		t.Errorf("expected move to 'doing' to succeed, got error: %v", err)
	}

	// Verify item's lane was updated
	if item.Lane != doingLane {
		t.Errorf("expected item lane to be 'doing', got '%s'", item.Lane.Name)
	}
}

func TestWorkItemMoveTo_ValidationFailure(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	inboxLane := mod.GetLane("inbox")

	// Create a work item in inbox
	item := createTestWorkItem(t, inboxLane, 1, "MT-0001-test-task")

	// Move to review (skip doing) should fail
	err := item.MoveTo("review")
	if err == nil {
		t.Fatal("expected move to 'review' to fail")
	}

	if !strings.Contains(err.Error(), "move not allowed") {
		t.Errorf("expected 'move not allowed' in error, got: %v", err)
	}

	// Verify item's lane was NOT updated
	if item.Lane != inboxLane {
		t.Errorf("expected item lane to remain 'inbox', got '%s'", item.Lane.Name)
	}

	// Verify directory was not moved
	srcPath := filepath.Join(inboxLane.DirAbs, item.Name)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		t.Error("expected source directory to still exist")
	}
}

func TestWorkItemMoveTo_SameLaneNoOp(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	inboxLane := mod.GetLane("inbox")

	// Create a work item in inbox
	item := createTestWorkItem(t, inboxLane, 1, "MT-0001-test-task")

	// Move to same lane should be no-op
	if err := item.MoveTo("inbox"); err != nil {
		t.Errorf("expected move to same lane to be no-op, got error: %v", err)
	}
}

func TestWorkItemMoveTo_NonExistentLane(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	inboxLane := mod.GetLane("inbox")

	// Create a work item in inbox
	item := createTestWorkItem(t, inboxLane, 1, "MT-0001-test-task")

	// Move to nonexistent lane should fail
	err := item.MoveTo("nonexistent")
	if err == nil {
		t.Fatal("expected move to nonexistent lane to fail")
	}

	if !strings.Contains(err.Error(), "to_lane not found") {
		t.Errorf("expected 'to_lane not found' in error, got: %v", err)
	}
}

func TestWorkItemMoveTo_InboxItemBypassesValidation(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	inboxLane := mod.GetLane("inbox")

	// Create an inbox item (Seq = 0)
	item := &WorkItem{
		Seq:        0,
		Name:       "idea.md",
		Lane:       inboxLane,
		Attributes: make(Attributes),
	}

	// Move should return nil immediately (bypasses validation)
	if err := item.MoveTo("doing"); err != nil {
		t.Errorf("expected inbox item (Seq=0) to bypass validation, got error: %v", err)
	}
}

func TestWorkItemMoveTo_ExplicitAllowedNext(t *testing.T) {
	_, tempDir, mod := setupMoveToTest(t)
	defer os.RemoveAll(tempDir)

	// Configure review lane with explicit allowed_next
	reviewLane := mod.GetLane("review")
	reviewLane.AllowedNext = []string{"done", "doing"}
	doneLane := mod.GetLane("done")

	// Create a work item in review
	item := createTestWorkItem(t, reviewLane, 1, "MT-0001-test-task")

	// Move to done (explicitly allowed) should succeed
	// Give filesystem watcher time to settle on Windows
	time.Sleep(100 * time.Millisecond)
	if err := item.MoveTo("done"); err != nil {
		t.Errorf("expected move to 'done' to succeed, got error: %v", err)
	}

	if item.Lane != doneLane {
		t.Errorf("expected item lane to be 'done', got '%s'", item.Lane.Name)
	}
}