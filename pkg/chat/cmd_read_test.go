package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleRead_NoProject(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleRead(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleRead: %v", err)
	}
	if resp != "No project loaded." {
		t.Errorf("expected 'No project loaded.', got %q", resp)
	}
}

func TestHandleRead_BlockedPath(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		DirAbs:  t.TempDir(),
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleRead(context.Background(), ".git/config", bot, "user1")
	if err != nil {
		t.Fatalf("handleRead: %v", err)
	}
	if !containsAny(resp, "Access denied", "protected") {
		t.Errorf("expected access denied message, got %q", resp)
	}
}

func TestHandleRead_FileContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Hello, World!"), 0644)

	proj := &engine.Project{
		Id:      "test-project",
		DirAbs:  tmpDir,
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleRead(context.Background(), "test.txt", bot, "user1")
	if err != nil {
		t.Fatalf("handleRead: %v", err)
	}
	if !containsAny(resp, "Hello, World!") {
		t.Errorf("expected file content, got %q", resp)
	}
}

func TestHandleRead_Registered(t *testing.T) {
	cmd, ok := GetCommand("read")
	if !ok {
		t.Fatal("command 'read' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
