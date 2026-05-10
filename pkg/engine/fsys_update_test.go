package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ============================================================================
// onFsysUpdate Tests
// ============================================================================

func TestOnFsysUpdate_CreateWorkItemDir(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a work item directory
	itemDir := filepath.Join(readyLane.DirAbs, "TASK-001-new-item")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Simulate file system event
	info, _ := os.Stat(itemDir)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     itemDir,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	// Verify item was added
	if readyLane.CountWorkItems() != 1 {
		t.Errorf("expected 1 item, got %d", readyLane.CountWorkItems())
	}

	item := taskModule.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item with Seq=1")
	}
	if item.Name != "TASK-001-new-item" {
		t.Errorf("expected item name 'TASK-001-new-item', got %s", item.Name)
	}
}

func TestOnFsysUpdate_CreateFileInWorkItem(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create work item directory
	itemDir := filepath.Join(readyLane.DirAbs, "TASK-001-test")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	// First event: create directory
	info, _ := os.Stat(itemDir)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     itemDir,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	// Second event: create file inside
	filePath := filepath.Join(itemDir, "TASK-001-CONTENT.md")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	fileInfo, _ := os.Stat(filePath)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     filePath,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    false,
		FileInfo: fileInfo,
	})

	// Verify file was added to item
	item := taskModule.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item")
	}
	if len(item.Files) != 1 {
		t.Errorf("expected 1 file in item, got %d", len(item.Files))
	}
}

func TestOnFsysUpdate_RemoveWorkItemDir(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create work item directory
	itemDir := filepath.Join(readyLane.DirAbs, "TASK-001-to-remove")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create event
	info, _ := os.Stat(itemDir)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     itemDir,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	if readyLane.CountWorkItems() != 1 {
		t.Fatalf("expected 1 item before removal, got %d", readyLane.CountWorkItems())
	}

	// Remove event
	readyLane.onFsysUpdate(FsysEvent{
		Path:  itemDir,
		Op:    FsysOpRemove,
		Time:  time.Now(),
		IsDir: true,
	})

	// Verify item was removed
	if readyLane.CountWorkItems() != 0 {
		t.Errorf("expected 0 items after removal, got %d", readyLane.CountWorkItems())
	}
}

func TestOnFsysUpdate_RenameWorkItemDir(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create work item directory
	oldPath := filepath.Join(readyLane.DirAbs, "TASK-001-old")
	if err := os.MkdirAll(oldPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create event
	info, _ := os.Stat(oldPath)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     oldPath,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	if readyLane.CountWorkItems() != 1 {
		t.Fatalf("expected 1 item before rename, got %d", readyLane.CountWorkItems())
	}

	// Rename event
	readyLane.onFsysUpdate(FsysEvent{
		Path:  oldPath,
		Op:    FsysOpRename,
		Time:  time.Now(),
		IsDir: true,
	})

	// Verify item was removed (rename is treated as removal)
	if readyLane.CountWorkItems() != 0 {
		t.Errorf("expected 0 items after rename, got %d", readyLane.CountWorkItems())
	}
}

func TestOnFsysUpdate_InboxFile(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	inboxLane := taskModule.GetLane("inbox")
	inboxLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "inbox")

	if err := os.MkdirAll(inboxLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .md file in inbox
	filePath := filepath.Join(inboxLane.DirAbs, "my-idea.md")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(filePath)
	inboxLane.onFsysUpdate(FsysEvent{
		Path:     filePath,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    false,
		FileInfo: info,
	})

	// Verify item was added (inbox items have Seq=0)
	if inboxLane.CountWorkItems() != 1 {
		t.Errorf("expected 1 item, got %d", inboxLane.CountWorkItems())
	}

	items := collectWorkItems(inboxLane.WorkItems())
	if len(items) != 1 {
		t.Fatalf("expected 1 item")
	}
	if items[0].Seq != 0 {
		t.Errorf("expected Seq=0 for inbox item, got %d", items[0].Seq)
	}
	if items[0].Name != "my-idea.md" {
		t.Errorf("expected item name 'my-idea.md', got %s", items[0].Name)
	}
}

func TestOnFsysUpdate_PreservesInProgressState(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create work item directory
	itemDir := filepath.Join(readyLane.DirAbs, "TASK-001-test")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create event
	info, _ := os.Stat(itemDir)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     itemDir,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	// Mark item as in progress
	item := taskModule.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item")
	}
	item.InProgress = true

	// Simulate another create event (e.g., file modification)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     itemDir,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	// Verify InProgress state was preserved
	item = taskModule.GetWorkItemBySeq(1)
	if item == nil {
		t.Fatal("expected to find item after update")
	}
	if !item.InProgress {
		t.Error("expected InProgress state to be preserved")
	}
}

func TestOnFsysUpdate_IgnoreNonWorkItemDir(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	readyLane := taskModule.GetLane("ready")
	readyLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "ready")

	if err := os.MkdirAll(readyLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a non-work-item directory
	otherDir := filepath.Join(readyLane.DirAbs, "not-a-task")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(otherDir)
	readyLane.onFsysUpdate(FsysEvent{
		Path:     otherDir,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    true,
		FileInfo: info,
	})

	// Verify no item was added
	if readyLane.CountWorkItems() != 0 {
		t.Errorf("expected 0 items, got %d", readyLane.CountWorkItems())
	}
}

func TestOnFsysUpdate_IgnoreWrongExtensionInInbox(t *testing.T) {
	project, tempDir := createTempProject(t)

	taskModule := project.GetModule("task")
	inboxLane := taskModule.GetLane("inbox")
	inboxLane.DirAbs = filepath.Join(tempDir, taskModule.Dir, "inbox")

	if err := os.MkdirAll(inboxLane.DirAbs, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .txt file (should be accepted)
	txtPath := filepath.Join(inboxLane.DirAbs, "idea.txt")
	if err := os.WriteFile(txtPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(txtPath)
	inboxLane.onFsysUpdate(FsysEvent{
		Path:     txtPath,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    false,
		FileInfo: info,
	})

	if inboxLane.CountWorkItems() != 1 {
		t.Fatalf("expected 1 item for .txt file, got %d", inboxLane.CountWorkItems())
	}

	// Create .json file (should be ignored)
	jsonPath := filepath.Join(inboxLane.DirAbs, "config.json")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	info, _ = os.Stat(jsonPath)
	inboxLane.onFsysUpdate(FsysEvent{
		Path:     jsonPath,
		Op:       FsysOpCreate,
		Time:     time.Now(),
		IsDir:    false,
		FileInfo: info,
	})

	// Should still have only 1 item
	if inboxLane.CountWorkItems() != 1 {
		t.Errorf("expected 1 item (ignoring .json), got %d", inboxLane.CountWorkItems())
	}
}
