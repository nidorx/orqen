package agent

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// Helper to create string pointers
func strPtr(s string) *string {
	return &s
}

func TestGenericClient_SessionUpdate_AgentMessageChunk(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	text := "Hello world"
	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
				Content: acp.ContentBlock{
					Text: &acp.ContentBlockText{
						Text: text,
					},
				},
			},
		},
	}

	err := client.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericClient_SessionUpdate_ToolCall(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	title := "ReadFile: test.txt"
	status := acp.ToolCallStatusInProgress
	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				Title:  title,
				Status: status,
			},
		},
	}

	err := client.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericClient_SessionUpdate_ToolCallUpdate(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	status := acp.ToolCallStatusCompleted
	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			ToolCallUpdate: &acp.SessionToolCallUpdate{
				ToolCallId: "call_abc123",
				Status:     &status,
			},
		},
	}

	err := client.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericClient_SessionUpdate_AgentThoughtChunk(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	thought := "I need to read the file"
	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			AgentThoughtChunk: &acp.SessionUpdateAgentThoughtChunk{
				Content: acp.ContentBlock{
					Text: &acp.ContentBlockText{
						Text: thought,
					},
				},
			},
		},
	}

	err := client.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericClient_SessionUpdate_Plan(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			Plan: &acp.SessionUpdatePlan{},
		},
	}

	err := client.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericClient_SessionUpdate_UserMessageChunk(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	params := acp.SessionNotification{
		Update: acp.SessionUpdate{
			UserMessageChunk: &acp.SessionUpdateUserMessageChunk{},
		},
	}

	err := client.SessionUpdate(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericClient_WriteTextFile_Success(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	// Use temp directory for test
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"

	params := acp.WriteTextFileRequest{
		Path:    filePath,
		Content: "Hello, World!",
	}

	resp, err := client.WriteTextFile(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	_ = resp

	// Verify file was created
	content, err := readFile(filePath)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if content != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", content)
	}
}

func TestGenericClient_WriteTextFile_RelativePath(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	params := acp.WriteTextFileRequest{
		Path:    "relative/path/test.txt",
		Content: "test",
	}

	_, err := client.WriteTextFile(ctx, params)
	if err == nil {
		t.Fatal("expected error for relative path, got nil")
	}
}

func TestGenericClient_ReadTextFile_Success(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	// Create test file
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	err := writeFile(filePath, "Line 1\nLine 2\nLine 3")
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	params := acp.ReadTextFileRequest{
		Path: filePath,
	}

	resp, err := client.ReadTextFile(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Line 1\nLine 2\nLine 3" {
		t.Errorf("expected full content, got '%s'", resp.Content)
	}
}

func TestGenericClient_ReadTextFile_WithLineAndLimit(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	// Create test file
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test.txt"
	err := writeFile(filePath, "Line 1\nLine 2\nLine 3\nLine 4\nLine 5")
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	line := 2
	limit := 2
	params := acp.ReadTextFileRequest{
		Path:  filePath,
		Line:  &line,
		Limit: &limit,
	}

	resp, err := client.ReadTextFile(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return lines 2-3 (0-indexed: lines[1:3])
	expected := "Line 2\nLine 3"
	if resp.Content != expected {
		t.Errorf("expected '%s', got '%s'", expected, resp.Content)
	}
}

func TestGenericClient_ReadTextFile_RelativePath(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	params := acp.ReadTextFileRequest{
		Path: "relative/path/test.txt",
	}

	_, err := client.ReadTextFile(ctx, params)
	if err == nil {
		t.Fatal("expected error for relative path, got nil")
	}
}

func TestGenericClient_ReadTextFile_NonExistent(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	params := acp.ReadTextFileRequest{
		Path: tmpDir + "/nonexistent.txt",
	}

	_, err := client.ReadTextFile(ctx, params)
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestGenericClient_CreateTerminal(t *testing.T) {
	client := &Client{terminals: NewTerminalManager()}
	ctx := context.Background()

	params := acp.CreateTerminalRequest{
		Command: "echo",
		Args:    []string{"test"},
	}

	resp, err := client.CreateTerminal(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TerminalId == "" {
		t.Error("expected non-empty TerminalId")
	}

	// Clean up
	_, _ = client.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{TerminalId: resp.TerminalId})
}

func TestGenericClient_TerminalOutput(t *testing.T) {
	client := &Client{terminals: NewTerminalManager()}
	ctx := context.Background()

	// Create a terminal that finishes quickly
	createResp, err := client.CreateTerminal(ctx, acp.CreateTerminalRequest{
		Command: "echo",
		Args:    []string{"hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating terminal: %v", err)
	}

	// Wait for it to finish
	_, _ = client.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{TerminalId: createResp.TerminalId})

	// Now get output
	params := acp.TerminalOutputRequest{
		TerminalId: createResp.TerminalId,
	}

	resp, err := client.TerminalOutput(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Truncated {
		t.Error("expected Truncated to be false")
	}

	if resp.ExitStatus == nil {
		t.Error("expected ExitStatus to be populated")
	}

	// Clean up
	_, _ = client.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{TerminalId: createResp.TerminalId})
}

func TestGenericClient_ReleaseTerminal(t *testing.T) {
	client := &Client{terminals: NewTerminalManager()}
	ctx := context.Background()

	// Create a terminal first
	createResp, err := client.CreateTerminal(ctx, acp.CreateTerminalRequest{
		Command: "echo",
		Args:    []string{"test"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating terminal: %v", err)
	}

	params := acp.ReleaseTerminalRequest{
		TerminalId: createResp.TerminalId,
	}

	resp, err := client.ReleaseTerminal(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = resp

	// Try to use released terminal - should fail
	_, err = client.TerminalOutput(ctx, acp.TerminalOutputRequest{TerminalId: createResp.TerminalId})
	if err == nil {
		t.Error("expected error when using released terminal")
	}
}

func TestGenericClient_WaitForTerminalExit(t *testing.T) {
	client := &Client{terminals: NewTerminalManager()}
	ctx := context.Background()

	// Create a terminal that exits quickly
	createResp, err := client.CreateTerminal(ctx, acp.CreateTerminalRequest{
		Command: "echo",
		Args:    []string{"test"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating terminal: %v", err)
	}

	params := acp.WaitForTerminalExitRequest{
		TerminalId: createResp.TerminalId,
	}

	resp, err := client.WaitForTerminalExit(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ExitCode == nil {
		t.Error("expected ExitCode to be populated")
	} else if *resp.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", *resp.ExitCode)
	}

	// Clean up
	_, _ = client.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{TerminalId: createResp.TerminalId})
}

func TestGenericClient_WaitForTerminalExit_ContextCancelled(t *testing.T) {
	client := &Client{terminals: NewTerminalManager()}

	// On Windows, use a command that waits longer
	cmd := "timeout"
	args := []string{"/T", "30"}
	if runtime.GOOS != "windows" {
		cmd = "sleep"
		args = []string{"30"}
	}

	ctx, cancel := context.WithCancel(context.Background())

	createResp, err := client.CreateTerminal(ctx, acp.CreateTerminalRequest{
		Command: cmd,
		Args:    args,
	})
	if err != nil {
		t.Fatalf("unexpected error creating terminal: %v", err)
	}

	// Cancel context quickly
	cancel()

	_, err = client.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{TerminalId: createResp.TerminalId})
	if err == nil || err == context.Canceled {
		// Expected - context was cancelled
	} else {
		t.Fatalf("expected context cancellation error, got: %v", err)
	}

	// Clean up - use a fresh context for release
	cleanCtx := context.Background()
	_, _ = client.ReleaseTerminal(cleanCtx, acp.ReleaseTerminalRequest{TerminalId: createResp.TerminalId})
}

func TestGenericClient_KillTerminal(t *testing.T) {
	client := &Client{terminals: NewTerminalManager()}
	ctx := context.Background()

	// On Windows, use a command that waits
	cmd := "timeout"
	args := []string{"/T", "30"}
	if runtime.GOOS != "windows" {
		cmd = "sleep"
		args = []string{"30"}
	}

	createResp, err := client.CreateTerminal(ctx, acp.CreateTerminalRequest{
		Command: cmd,
		Args:    args,
	})
	if err != nil {
		t.Fatalf("unexpected error creating terminal: %v", err)
	}

	params := acp.KillTerminalRequest{
		TerminalId: createResp.TerminalId,
	}

	resp, err := client.KillTerminal(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = resp

	// Wait for it to actually exit
	_, _ = client.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{TerminalId: createResp.TerminalId})

	// Clean up
	_, _ = client.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{TerminalId: createResp.TerminalId})
}

// Helper functions for file operations
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
