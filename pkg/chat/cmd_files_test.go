package chat

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleFiles_NoProject(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleFiles(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleFiles: %v", err)
	}
	if resp != "No project loaded." {
		t.Errorf("expected 'No project loaded.', got %q", resp)
	}
}

func TestHandleFiles_BlockedPath(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		DirAbs:  t.TempDir(),
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleFiles(context.Background(), ".orqen", bot, "user1")
	if err != nil {
		t.Fatalf("handleFiles: %v", err)
	}
	if !containsAny(resp, "Access denied", "protected") {
		t.Errorf("expected access denied message, got %q", resp)
	}
}

func TestHandleFiles_Registered(t *testing.T) {
	cmd, ok := GetCommand("files")
	if !ok {
		t.Fatal("command 'files' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
