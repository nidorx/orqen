package cmd

import (
	"context"
	"strings"
	"testing"
)

func TestHandleHelp(t *testing.T) {
	// Register a test command for this test
	Register(Command{
		Name:        "testcmd",
		Description: "A test command",
		Handler: func(ctx context.Context, req *Request) (string, error) {
			return "test", nil
		},
	})

	resp, err := helpCommandHandler(context.Background(), &Request{ExtId: "user1"})
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
	cmd, ok := Get("help")
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

func TestHandleHelp_ListCommands(t *testing.T) {
	// Save original commands and restore after test
	origCommands := make(map[string]Command)
	for k, v := range commands {
		origCommands[k] = v
	}
	t.Cleanup(func() {
		commands = origCommands
	})

	// Clear commands and add test ones
	commands = make(map[string]Command)
	Register(Command{
		Name:        "alpha",
		Description: "First command",
		Handler:     nil,
	})
	Register(Command{
		Name:        "beta",
		Description: "Second command",
		Handler:     nil,
	})

	result, _ := helpCommandHandler(context.Background(), &Request{ExtId: "user1"})
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
