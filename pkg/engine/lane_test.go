package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// ============================================================================
// Test Helpers
// ============================================================================

// scanLaneDirectory simulates the file system scan by calling onFsysUpdate for each entry
// This is needed because tests create directories after the project is loaded
func scanLaneDirectory(lane *Lane) {
	if entries, err := os.ReadDir(lane.DirAbs); err == nil {
		for _, entry := range entries {
			info, _ := entry.Info()
			isDir := info != nil && info.IsDir()
			lane.onFsysUpdate(FsysEvent{
				Path:     filepath.Join(lane.DirAbs, entry.Name()),
				Op:       FsysOpCreate,
				Time:     info.ModTime(),
				IsDir:    isDir,
				FileInfo: info,
			})
		}
	}

	// Set ModTime to 1 minute ago for all items to avoid the executor's "recently updated" filter
	oldTime := time.Now().Add(-1 * time.Minute)
	for item := range lane.WorkItems() {
		item.ModTime = oldTime
	}
}

// ============================================================================
// Lane Tests
// ============================================================================

func TestLaneListItems(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")
	if taskModule == nil {
		t.Fatal("task module not found")
	}

	// Set up lane directories
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	if readyLane == nil {
		t.Fatal("ready lane not found")
	}

	// Initially should be empty
	items := slices.Collect(readyLane.WorkItems())
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	// Create some work items
	createWorkItemDir(t, readyLane, "TASK-001-create-project")
	createWorkItemDir(t, readyLane, "TASK-002-implement-feature")
	createWorkItemDir(t, readyLane, "not-a-task") // should be ignored

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	items = slices.Collect(readyLane.WorkItems())
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// Check item IDs
	idMap := make(map[int]bool)
	for _, item := range items {
		idMap[item.Seq] = true
	}

	if !idMap[1] || !idMap[2] {
		t.Errorf("expected items with IDs 1 and 2, got %v", idMap)
	}
}

func TestLaneHasItems(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// Initially no items
	if readyLane.HasWorkItems() {
		t.Error("expected HasItems to return false")
	}

	// Create an item
	createWorkItemDir(t, readyLane, "TASK-001-test")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	if !readyLane.HasWorkItems() {
		t.Error("expected HasItems to return true")
	}
}

func TestLaneActiveItemCount(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")
	doingLane := taskModule.GetLane("doing")
	doingLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "doing")

	if err := os.MkdirAll(doingLane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	createWorkItemDir(t, doingLane, "TASK-001-test1")
	createWorkItemDir(t, doingLane, "TASK-002-test2")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	// Initially no items are in progress
	if doingLane.CountActiveWorkItems() != 0 {
		t.Errorf("expected 0 active items, got %d", doingLane.CountActiveWorkItems())
	}

	// Get items and mark them as in progress
	items := slices.Collect(doingLane.WorkItems())
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	items[0].InProgress = true
	if doingLane.CountActiveWorkItems() != 1 {
		t.Errorf("expected 1 active item, got %d", doingLane.CountActiveWorkItems())
	}

	items[1].InProgress = true
	if doingLane.CountActiveWorkItems() != 2 {
		t.Errorf("expected 2 active items, got %d", doingLane.CountActiveWorkItems())
	}
}

func TestLaneHasAvailableSlot(t *testing.T) {
	project, tempDir := createTempProject(t)
	defer os.RemoveAll(tempDir)

	taskModule := project.GetModule("task")
	doingLane := taskModule.GetLane("doing")
	doingLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "doing")
	doingLane.MaxAgents = 2

	if err := os.MkdirAll(doingLane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	createWorkItemDir(t, doingLane, "TASK-001-test")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	items := slices.Collect(doingLane.WorkItems())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	// Should have slot available (0 active < 2 max)
	if !doingLane.HasAvailableSlot() {
		t.Error("expected HasAvailableSlot to return true")
	}

	// Mark as in progress
	items[0].InProgress = true

	// Should still have slot (1 active < 2 max)
	if !doingLane.HasAvailableSlot() {
		t.Error("expected HasAvailableSlot to return true")
	}

	// Add another item and mark as in progress
	createWorkItemDir(t, doingLane, "TASK-002-test2")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	items = slices.Collect(doingLane.WorkItems())
	for _, item := range items {
		item.InProgress = true
	}

	// Should not have slot (2 active = 2 max)
	if doingLane.HasAvailableSlot() {
		t.Error("expected HasAvailableSlot to return false")
	}
}

// ============================================================================
// Lane Reference Parsing Tests
// ============================================================================

func TestParseLaneReference(t *testing.T) {
	tests := []struct {
		input      string
		wantModule string
		wantLane   string
		wantFile   string
	}{
		// Standard lane references
		{"doing", "", "doing", ""},
		{"adr.draft", "adr", "draft", ""},
		{"task.ready", "task", "ready", ""},
		{"", "", "", ""},

		// File references - just file path
		{"file:file", "", "", "file"},
		{"file:file.ext", "", "", "file.ext"},
		{"file:path/to/file", "", "", "path/to/file"},
		{"file:path/to/file.ext", "", "", "path/to/file.ext"},

		// File references - lane.file
		{"file:lane_name.file.ext", "", "lane_name", "file.ext"},
		{"file:lane_name.path/to/file.ext", "", "lane_name", "path/to/file.ext"},

		// File references - module.lane.file
		{"file:module.lane_name.file.ext", "module", "lane_name", "file.ext"},
		{"file:module.lane_name.path/to/file.ext", "module", "lane_name", "path/to/file.ext"},
		{"file:module.lane_name.path/to/file.*", "module", "lane_name", "path/to/file.*"},
		{"file:module.lane_name.path/to/*.ext", "module", "lane_name", "path/to/*.ext"},
		{"file:module.lane_name.path/to/*.*", "module", "lane_name", "path/to/*.*"},
		{"file:module.lane_name.**/*.*", "module", "lane_name", "**/*.*"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotModule, gotLane, gotFile := laneParseReference(tt.input)
			if gotModule != tt.wantModule {
				t.Errorf("module = %q, want %q", gotModule, tt.wantModule)
			}
			if gotLane != tt.wantLane {
				t.Errorf("lane = %q, want %q", gotLane, tt.wantLane)
			}
			if gotFile != tt.wantFile {
				t.Errorf("file = %q, want %q", gotFile, tt.wantFile)
			}
		})
	}
}

// ============================================================================
// Lane Additional Tests
// ============================================================================

func TestLaneItemCount(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// Initially 0
	if readyLane.CountWorkItems() != 0 {
		t.Errorf("expected 0 items, got %d", readyLane.CountWorkItems())
	}

	// Create items
	createWorkItemDir(t, readyLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	if readyLane.CountWorkItems() != 2 {
		t.Errorf("expected 2 items, got %d", readyLane.CountWorkItems())
	}
}

func TestLaneGetItem(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	createWorkItemDir(t, readyLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")

	// Scan to populate cache
	scanLaneDirectory(readyLane)

	// Get by ID
	item := taskModule.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item with ID 1")
	}
	if item.Name != "TASK-001-test1" {
		t.Errorf("expected TASK-001-test1, got %s", item.Name)
	}

	// Get nonexistent item
	item = taskModule.GetWorkItemBySeq(999)
	if item != nil {
		t.Error("expected nil for nonexistent item")
	}
}

func TestResolveLanePath(t *testing.T) {
	project, _ := createTempProject(t)
	taskModule := project.GetModule("task")

	// Same module
	lane := laneResolvePath(project, taskModule, "doing")
	if lane == nil || lane.Name != "doing" {
		t.Error("expected to resolve doing lane")
	}

	// Cross module
	_ = project.GetModule("adr")
	lane = laneResolvePath(project, taskModule, "adr.draft")
	if lane == nil || lane.Name != "draft" {
		t.Error("expected to resolve adr.draft lane")
	}

	// Nonexistent
	lane = laneResolvePath(project, taskModule, "nonexistent")
	if lane != nil {
		t.Error("expected nil for nonexistent lane")
	}

	// Nonexistent module
	lane = laneResolvePath(project, taskModule, "fake.lane")
	if lane != nil {
		t.Error("expected nil for nonexistent module")
	}
}

func TestHasDependencyInReferencedLanes(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories and module references
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")

	// Create a dependency item
	createWorkItemDir(t, doingLane, "TASK-001-dep")

	// Scan to populate cache
	scanLaneDirectory(doingLane)

	items := slices.Collect(doingLane.WorkItems())
	if len(items) == 0 {
		t.Fatal("expected item in doing lane")
	}

	// Test with item that has dependency in doing lane
	testItem := &WorkItem{
		Seq:        999,
		Name:       "TASK-999-test",
		Lane:       taskModule.GetLane("ready"),
		Attributes: make(Attributes),
	}

	testItem.Attributes.Set("dependencies", []string{strconv.Itoa(items[0].Seq)})

	readyLane := taskModule.GetLane("ready")
	readyLane.IgnoreIfDependency = []string{"doing"}

	if !shouldIgnoreIfDependency(testItem) {
		t.Error("expected true when dependency is in referenced lane")
	}

	// Test with item that has no matching dependency
	testItem2 := &WorkItem{
		Seq:  998,
		Name: "TASK-998-test",
		Lane: taskModule.GetLane("ready"),
	}

	if shouldIgnoreIfDependency(testItem2) {
		t.Error("expected false when no dependencies match")
	}
}
