package agent

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

// Helper: test project without real agent config

// NoOpChannel is a channel that discards all output. Used when no real channel is available.
type NoOpChannel struct{}

func (n *NoOpChannel) Send(ctx context.Context, text string)       {}
func (n *NoOpChannel) OnProgress(ctx context.Context, text string) {}

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

func newTestStore(t *testing.T) *memory.ChatStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	s, err := memory.NewChatStore(dbPath)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// Test 1: AgentSession - Prompt with nonexistent agent

func TestAgentSession_Prompt_NoAgent(t *testing.T) {
	proj := testProjectNoAgent(t)
	store := newTestStore(t)
	session, err := memory.NewSessionManager(store, memory.SessionTTL).GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, nil)
	defer as.Close()

	_, err = as.Prompt(ctx, "hello", &NoOpChannel{})
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
	if !strings.Contains(err.Error(), "agent") && !strings.Contains(err.Error(), "start") && !strings.Contains(err.Error(), "process") {
		t.Errorf("expected error about agent/process, got: %v", err)
	}
}

// Test 2: AgentSession - Confirmation intercept (approval)

func TestAgentSession_ConfirmIntercept_Approval(t *testing.T) {
	store := newTestStore(t)
	sm := memory.NewSessionManager(store, memory.SessionTTL)
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
		sendFunc: func(ctx context.Context, text string) {
			sentMsg = text
		},
	}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, mockCM)
	defer as.Close()

	ctx := context.Background()
	resp, err := as.Prompt(ctx, "yes", mockChannel)
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

// Test 3: AgentSession - Confirmation intercept (rejection)

func TestAgentSession_ConfirmIntercept_Rejection(t *testing.T) {
	store := newTestStore(t)
	sm := memory.NewSessionManager(store, memory.SessionTTL)
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
		store, session.ID, mockCM)
	defer as.Close()

	ctx := context.Background()
	resp, err := as.Prompt(ctx, "no", mockChannel)
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

// Test 4: AgentSession - Close

func TestAgentSession_Close(t *testing.T) {
	proj := testProjectNoAgent(t)
	store := newTestStore(t)
	session, err := memory.NewSessionManager(store, memory.SessionTTL).GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, nil)

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
	_, err = as.Prompt(ctx, "hello", &NoOpChannel{})
	if err == nil {
		t.Fatal("expected error after close, got nil")
	}
}

// Test 5: AgentSession - Queue ordering
// Prompts are processed in FIFO order by the queue worker.
// We verify by sending prompts sequentially and checking they complete one at a time.

func TestAgentSession_QueueOrder(t *testing.T) {
	proj := testProjectNoAgent(t)
	store := newTestStore(t)
	session, err := memory.NewSessionManager(store, memory.SessionTTL).GetOrCreateSession("test-user")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	as := NewAgentSession(proj, "nonexistent-agent", "http://localhost:0/chat/mcp/test",
		store, session.ID, nil)
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
			_, _ = as.Prompt(ctx, "msg", &NoOpChannel{})
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

// Test 6: Confirmation Keywords

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

// Test 7: System Prompt Builder

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

// Test 8: FormatHistoryForPrompt (existing tests, re-run)

// Test 9: chatClientSession with OnProgress

// Test 11: Subprocess idle timer

// Mock types

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

func (m *mockConfirmationManager) CleanupExpiredEdits() (int, error) {
	return 0, nil
}

type mockChannel struct {
	sendFunc       func(ctx context.Context, text string)
	onProgressFunc func(ctx context.Context, text string)
}

func (m *mockChannel) Send(ctx context.Context, text string) {
	if m.sendFunc != nil {
		m.sendFunc(ctx, text)
	}
}

func (m *mockChannel) OnProgress(ctx context.Context, text string) {
	if m.onProgressFunc != nil {
		m.onProgressFunc(ctx, text)
	}
}

// Test 12: formatACPError

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

func TestIsRejection(t *testing.T) {
	rejectionCases := []string{
		"no", "NO", "No", "n", "N",
		"cancel", "reject", "discard", "skip",
		"dont", "don't",
	}
	for _, text := range rejectionCases {
		if !isRejectionKeyword(text) {
			t.Errorf("isRejectionKeyword(%q) = false, expected true", text)
		}
	}

	approvalCases := []string{
		"yes", "maybe", "apply",
	}
	for _, text := range approvalCases {
		if isRejectionKeyword(text) {
			t.Errorf("isRejectionKeyword(%q) = true, expected false", text)
		}
	}
}

func TestIsApproval(t *testing.T) {
	approvalCases := []string{
		"yes", "YES", "Yes", "y", "Y",
		"ok", "OK", "approve", "Apply", "APPLY",
		"do it", "go ahead",
	}
	for _, text := range approvalCases {
		if !isApprovalKeyword(text) {
			t.Errorf("isApprovalKeyword(%q) = false, expected true", text)
		}
	}

	rejectionCases := []string{
		"no", "maybe", "cancel",
	}
	for _, text := range rejectionCases {
		if isApprovalKeyword(text) {
			t.Errorf("isApprovalKeyword(%q) = true, expected false", text)
		}
	}
}
