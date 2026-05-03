package project

import (
	"os"
	"path/filepath"
	"testing"
)

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
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	if readyLane == nil {
		t.Fatal("ready lane not found")
	}

	// Initially should be empty
	items := readyLane.ListItems()
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}

	// Create some work items
	createWorkItemDir(t, readyLane, "TASK-001-create-project")
	createWorkItemDir(t, readyLane, "TASK-002-implement-feature")
	createWorkItemDir(t, readyLane, "not-a-task") // should be ignored

	items = readyLane.ListItems()
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

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Initially no items
	if readyLane.HasWorkItems() {
		t.Error("expected HasItems to return false")
	}

	// Create an item
	createWorkItemDir(t, readyLane, "TASK-001-test")

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

	if err := os.MkdirAll(doingLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	createWorkItemDir(t, doingLane, "TASK-001-test1")
	createWorkItemDir(t, doingLane, "TASK-002-test2")

	// Initially no items are in progress
	if doingLane.CountActiveWorkItems() != 0 {
		t.Errorf("expected 0 active items, got %d", doingLane.CountActiveWorkItems())
	}

	// Get items and mark them as in progress
	items := doingLane.ListItems()
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

	if err := os.MkdirAll(doingLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	createWorkItemDir(t, doingLane, "TASK-001-test")

	items := doingLane.ListItems()
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
	items = doingLane.ListItems()
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
			gotModule, gotLane, gotFile := ParseLaneReference(tt.input)
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

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Initially 0
	if readyLane.CountWorkItems() != 0 {
		t.Errorf("expected 0 items, got %d", readyLane.CountWorkItems())
	}

	// Create items
	createWorkItemDir(t, readyLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")

	if readyLane.CountWorkItems() != 2 {
		t.Errorf("expected 2 items, got %d", readyLane.CountWorkItems())
	}
}

func TestLaneGetItem(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	createWorkItemDir(t, readyLane, "TASK-001-test1")
	createWorkItemDir(t, readyLane, "TASK-002-test2")

	// Get by ID
	item := readyLane.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item with ID 1")
	}
	if item.Name != "TASK-001-test1" {
		t.Errorf("expected TASK-001-test1, got %s", item.Name)
	}

	// Get nonexistent item
	item = readyLane.GetWorkItemBySeq(999)
	if item != nil {
		t.Error("expected nil for nonexistent item")
	}
}

func TestLaneFindItemDependencies(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories and module references
	// Note: FindItemDependencies uses l.Dir (not l.DirAbs), so we set Dir to the absolute path
	for _, lane := range taskModule.Lanes {
		lane.Dir = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create a dependency item in doing lane
	createWorkItemDir(t, doingLane, "TASK-001-dependency")

	// Create an item in ready lane
	createWorkItemDir(t, readyLane, "TASK-002-dependent")
	depFile := filepath.Join(readyLane.Dir, "TASK-002-dependent", "DEP_001")
	if err := os.WriteFile(depFile, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}

	// List items to populate caches
	doingItems := doingLane.ListItems()
	readyItems := readyLane.ListItems()

	if len(doingItems) == 0 || len(readyItems) == 0 {
		t.Fatal("expected items in both lanes")
	}

	// Find dependencies
	deps := readyLane.FindItemDependencies(readyItems[0])
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].Seq != 1 {
		t.Errorf("expected dependency ID 1, got %d", deps[0].Seq)
	}
}

func TestHasItemsInReferencedLanes(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")

	// Set up lane directories and module references
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")

	// No items in any lane
	if doingLane.HasItemsInReferencedLanes([]string{"doing"}) {
		t.Error("expected false when doing lane is empty")
	}

	// Create item in doing lane
	createWorkItemDir(t, doingLane, "TASK-001-test")

	// Now should return true
	if !doingLane.HasItemsInReferencedLanes([]string{"doing"}) {
		t.Error("expected true when doing lane has items")
	}

	// Ready lane should be empty
	if doingLane.HasItemsInReferencedLanes([]string{"ready"}) {
		t.Error("expected false when ready lane is empty")
	}

	// Test multiple refs
	if !doingLane.HasItemsInReferencedLanes([]string{"ready", "doing"}) {
		t.Error("expected true when at least one ref has items")
	}
}

func TestResolveLanePath(t *testing.T) {
	project, _ := createTempProject(t)
	taskModule := project.GetModule("task")

	// Same module
	lane := ResolveLanePath(project, taskModule, "doing")
	if lane == nil || lane.Name != "doing" {
		t.Error("expected to resolve doing lane")
	}

	// Cross module
	_ = project.GetModule("adr")
	lane = ResolveLanePath(project, taskModule, "adr.draft")
	if lane == nil || lane.Name != "draft" {
		t.Error("expected to resolve adr.draft lane")
	}

	// Nonexistent
	lane = ResolveLanePath(project, taskModule, "nonexistent")
	if lane != nil {
		t.Error("expected nil for nonexistent lane")
	}

	// Nonexistent module
	lane = ResolveLanePath(project, taskModule, "fake.lane")
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
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")

	// Create a dependency item
	createWorkItemDir(t, doingLane, "TASK-001-dep")
	items := doingLane.ListItems()
	if len(items) == 0 {
		t.Fatal("expected item in doing lane")
	}

	// Test with item that has dependency in doing lane
	testItem := &WorkItem{
		Seq:          999,
		Name:         "TASK-999-test",
		Dependencies: []*WorkItem{items[0]},
	}

	if !HasDependencyInReferencedLanes(project, taskModule, testItem, []string{"doing"}) {
		t.Error("expected true when dependency is in referenced lane")
	}

	// Test with item that has no matching dependency
	testItem2 := &WorkItem{
		Seq:          998,
		Name:         "TASK-998-test",
		Dependencies: []*WorkItem{},
	}

	if HasDependencyInReferencedLanes(project, taskModule, testItem2, []string{"doing"}) {
		t.Error("expected false when no dependencies match")
	}
}
