package chat

import (
	"context"
	"strings"
	"testing"
)

func TestHandleHelp(t *testing.T) {
	// Register a test command for this test
	RegisterCommand(CommandDef{
		Name:        "testcmd",
		Description: "A test command",
		Handler: func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
			return "test", nil
		},
	})

	bot := &TelegramBot{}
	resp, err := handleHelp(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleHelp: %v", err)
	}
	if resp == "" {
		t.Fatal("expected non-empty response")
	}
	// Should list the test command
	if !strings.Contains(resp, "/testcmd") {
		t.Errorf("response missing /testcmd: %s", resp)
	}
}

func TestHandleHelp_Registered(t *testing.T) {
	cmd, ok := GetCommand("help")
	if !ok {
		t.Fatal("command 'help' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
