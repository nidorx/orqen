package chat

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestHandleStatus_NoProject(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleStatus(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	if resp != "No project loaded." {
		t.Errorf("expected 'No project loaded.', got %q", resp)
	}
}

func TestHandleStatus_WithProject(t *testing.T) {
	proj := &engine.Project{
		Id:      "test-project",
		Modules: []*engine.Module{},
	}
	bot := &TelegramBot{Project: proj}
	resp, err := handleStatus(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	if !containsAny(resp, "Project:", "test-project", "Status:") {
		t.Errorf("response missing expected content: %s", resp)
	}
}
