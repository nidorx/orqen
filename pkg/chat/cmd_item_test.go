package chat

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleItem_NoProject(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleItem(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleItem: %v", err)
	}
	if resp != "No project loaded." {
		t.Errorf("expected 'No project loaded.', got %q", resp)
	}
}

func TestHandleItem_NotFound(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleItem(context.Background(), "NONEXISTENT", bot, "user1")
	if err != nil {
		t.Fatalf("handleItem: %v", err)
	}
	if resp != `Workitem "NONEXISTENT" not found.` {
		t.Errorf("expected not found message, got %q", resp)
	}
}

func TestHandleItem_Registered(t *testing.T) {
	cmd, ok := GetCommand("item")
	if !ok {
		t.Fatal("command 'item' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
