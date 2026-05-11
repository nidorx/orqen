package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/utils"
)

// ── Helper: test project without real agent config ───────────────────────────

func testProjectNoAgent(t *testing.T) *engine.Project {
	t.Helper()
	return &engine.Project{
		Id:     "test-project",
		DirAbs: t.TempDir(),
		Agents: engine.Agent{
			Default: "nonexistent-agent",
			Clients: map[string]engine.AgentClient{
				"nonexistent-agent": {Command: []string{"nonexistent-command-abc"}},
			},
		},
		Execution: &engine.Execution{
			MaxAgents:            10,
			SleepIntervalSeconds: 60,
		},
	}
}

// ── Test 1: AgentSession — Prompt with nonexistent agent ─────────────────────

func TestAgentSession_Prompt_NoAgent(t *testing.T) {
	proj := testProjectNoAgent(t)
	store := newTestStore(t)
	session, err := NewSessionManager(store, SessionTTL).GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, &NoOpChannel{}, nil)
	defer as.Close()

	_, err = as.Prompt(ctx, "hello")
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
	if !strings.Contains(err.Error(), "agent") && !strings.Contains(err.Error(), "start") && !strings.Contains(err.Error(), "process") {
		t.Errorf("expected error about agent/process, got: %v", err)
	}
}

// ── Test 2: AgentSession — Confirmation intercept (approval) ─────────────────

func TestAgentSession_ConfirmIntercept_Approval(t *testing.T) {
	store := newTestStore(t)
	sm := NewSessionManager(store, SessionTTL)
	session, err := sm.GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	proj := testProjectNoAgent(t)

	// Create a mock confirmation manager that simulates a pending edit
	mockCM := &mockConfirmationManager{
		hasPending: true,
		applyErr:   nil,
	}

	var sentMsg string
	mockChannel := &mockChannel{
		sendFunc: func(ctx context.Context, sessionID, text string) error {
			sentMsg = text
			return nil
		},
	}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, mockChannel, mockCM)
	defer as.Close()

	ctx := context.Background()
	resp, err := as.Prompt(ctx, "yes")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !strings.Contains(resp, "Edit applied") {
		t.Errorf("expected 'Edit applied' response, got: %s", resp)
	}
	if !strings.Contains(sentMsg, "Edit applied") {
		t.Errorf("expected channel to receive 'Edit applied', got: %s", sentMsg)
	}
	if !mockCM.applyCalled {
		t.Error("expected ApplyEdit to be called")
	}
}

// ── Test 3: AgentSession — Confirmation intercept (rejection) ────────────────

func TestAgentSession_ConfirmIntercept_Rejection(t *testing.T) {
	store := newTestStore(t)
	sm := NewSessionManager(store, SessionTTL)
	session, err := sm.GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	proj := testProjectNoAgent(t)

	mockCM := &mockConfirmationManager{
		hasPending: true,
		rejectErr:  nil,
	}

	mockChannel := &mockChannel{}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, mockChannel, mockCM)
	defer as.Close()

	ctx := context.Background()
	resp, err := as.Prompt(ctx, "no")
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !strings.Contains(resp, "Edit discarded") {
		t.Errorf("expected 'Edit discarded' response, got: %s", resp)
	}
	if !mockCM.rejectCalled {
		t.Error("expected RejectEdit to be called")
	}
}

// ── Test 4: AgentSession — Close ─────────────────────────────────────────────

func TestAgentSession_Close(t *testing.T) {
	proj := testProjectNoAgent(t)
	store := newTestStore(t)
	session, err := NewSessionManager(store, SessionTTL).GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, &NoOpChannel{}, nil)

	// Close should succeed
	if err := as.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Second close should be idempotent
	if err := as.Close(); err != nil {
		t.Fatalf("Close (second): %v", err)
	}

	// Prompt after close should fail
	ctx := context.Background()
	_, err = as.Prompt(ctx, "hello")
	if err == nil {
		t.Fatal("expected error after close, got nil")
	}
}

// ── Test 5: AgentSession — Queue ordering ───────────────────────────────────
// Prompts are processed in FIFO order by the queue worker.
// We verify by sending prompts sequentially and checking they complete one at a time.

func TestAgentSession_QueueOrder(t *testing.T) {
	proj := testProjectNoAgent(t)
	store := newTestStore(t)
	session, err := NewSessionManager(store, SessionTTL).GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, &NoOpChannel{}, nil)
	defer as.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send prompts sequentially (since agent doesn't exist, they fail fast)
	// and verify they're processed one at a time (not concurrently).
	var completedOrder []int
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		idx := i
		go func() {
			_, _ = as.Prompt(ctx, "msg")
			mu.Lock()
			completedOrder = append(completedOrder, idx)
			mu.Unlock()
		}()
		// Small delay to ensure goroutines start in order
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for all to complete
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := make([]int, len(completedOrder))
	copy(got, completedOrder)
	mu.Unlock()

	// Results should be in FIFO order since prompts are processed sequentially
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Errorf("expected FIFO order [0,1,2], got %v", got)
	}
}

// ── Test 6: Confirmation Keywords ───────────────────────────────────────────

func TestIsApprovalKeyword(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"yes", true},
		{"YES", false}, // case-sensitive (already lowered)
		{"y", true},
		{"ok", true},
		{"approve", true},
		{"apply", true},
		{"do it", true},
		{"go ahead", true},
		{"no", false},
		{"cancel", false},
		{"maybe", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isApprovalKeyword(tc.input)
		if got != tc.expected {
			t.Errorf("isApprovalKeyword(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestIsRejectionKeyword(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"no", true},
		{"n", true},
		{"cancel", true},
		{"reject", true},
		{"discard", true},
		{"skip", true},
		{"dont", true},
		{"don't", true},
		{"yes", false},
		{"ok", false},
		{"apply", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isRejectionKeyword(tc.input)
		if got != tc.expected {
			t.Errorf("isRejectionKeyword(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

// ── Test 7: System Prompt Builder ───────────────────────────────────────────

func TestBuildSystemPrompt(t *testing.T) {
	proj := &engine.Project{
		Id:     "test-123",
		DirAbs: "/tmp/test",
		Agents: engine.Agent{
			Default: "qwen",
			Clients: map[string]engine.AgentClient{
				"qwen": {Command: []string{"qwen", "--acp"}},
			},
		},
		Execution: &engine.Execution{MaxAgents: 10},
	}

	prompt := buildSystemPrompt(proj)

	// Verify key sections exist
	if !strings.Contains(prompt, "<system>") {
		t.Error("system prompt should start with <system>")
	}
	if !strings.Contains(prompt, "</system>") {
		t.Error("system prompt should end with </system>")
	}
	if !strings.Contains(prompt, "Available Tools") {
		t.Error("system prompt should list available tools")
	}
	if !strings.Contains(prompt, "chat_file_edit") {
		t.Error("system prompt should mention chat_file_edit")
	}
	if !strings.Contains(prompt, "Rules") {
		t.Error("system prompt should have rules section")
	}
	if !strings.Contains(prompt, "Project Context") {
		t.Error("system prompt should have project context")
	}
	if !strings.Contains(prompt, "test-123") {
		t.Error("system prompt should include project ID")
	}
}

func TestBuildSystemPrompt_NilProject(t *testing.T) {
	prompt := buildSystemPrompt(nil)
	if !strings.Contains(prompt, "No project loaded") {
		t.Error("nil project should produce 'No project loaded' message")
	}
}

func TestBuildProjectSummary(t *testing.T) {
	proj := &engine.Project{
		Id:     "proj-001",
		DirAbs: "/tmp/proj",
	}
	summary := buildProjectSummary(proj)
	if !strings.Contains(summary, "proj-001") {
		t.Errorf("expected project ID in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "No modules configured") {
		t.Errorf("expected 'No modules' for empty project, got: %s", summary)
	}
}

// ── Test 8: FormatHistoryForPrompt (existing tests, re-run) ─────────────────

func TestFormatHistoryForPrompt_Integration(t *testing.T) {
	store := newTestStore(t)
	sm := NewSessionManager(store, SessionTTL)

	sess, err := sm.GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	messages := []struct {
		role    MessageRole
		content string
	}{
		{RoleSystem, "You are a helpful assistant"},
		{RoleUser, "Hello, how are you?"},
		{RoleAssistant, "I'm doing well, thank you!"},
		{RoleSystem, "Remember to be concise"},
		{RoleUser, "What is 2+2?"},
	}

	for _, m := range messages {
		if err := store.AddMessage(sess.ID, m.role, m.content); err != nil {
			t.Fatalf("AddMessage %s: %v", m.role, err)
		}
	}

	history, err := sm.GetFullHistoryForContext(sess.ID)
	if err != nil {
		t.Fatalf("GetFullHistoryForContext: %v", err)
	}

	if len(history) != 5 {
		t.Errorf("expected 5 messages in history, got %d", len(history))
	}

	formatted := FormatHistoryForPrompt(history)

	if strings.Contains(formatted, "You are a helpful assistant") {
		t.Error("system message should not appear in formatted prompt")
	}
	if !strings.Contains(formatted, "Hello, how are you?") {
		t.Error("user message missing from formatted prompt")
	}
	if !strings.Contains(formatted, "I'm doing well, thank you!") {
		t.Error("assistant message missing from formatted prompt")
	}
}

// ── Test 9: chatClientSession with OnProgress ───────────────────────────────

func TestChatClientSession_AgentMessageChunk(t *testing.T) {
	var progress []string
	channel := &mockChannel{
		onProgressFunc: func(ctx context.Context, sessionID, text string) error {
			progress = append(progress, text)
			return nil
		},
	}

	session := newChatClientSession(channel, "test-session", context.Background())

	text1 := "Hello"
	text2 := ", world!"
	content1 := acp.ContentBlock{Text: &acp.ContentBlockText{Text: text1}}
	content2 := acp.ContentBlock{Text: &acp.ContentBlockText{Text: text2}}

	err := session.SessionUpdate(context.Background(), acp.SessionNotification{
		Update: acp.SessionUpdate{
			AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{Content: content1},
		},
	})
	if err != nil {
		t.Fatalf("SessionUpdate 1: %v", err)
	}

	err = session.SessionUpdate(context.Background(), acp.SessionNotification{
		Update: acp.SessionUpdate{
			AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{Content: content2},
		},
	})
	if err != nil {
		t.Fatalf("SessionUpdate 2: %v", err)
	}

	response := session.getResponse()
	expected := "Hello, world!"
	if response != expected {
		t.Errorf("expected response %q, got %q", expected, response)
	}

	// Agent message chunks should NOT trigger OnProgress
	if len(progress) > 0 {
		t.Errorf("expected no OnProgress for agent message chunks, got: %v", progress)
	}
}

func TestChatClientSession_ToolCall_OnProgress(t *testing.T) {
	var progress []string
	channel := &mockChannel{
		onProgressFunc: func(ctx context.Context, sessionID, text string) error {
			progress = append(progress, text)
			return nil
		},
	}

	session := newChatClientSession(channel, "test-session", context.Background())

	toolCallID := acp.ToolCallId("tc-001")
	title := "read_file"
	status := acp.ToolCallStatusInProgress

	err := session.SessionUpdate(context.Background(), acp.SessionNotification{
		Update: acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				ToolCallId: toolCallID,
				Title:      title,
				Status:     status,
			},
		},
	})
	if err != nil {
		t.Fatalf("SessionUpdate ToolCall: %v", err)
	}

	// Should have sent OnProgress
	if len(progress) != 1 {
		t.Fatalf("expected 1 OnProgress call, got %d", len(progress))
	}
	if !strings.Contains(progress[0], "read_file") {
		t.Errorf("expected progress to contain tool name, got: %s", progress[0])
	}

	// Now send completion
	err = session.SessionUpdate(context.Background(), acp.SessionNotification{
		Update: acp.SessionUpdate{
			ToolCallUpdate: &acp.SessionToolCallUpdate{
				ToolCallId: toolCallID,
				Title:      &title,
				Status:     func() *acp.ToolCallStatus { s := acp.ToolCallStatusCompleted; return &s }(),
			},
		},
	})
	if err != nil {
		t.Fatalf("SessionUpdate ToolCallUpdate: %v", err)
	}

	// Should have sent completion OnProgress
	if len(progress) != 2 {
		t.Fatalf("expected 2 OnProgress calls, got %d", len(progress))
	}
	if !strings.Contains(progress[1], "✅") {
		t.Errorf("expected completion progress to contain checkmark, got: %s", progress[1])
	}
}

func TestChatClientSession_ThoughtChunk(t *testing.T) {
	var progress []string
	channel := &mockChannel{
		onProgressFunc: func(ctx context.Context, sessionID, text string) error {
			progress = append(progress, text)
			return nil
		},
	}

	session := newChatClientSession(channel, "test-session", context.Background())

	text := "Let me think about this..."
	content := acp.ContentBlock{Text: &acp.ContentBlockText{Text: text}}

	err := session.SessionUpdate(context.Background(), acp.SessionNotification{
		Update: acp.SessionUpdate{
			AgentThoughtChunk: &acp.SessionUpdateAgentThoughtChunk{Content: content},
		},
	})
	if err != nil {
		t.Fatalf("SessionUpdate: %v", err)
	}

	// Should have sent OnProgress "Thinking..."
	if len(progress) != 1 {
		t.Fatalf("expected 1 OnProgress call, got %d", len(progress))
	}
	if !strings.Contains(progress[0], "Thinking") {
		t.Errorf("expected 'Thinking' progress, got: %s", progress[0])
	}
}

// ── Test 10: chatTerminalManager ────────────────────────────────────────────

func TestChatTerminalManager_CreateAndOutput(t *testing.T) {
	tm := newChatTerminalManager()

	ctx := context.Background()
	cmd := "echo"
	args := []string{"hello world"}

	id, err := tm.createSession(ctx, cmd, args, nil, "", nil)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	output, _, exitStatus, err := tm.terminalOutput(id)
	if err != nil {
		t.Fatalf("terminalOutput: %v", err)
	}

	if trimmed := strings.TrimSpace(output); !strings.Contains(trimmed, "hello world") {
		t.Errorf("expected output to contain 'hello world', got: %q", trimmed)
	}
	if exitStatus == nil || exitStatus.ExitCode == nil || *exitStatus.ExitCode != 0 {
		t.Errorf("expected exit code 0, got: %v", exitStatus)
	}
}

// ── Test 11: Subprocess idle timer ──────────────────────────────────────────

func TestScheduleIdle_TimerStopsProcess(t *testing.T) {
	// Clean state
	agentsMu.Lock()
	agents = map[string]*agentProcess{}
	idleTimers = map[string]*time.Timer{}
	agentsMu.Unlock()

	// Use the same key computation as scheduleIdle
	agentID := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", "test-agent", "test-proj")))

	agents[agentID] = &agentProcess{}

	scheduleIdle("test-proj", "test-agent", 100*time.Millisecond)

	agentsMu.Lock()
	_, hasTimer := idleTimers[agentID]
	agentsMu.Unlock()

	if !hasTimer {
		t.Fatal("expected idle timer to be set")
	}

	// Wait for timer to fire
	time.Sleep(200 * time.Millisecond)

	agentsMu.Lock()
	_, timerStillExists := idleTimers[agentID]
	_, agentStillExists := agents[agentID]
	agentsMu.Unlock()

	if timerStillExists {
		t.Error("expected idle timer to be removed after firing")
	}
	if agentStillExists {
		t.Error("expected agent to be removed after idle timeout")
	}
}

func TestCancelIdle(t *testing.T) {
	agentID := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", "test-agent", "test-proj")))

	// Set up
	agentsMu.Lock()
	agents = map[string]*agentProcess{}
	idleTimers = map[string]*time.Timer{}
	agents[agentID] = &agentProcess{}
	agentsMu.Unlock()

	scheduleIdle("test-proj", "test-agent", 5*time.Second)

	// Cancel idle
	cancelIdle("test-proj", "test-agent")

	agentsMu.Lock()
	_, hasTimer := idleTimers[agentID]
	agentsMu.Unlock()

	if hasTimer {
		t.Error("expected idle timer to be cancelled")
	}
}

// ── Mock types ───────────────────────────────────────────────────────────────

type mockConfirmationManager struct {
	hasPending   bool
	applyCalled  bool
	applyErr     error
	rejectCalled bool
	rejectErr    error
}

func (m *mockConfirmationManager) HasPendingEdit(sessionID string) bool {
	return m.hasPending
}
func (m *mockConfirmationManager) ApplyEdit(sessionID string) error {
	m.applyCalled = true
	return m.applyErr
}
func (m *mockConfirmationManager) RejectEdit(sessionID string) error {
	m.rejectCalled = true
	return m.rejectErr
}

type mockChannel struct {
	sendFunc       func(ctx context.Context, sessionID, text string) error
	onProgressFunc func(ctx context.Context, sessionID, text string) error
}

func (m *mockChannel) Send(ctx context.Context, sessionID, text string) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, sessionID, text)
	}
	return nil
}

func (m *mockChannel) OnProgress(ctx context.Context, sessionID, text string) error {
	if m.onProgressFunc != nil {
		return m.onProgressFunc(ctx, sessionID, text)
	}
	return nil
}

// ── Test 12: formatACPError ─────────────────────────────────────────────────

func TestFormatACPError_RequestError(t *testing.T) {
	re := &acp.RequestError{
		Code:    400,
		Message: "bad request",
	}

	err := formatACPError("test-agent", "prompt", re)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "test-agent") {
		t.Errorf("expected error to contain agent name, got: %s", errStr)
	}
	if !strings.Contains(errStr, "400") {
		t.Errorf("expected error to contain error code, got: %s", errStr)
	}
	if !strings.Contains(errStr, "bad request") {
		t.Errorf("expected error to contain error message, got: %s", errStr)
	}
}

func TestFormatACPError_GenericError(t *testing.T) {
	inner := context.DeadlineExceeded
	err := formatACPError("test-agent", "new session", inner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "test-agent") {
		t.Errorf("expected error to contain agent name, got: %s", errStr)
	}
	if !strings.Contains(errStr, "context deadline exceeded") {
		t.Errorf("expected error to contain inner error, got: %s", errStr)
	}
}
