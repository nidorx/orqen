package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldIgnoreIfNotExists(t *testing.T) {
	project, tempDir := createTempProject(t)
	taskModule := project.GetModule("task")

	for _, laneName := range []string{"ready", "doing"} {
		lane := taskModule.GetLane(laneName)
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	testItem := &WorkItem{
		Seq:        1,
		Name:       "TASK-1-test",
		Lane:       doingLane,
		Attributes: make(Attributes),
	}

	// No items in any lane — should return true (doing has no items)
	doingLane.IgnoreIfNotExists = []string{"doing"}
	if !shouldIgnoreIfNotExists(testItem) {
		t.Error("expected true when doing lane is empty")
	}

	// Create item in doing lane and scan
	createWorkItemDir(t, doingLane, "TASK-001-test")
	scanLaneDirectory(doingLane)

	// Now doing has items — should return false
	if shouldIgnoreIfNotExists(testItem) {
		t.Error("expected false when doing lane has items")
	}

	// Ready lane is still empty — should return true
	doingLane.IgnoreIfNotExists = []string{"ready"}
	if !shouldIgnoreIfNotExists(testItem) {
		t.Error("expected true when ready lane is empty")
	}

	// Test multiple refs: returns true if ANY referenced lane is empty
	// doing has items, ready is empty → should return true
	doingLane.IgnoreIfNotExists = []string{"doing", "ready"}
	if !shouldIgnoreIfNotExists(testItem) {
		t.Error("expected true when at least one referenced lane is empty")
	}

	// Create item in ready lane too
	createWorkItemDir(t, readyLane, "TASK-002-test")
	scanLaneDirectory(readyLane)

	// Now both have items — should return false
	if shouldIgnoreIfNotExists(testItem) {
		t.Error("expected false when all referenced lanes have items")
	}
}

func TestShouldIgnoreIfNotExistsWithFiles(t *testing.T) {
	project, tempDir := createTempProject(t)
	taskModule := project.GetModule("task")

	for _, laneName := range []string{"ready", "doing"} {
		lane := taskModule.GetLane(laneName)
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")

	// Create an item in doing lane with a file
	createWorkItemDir(t, doingLane, "TASK-001-test")
	itemDir := filepath.Join(doingLane.DirAbs, "TASK-001-test")
	artifactsDir := filepath.Join(itemDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "test.md"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:        1,
		Name:       "TASK-1-test",
		Lane:       doingLane,
		Attributes: make(Attributes),
	}

	// Test exact file reference — item has artifacts/test.md
	// Files are stored as full relative paths, so use glob to match
	doingLane.IgnoreIfNotExists = []string{"file:**/artifacts/test.md"}

	// Item has the file, so should return false
	if shouldIgnoreIfNotExists(testItem) {
		t.Error("expected false when the referenced file exists")
	}

	// Test exact file reference — item does NOT have the file
	doingLane.IgnoreIfNotExists = []string{"file:**/other.md"}
	if !shouldIgnoreIfNotExists(testItem) {
		t.Error("expected true when the referenced file does not exist in any item")
	}

	// Test glob pattern — matching files exist (*.md matches artifacts/test.md)
	doingLane.IgnoreIfNotExists = []string{"file:**/*.md"}
	if shouldIgnoreIfNotExists(testItem) {
		t.Error("expected false when files match glob pattern")
	}

	// Test glob pattern — no matching files (*.txt does not match)
	doingLane.IgnoreIfNotExists = []string{"file:**/*.txt"}
	if !shouldIgnoreIfNotExists(testItem) {
		t.Error("expected true when no files match glob pattern")
	}
}
