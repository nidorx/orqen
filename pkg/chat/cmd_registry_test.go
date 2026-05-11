package chat

import (
	"context"
	"testing"
)

func TestRegisterCommand(t *testing.T) {
	// Save original commands and restore after test
	origCommands := make(map[string]CommandDef)
	for k, v := range commands {
		origCommands[k] = v
	}
	t.Cleanup(func() {
		commands = origCommands
	})

	// Clear commands for this test
	commands = make(map[string]CommandDef)

	RegisterCommand(CommandDef{
		Name:        "testcmd",
		Description: "A test command",
		Handler: func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
			return "test response", nil
		},
	})

	cmd, ok := GetCommand("testcmd")
	if !ok {
		t.Fatal("expected command to be registered")
	}
	if cmd.Name != "testcmd" {
		t.Errorf("Name = %q, want %q", cmd.Name, "testcmd")
	}
	if cmd.Description != "A test command" {
		t.Errorf("Description = %q, want %q", cmd.Description, "A test command")
	}
}

func TestGetCommand_NotFound(t *testing.T) {
	_, ok := GetCommand("nonexistent_command_xyz")
	if ok {
		t.Error("expected false for nonexistent command")
	}
}

func TestListCommands(t *testing.T) {
	// Save original commands and restore after test
	origCommands := make(map[string]CommandDef)
	for k, v := range commands {
		origCommands[k] = v
	}
	t.Cleanup(func() {
		commands = origCommands
	})

	// Clear commands and add test ones
	commands = make(map[string]CommandDef)
	RegisterCommand(CommandDef{
		Name:        "alpha",
		Description: "First command",
		Handler:     nil,
	})
	RegisterCommand(CommandDef{
		Name:        "beta",
		Description: "Second command",
		Handler:     nil,
	})

	result := ListCommands()
	if result == "" {
		t.Fatal("expected non-empty list")
	}
	// Check both commands are present
	if !containsAny(result, "/alpha", "/beta") {
		t.Errorf("list missing commands: %s", result)
	}
	// Check ordering (alpha before beta)
	alphaIdx := indexOf(result, "/alpha")
	betaIdx := indexOf(result, "/beta")
	if alphaIdx > betaIdx {
		t.Errorf("expected alpha before beta, got %s", result)
	}
}

func TestHandleCommand_Unknown(t *testing.T) {
	resp, err := HandleCommand(context.Background(), "unknown_cmd", "", &TelegramBot{}, "user1")
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	if resp != "Unknown command. Type /help for available commands." {
		t.Errorf("expected unknown command message, got %q", resp)
	}
}

func TestHandleCommand_Dispatch(t *testing.T) {
	// Save original commands and restore after test
	origCommands := make(map[string]CommandDef)
	for k, v := range commands {
		origCommands[k] = v
	}
	t.Cleanup(func() {
		commands = origCommands
	})

	// Clear commands and add test one
	commands = make(map[string]CommandDef)
	RegisterCommand(CommandDef{
		Name:        "dispatch",
		Description: "Test dispatch",
		Handler: func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
			return "dispatched: " + args, nil
		},
	})

	resp, err := HandleCommand(context.Background(), "dispatch", "hello world", &TelegramBot{}, "user1")
	if err != nil {
		t.Fatalf("HandleCommand: %v", err)
	}
	if resp != "dispatched: hello world" {
		t.Errorf("expected 'dispatched: hello world', got %q", resp)
	}
}

func TestParseCommand_NotACommand(t *testing.T) {
	cmd, args, ok := ParseCommand("hello world")
	if ok {
		t.Errorf("expected false for non-command text, got cmd=%q args=%q", cmd, args)
	}
}

func TestParseCommand_Simple(t *testing.T) {
	cmd, args, ok := ParseCommand("/help")
	if !ok {
		t.Fatal("expected true")
	}
	if cmd != "help" {
		t.Errorf("cmd = %q, want %q", cmd, "help")
	}
	if args != "" {
		t.Errorf("args = %q, want empty", args)
	}
}

func TestParseCommand_WithArgs(t *testing.T) {
	cmd, args, ok := ParseCommand("/list doing")
	if !ok {
		t.Fatal("expected true")
	}
	if cmd != "list" {
		t.Errorf("cmd = %q, want %q", cmd, "list")
	}
	if args != "doing" {
		t.Errorf("args = %q, want %q", args, "doing")
	}
}

func TestParseCommand_MultipleArgs(t *testing.T) {
	cmd, args, ok := ParseCommand("/search hello world test")
	if !ok {
		t.Fatal("expected true")
	}
	if cmd != "search" {
		t.Errorf("cmd = %q, want %q", cmd, "search")
	}
	if args != "hello world test" {
		t.Errorf("args = %q, want %q", args, "hello world test")
	}
}

func TestParseCommand_CaseInsensitive(t *testing.T) {
	cmd, _, ok := ParseCommand("/HELP")
	if !ok {
		t.Fatal("expected true")
	}
	if cmd != "help" {
		t.Errorf("cmd = %q, want %q", cmd, "help")
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
