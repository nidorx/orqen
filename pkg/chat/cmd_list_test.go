package chat

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleList_NoProject(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleList(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if resp != "No project loaded." {
		t.Errorf("expected 'No project loaded.', got %q", resp)
	}
}

func TestHandleList_EmptyProject(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleList(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if !containsAny(resp, "No workitems found") {
		t.Errorf("expected 'No workitems found', got %q", resp)
	}
}

func TestHandleList_LaneNotFound(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleList(context.Background(), "nonexistent", bot, "user1")
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	if resp != `Lane "nonexistent" not found.` {
		t.Errorf("expected lane not found message, got %q", resp)
	}
}
