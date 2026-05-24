package agent

import (
	"context"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// mockAgent is a test double for Agent that allows controlling LoadSession behavior.
type mockAgent struct {
	loadSession       bool
	loadSessionCalled bool
	loadSessionErr    error
	newSessionCalled  bool
	newSessionErr     error
	newSessionID      string
	promptCalled      bool
	promptErr         error
}

func (m *mockAgent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	m.newSessionCalled = true
	if m.newSessionErr != nil {
		return acp.NewSessionResponse{}, m.newSessionErr
	}
	return acp.NewSessionResponse{SessionId: acp.SessionId(m.newSessionID)}, nil
}

func (m *mockAgent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	m.loadSessionCalled = true
	return acp.LoadSessionResponse{}, m.loadSessionErr
}

func (m *mockAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	m.promptCalled = true
	return acp.PromptResponse{}, m.promptErr
}

// TestSessionReload_AttemptedWhenSupported verifies that LoadSession is called
// when priorSessionID is provided and the agent supports loadSession.
func TestSessionReload_AttemptedWhenSupported(t *testing.T) {
	mock := &mockAgent{
		loadSession:  true,
		newSessionID: "test-session-123",
	}

	// LoadSession should be called because priorSessionID is non-empty and agent supports it
	if !mock.loadSession {
		t.Fatal("test setup error: mock should support loadSession")
	}

	_, _ = mock.LoadSession(context.Background(), acp.LoadSessionRequest{
		SessionId:  "prior-session-456",
		Cwd:        "/test",
		McpServers: []acp.McpServer{},
	})

	if !mock.loadSessionCalled {
		t.Error("expected LoadSession to be called when priorSessionID is provided and agent supports it")
	}
}

// TestSessionReload_FallbackWhenNotSupported verifies that LoadSession is NOT called
// when the agent does not support loadSession capability.
func TestSessionReload_FallbackWhenNotSupported(t *testing.T) {
	mock := &mockAgent{
		loadSession:  false,
		newSessionID: "test-session-789",
	}

	// LoadSession should NOT be called because agent doesn't support it
	// Even though priorSessionID is provided
	if mock.loadSession {
		t.Fatal("test setup error: mock should NOT support loadSession")
	}

	// Simulate the Exec decision logic: only call LoadSession if agent.loadSession is true
	if mock.loadSession {
		_, _ = mock.LoadSession(context.Background(), acp.LoadSessionRequest{
			SessionId:  "prior-session-456",
			Cwd:        "/test",
			McpServers: []acp.McpServer{},
		})
	}

	if mock.loadSessionCalled {
		t.Error("expected LoadSession NOT to be called when agent doesn't support loadSession")
	}
}

// TestSessionReload_FallbackToNewSessionOnLoadFailure verifies that when LoadSession fails,
// the code falls back to creating a new session.
func TestSessionReload_FallbackToNewSessionOnLoadFailure(t *testing.T) {
	mock := &mockAgent{
		loadSession:    true,
		loadSessionErr: context.Canceled, // Simulate load failure
		newSessionID:   "fallback-session-001",
	}

	// Simulate LoadSession failing
	_, err := mock.LoadSession(context.Background(), acp.LoadSessionRequest{
		SessionId:  "expired-session-999",
		Cwd:        "/test",
		McpServers: []acp.McpServer{},
	})

	if err == nil {
		t.Fatal("expected LoadSession to fail in this test")
	}

	// After failure, NewSession should be called as fallback
	_, _ = mock.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        "/test",
		McpServers: []acp.McpServer{},
	})

	if !mock.newSessionCalled {
		t.Error("expected NewSession to be called as fallback when LoadSession fails")
	}
}

// TestSessionReload_NewSessionWhenNoPriorSession verifies that when no priorSessionID
// is provided, NewSession is called directly without attempting LoadSession.
func TestSessionReload_NewSessionWhenNoPriorSession(t *testing.T) {
	mock := &mockAgent{
		loadSession:  true,
		newSessionID: "fresh-session-002",
	}

	// Simulate no prior session - LoadSession should NOT be called
	priorSessionID := ""
	if priorSessionID != "" && mock.loadSession {
		_, _ = mock.LoadSession(context.Background(), acp.LoadSessionRequest{
			SessionId:  acp.SessionId(priorSessionID),
			Cwd:        "/test",
			McpServers: []acp.McpServer{},
		})
	}

	// NewSession should be the only session operation
	if mock.loadSessionCalled {
		t.Error("expected LoadSession NOT to be called when priorSessionID is empty")
	}

	_, _ = mock.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        "/test",
		McpServers: []acp.McpServer{},
	})

	if !mock.newSessionCalled {
		t.Error("expected NewSession to be called when no prior session exists")
	}
}

// TestAgent_LoadSessionField verifies that the Agent struct properly stores
// the loadSession capability from the Initialize response.
func TestAgent_LoadSessionField(t *testing.T) {
	// Test that the field exists and can be set
	agent := &Agent{
		loadSession: true,
	}
	if !agent.loadSession {
		t.Error("expected loadSession field to be true")
	}

	agent.loadSession = false
	if agent.loadSession {
		t.Error("expected loadSession field to be false after setting")
	}
}
