package agent

import (
	"context"
	"os"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// Helper to create string pointers
func strPtr(s string) *string {
	return &s
}

func TestGenericClient_RequestPermission_AutoApprove(t *testing.T) {
	client := &GenericClient{autoApprove: true}
	ctx := context.Background()

	title := "Test permission"
	params := acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{
			{OptionId: "opt1", Name: "Allow Once", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionId: "opt2", Name: "Deny", Kind: acp.PermissionOptionKindRejectAlways},
		},
		ToolCall: acp.ToolCallUpdate{
			Title: &title,
		},
	}

	resp, err := client.RequestPermission(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Outcome.Selected == nil {
		t.Fatal("expected selected option, got nil")
	}

	// Should auto-approve with allow option
	if resp.Outcome.Selected.OptionId != "opt1" {
		t.Errorf("expected opt1, got %s", resp.Outcome.Selected.OptionId)
	}
}

func TestGenericClient_RequestPermission_AutoApprove_NoAllow(t *testing.T) {
	client := &GenericClient{autoApprove: true}
	ctx := context.Background()

	title := "Test permission"
	params := acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{
			{OptionId: "opt1", Name: "Deny", Kind: acp.PermissionOptionKindRejectAlways},
		},
		ToolCall: acp.ToolCallUpdate{
			Title: &title,
		},
	}

	resp, err := client.RequestPermission(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Outcome.Selected == nil {
		t.Fatal("expected selected option, got nil")
	}

	if resp.Outcome.Selected.OptionId != "opt1" {
		t.Errorf("expected opt1, got %s", resp.Outcome.Selected.OptionId)
	}
}

func TestGenericClient_RequestPermission_AutoApprove_NoOptions(t *testing.T) {
	client := &GenericClient{autoApprove: true}
	ctx := context.Background()

	title := "Test permission"
	params := acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{},
		ToolCall: acp.ToolCallUpdate{
			Title: &title,
		},
	}

	resp, err := client.RequestPermission(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Outcome.Cancelled == nil {
		t.Fatal("expected cancelled outcome, got nil")
	}
}

func TestGenericClient_SessionUpdate_AgentMessageChunk(t *testing.T) {
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
	client := &GenericClient{}
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
}

func TestGenericClient_TerminalOutput(t *testing.T) {
	client := &GenericClient{}
	ctx := context.Background()

	params := acp.TerminalOutputRequest{
		TerminalId: "term-1",
	}

	resp, err := client.TerminalOutput(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Truncated {
		t.Error("expected Truncated to be false")
	}
}

func TestGenericClient_ReleaseTerminal(t *testing.T) {
	client := &GenericClient{}
	ctx := context.Background()

	params := acp.ReleaseTerminalRequest{
		TerminalId: "term-1",
	}

	resp, err := client.ReleaseTerminal(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = resp
}

func TestGenericClient_WaitForTerminalExit(t *testing.T) {
	client := &GenericClient{}
	ctx := context.Background()

	params := acp.WaitForTerminalExitRequest{
		TerminalId: "term-1",
	}

	resp, err := client.WaitForTerminalExit(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = resp
}

func TestGenericClient_KillTerminal(t *testing.T) {
	client := &GenericClient{}
	ctx := context.Background()

	params := acp.KillTerminalRequest{
		TerminalId: "term-1",
	}

	resp, err := client.KillTerminal(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = resp
}

func TestGenericClient_HandleExtensionMethod_Success(t *testing.T) {
	client := &GenericClient{}
	ctx := context.Background()

	params := []byte(`{"name": "World"}`)

	resp, err := client.HandleExtensionMethod(ctx, "_example.com/hello", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resp)
	}

	if result["greeting"] != "hello World" {
		t.Errorf("expected 'hello World', got '%v'", result["greeting"])
	}
}

func TestGenericClient_HandleExtensionMethod_Unknown(t *testing.T) {
	client := &GenericClient{}
	ctx := context.Background()

	params := []byte(`{}`)

	_, err := client.HandleExtensionMethod(ctx, "_unknown/method", params)
	if err == nil {
		t.Fatal("expected error for unknown method, got nil")
	}
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
