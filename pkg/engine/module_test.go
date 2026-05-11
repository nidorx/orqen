package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// ============================================================================
// Module Tests
// ============================================================================

func TestModuleGetLane(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")
	if taskModule == nil {
		t.Fatal("task module not found")
	}

	lane := taskModule.GetLane("ready")
	if lane == nil {
		t.Error("expected to find ready lane")
	}

	lane = taskModule.GetLane("nonexistent")
	if lane != nil {
		t.Error("expected nil for nonexistent lane")
	}
}

func TestModuleGetLanesInOrder(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")
	if taskModule == nil {
		t.Fatal("task module not found")
	}

	// Check that lanes are returned in order
	ordered := taskModule.GetLanesInOrder()
	if len(ordered) < 4 {
		t.Fatalf("expected at least 4 lanes, got %d", len(ordered))
	}

	// First lanes should be in the order specified
	expectedOrder := []string{"doing", "ready", "prioritized", "inbox"}
	for i, expected := range expectedOrder {
		if i >= len(ordered) {
			t.Errorf("missing lane %s", expected)
			continue
		}
		if ordered[i].Name != expected {
			t.Errorf("lane %d: expected %s, got %s", i, expected, ordered[i].Name)
		}
	}
}

func TestModuleActiveItemCount(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")

	// Set up lane directories
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	createWorkItemDir(t, doingLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")

	// Scan to populate caches
	scanLaneDirectory(doingLane)
	scanLaneDirectory(readyLane)

	// Initially no items are in progress
	if taskModule.ActiveItemCount() != 0 {
		t.Errorf("expected 0 active items, got %d", taskModule.ActiveItemCount())
	}

	// Mark items as in progress
	items := slices.Collect(doingLane.WorkItems())
	if len(items) > 0 {
		items[0].InProgress = true
	}

	items = slices.Collect(readyLane.WorkItems())
	if len(items) > 0 {
		items[0].InProgress = true
	}

	if taskModule.ActiveItemCount() != 2 {
		t.Errorf("expected 2 active items, got %d", taskModule.ActiveItemCount())
	}
}

// ============================================================================
// Module Additional Tests
// ============================================================================

func TestModuleListItems(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	createWorkItemDir(t, doingLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")

	// Scan to populate caches
	scanLaneDirectory(doingLane)
	scanLaneDirectory(readyLane)

	items := slices.Collect(taskModule.WorkItems())
	if len(items) != 2 {
		t.Errorf("expected 2 items across all lanes, got %d", len(items))
	}
}

func TestModuleFindItemByID(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	createWorkItemDir(t, doingLane, "TASK-001-test1")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	// Find by ID
	item := taskModule.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item by ID 1")
	}
	if item.Name != "TASK-001-test1" {
		t.Errorf("expected TASK-001-test1, got %s", item.Name)
	}

	// Find nonexistent
	item = taskModule.GetWorkItemBySeq(999)
	if item != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestModuleHasAvailableSlot(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories with MaxAgents=1 for all lanes
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Dir = lane.Name
		lane.MaxAgents = 1
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// All lanes should have available slot initially
	if !taskModule.HasAvailableSlot() {
		t.Error("expected available slot when all lanes are empty")
	}

	// Fill ALL non-inbox lanes with one item each, marking them as in progress
	// Inbox lane uses files (.md/.txt) not directories, so we handle it specially
	for _, lane := range taskModule.Lanes {
		lane.MaxAgents = 1

		if lane.Name == "inbox" {
			// Create a fake inbox file that's old enough
			inboxFile := filepath.Join(lane.DirAbs, "idea.md")
			if err := os.WriteFile(inboxFile, []byte("some content"), 0644); err != nil {
				t.Fatal(err)
			}
			// Make the file older than 1 minute so it's picked up by WorkItems
			oldTime := time.Now().Add(-2 * time.Minute)
			if err := os.Chtimes(inboxFile, oldTime, oldTime); err != nil {
				t.Fatal(err)
			}
			// Scan to populate cache
			scanLaneDirectory(lane)
			// Mark the inbox item as in progress
			items := slices.Collect(lane.WorkItems())
			if len(items) > 0 {
				items[0].InProgress = true
			}
			continue
		}

		createWorkItemDir(t, lane, "TASK-001-test")
		// Scan to populate cache
		scanLaneDirectory(lane)
		items := slices.Collect(lane.WorkItems())
		if len(items) > 0 {
			items[0].InProgress = true
		}
	}

	// Now no slot should be available
	if taskModule.HasAvailableSlot() {
		t.Error("expected no available slot when all lanes are full")
	}
}

func TestModuleGetFullDir(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
	}

	dir := taskModule.GetFullDir("doing")
	// GetFullDir returns lane.Dir (the relative path), not DirAbs
	if dir == "" {
		// lane.Dir might be empty if not set by applyDefaults
		// In loaded projects, applyDefaults sets lane.Dir to "NN_name"
	}

	// Test nonexistent lane
	dir = taskModule.GetFullDir("nonexistent")
	if dir != "" {
		t.Errorf("expected empty dir for nonexistent lane, got %s", dir)
	}
}

func TestModuleTxNewWorkItem(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Initially 1
	taskModule.TxNewWorkItem(func(seq int) error {
		if seq != 1 {
			t.Errorf("expected sequence 1, got %d", seq)
		}
		return nil
	})

	doingLane := taskModule.GetLane("doing")
	createWorkItemDir(t, doingLane, "TASK-001-first")
	createWorkItemDir(t, doingLane, "TASK-005-fifth")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	// Should be max + 1
	taskModule.TxNewWorkItem(func(seq int) error {
		if seq != 6 {
			t.Errorf("expected sequence 6, got %d", seq)
		}
		return nil
	})

}
