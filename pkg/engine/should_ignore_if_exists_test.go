package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// shouldIgnoreIfExists Tests
// ============================================================================

func TestShouldIgnoreIfExists_EmptyIgnoreList(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	doingLane.IgnoreIfExists = []string{}

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: doingLane,
	}

	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when IgnoreIfExists is empty")
	}
}

func TestShouldIgnoreIfExists_NilIgnoreList(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	doingLane.IgnoreIfExists = nil

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: doingLane,
	}

	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when IgnoreIfExists is nil")
	}
}

func TestShouldIgnoreIfExists_SameModuleLaneRef(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: readyLane,
	}

	// No items in doing lane
	doingLane.IgnoreIfExists = []string{"doing"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when referenced lane is empty")
	}

	// Create item in doing lane
	createWorkItemDir(t, doingLane, "TASK-001-existing")
	scanLaneDirectory(doingLane)

	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when referenced lane has items")
	}
}

func TestShouldIgnoreIfExists_CrossModuleLaneRef(t *testing.T) {
	project, tempDir := createTempProject(t)

	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			lane.Module = mod
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	adrModule := project.GetModule("adr")
	readyLane := taskModule.GetLane("ready")
	draftLane := adrModule.GetLane("draft")

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: readyLane,
	}

	// No items in adr.draft
	readyLane.IgnoreIfExists = []string{"adr.draft"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when cross-module lane is empty")
	}

	// Create item in draft lane
	createWorkItemDir(t, draftLane, "ADR-001-draft-title")
	scanLaneDirectory(draftLane)

	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when cross-module lane has items")
	}
}

func TestShouldIgnoreIfExists_NonexistentModule(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	readyLane.IgnoreIfExists = []string{"nonexistent.lane"}

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: readyLane,
	}

	// Nonexistent module should be skipped (continue), result false
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when referenced module does not exist")
	}
}

func TestShouldIgnoreIfExists_NonexistentLane(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	readyLane.IgnoreIfExists = []string{"task.nonexistent"}

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: readyLane,
	}

	// Nonexistent lane should be skipped (continue), result false
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when referenced lane does not exist")
	}
}

func TestShouldIgnoreIfExists_MultipleRefs(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	inboxLane := taskModule.GetLane("inbox")

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: inboxLane,
	}

	// None of the referenced lanes have items
	inboxLane.IgnoreIfExists = []string{"doing", "ready"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when none of the referenced lanes have items")
	}

	// Create item only in doing lane
	createWorkItemDir(t, doingLane, "TASK-001-existing")
	scanLaneDirectory(doingLane)

	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when at least one referenced lane has items")
	}
}

func TestShouldIgnoreIfExists_FileRefExactMatch(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create item in doing lane with specific file
	itemDir := filepath.Join(doingLane.DirAbs, "TASK-001-test")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFilePath := filepath.Join(itemDir, "summary.md")
	if err := os.WriteFile(testFilePath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// File reference: check for summary.md in doing lane
	readyLane.IgnoreIfExists = []string{"file:doing.**/summary.md"}
	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when file exists in referenced lane")
	}
}

func TestShouldIgnoreIfExists_FileRefNoMatch(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create item without the referenced file
	createWorkItemDir(t, doingLane, "TASK-001-test")
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	readyLane.IgnoreIfExists = []string{"file:doing.nonexistent.md"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when file does not exist in referenced lane")
	}
}

func TestShouldIgnoreIfExists_FileRefGlobMatch(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create item with a .md file
	itemDir := filepath.Join(doingLane.DirAbs, "TASK-001-test")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "notes.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// Glob pattern match - **/notes.md matches any path ending with notes.md
	readyLane.IgnoreIfExists = []string{"file:doing.**/*.md"}
	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when glob pattern matches a file in referenced lane")
	}
}

func TestShouldIgnoreIfExists_FileRefGlobNoMatch(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create item with a .txt file
	itemDir := filepath.Join(doingLane.DirAbs, "TASK-001-test")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "notes.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// Glob pattern that should NOT match .txt
	readyLane.IgnoreIfExists = []string{"file:doing.*.md"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when glob pattern does not match any file")
	}
}

func TestShouldIgnoreIfExists_FileRefDoubleStarGlob(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create item with nested file
	itemDir := filepath.Join(doingLane.DirAbs, "TASK-001-test")
	subDir := filepath.Join(itemDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "deep.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// ** glob should match nested files
	readyLane.IgnoreIfExists = []string{"file:doing.**/*.md"}
	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when ** glob matches nested file")
	}
}

func TestShouldIgnoreIfExists_FileRefPlainFilePath(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	// Create item in doing lane with a file
	itemDir := filepath.Join(doingLane.DirAbs, "TASK-001-test")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "summary.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(doingLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// File reference with glob that matches any path ending with summary.md in doing lane
	readyLane.IgnoreIfExists = []string{"file:doing.**/summary.md"}
	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when plain file path matches")
	}
}

func TestShouldIgnoreIfExists_MixedRefsLaneAndFile(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			lane.Module = mod
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	doingLane := taskModule.GetLane("doing")
	readyLane := taskModule.GetLane("ready")

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// No items in doing, no file in draft
	readyLane.IgnoreIfExists = []string{"doing", "file:adr.draft.**/some.md"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when none of the refs match")
	}

	// Create item in doing lane — first ref now matches
	createWorkItemDir(t, doingLane, "TASK-001-existing")
	scanLaneDirectory(doingLane)

	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when first ref (lane) matches")
	}
}

func TestShouldIgnoreIfExists_MixedRefsFileOnly(t *testing.T) {
	project, tempDir := createTempProject(t)

	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			lane.Module = mod
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	draftLane := project.GetModule("adr").GetLane("draft")

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	// No file in draft yet
	readyLane.IgnoreIfExists = []string{"file:adr.draft.**/some.md"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when file ref does not match")
	}

	// Create file in draft lane
	draftItemDir := filepath.Join(draftLane.DirAbs, "ADR-001-draft")
	if err := os.MkdirAll(draftItemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftItemDir, "some.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(draftLane)

	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when file ref matches")
	}
}

func TestShouldIgnoreIfExists_FileRefCrossModule(t *testing.T) {
	project, tempDir := createTempProject(t)

	for _, mod := range project.Modules {
		for _, lane := range mod.Lanes {
			lane.DirAbs = filepath.Join(tempDir, mod.Dir, lane.Name)
			lane.Module = mod
			if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	taskModule := project.GetModule("task")
	adrModule := project.GetModule("adr")
	readyLane := taskModule.GetLane("ready")
	draftLane := adrModule.GetLane("draft")

	// Create item in draft lane with a specific file
	draftItemDir := filepath.Join(draftLane.DirAbs, "ADR-001-draft")
	if err := os.MkdirAll(draftItemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftItemDir, "proposal.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	scanLaneDirectory(draftLane)

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-2-test",
		Lane: readyLane,
	}

	readyLane.IgnoreIfExists = []string{"file:adr.draft.**/proposal.md"}
	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when cross-module file ref matches")
	}
}

func TestShouldIgnoreIfExists_SameLaneSelfRef(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	doingLane := taskModule.GetLane("doing")

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: doingLane,
	}

	// No items yet
	doingLane.IgnoreIfExists = []string{"doing"}
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when self-referenced lane is empty")
	}

	// Add another item to the lane
	createWorkItemDir(t, doingLane, "TASK-002-existing")
	scanLaneDirectory(doingLane)

	if !shouldIgnoreIfExists(testItem) {
		t.Error("expected true when self-referenced lane has items")
	}
}

func TestShouldIgnoreIfExists_FileRefWithNonexistentModule(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	readyLane.IgnoreIfExists = []string{"file:fake.lane.file.md"}

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: readyLane,
	}

	// Nonexistent module in file ref should continue, result false
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when file ref points to nonexistent module")
	}
}

func TestShouldIgnoreIfExists_FileRefWithNonexistentLane(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	for _, lane := range taskModule.Lanes {
		lane.DirAbs = filepath.Join(tempDir, taskModule.Dir, lane.Name)
		lane.Module = taskModule
		if err := os.MkdirAll(lane.DirAbs, 0755); err != nil {
			t.Fatal(err)
		}
	}

	readyLane := taskModule.GetLane("ready")
	readyLane.IgnoreIfExists = []string{"file:task.fake.file.md"}

	testItem := &WorkItem{
		Seq:  1,
		Name: "TASK-1-test",
		Lane: readyLane,
	}

	// Nonexistent lane in file ref should continue, result false
	if shouldIgnoreIfExists(testItem) {
		t.Error("expected false when file ref points to nonexistent lane")
	}
}
