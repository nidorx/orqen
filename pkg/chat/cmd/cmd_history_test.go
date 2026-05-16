package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/nidorx/orqen/pkg/chat/memory"
)

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

func newTestSessionManager(t *testing.T) *memory.SessionManager {
	t.Helper()
	store := newTestStore(t)
	return memory.NewSessionManager(store, 24*time.Hour)
}

func TestHandleHistory_NoUserID(t *testing.T) {
	resp, err := commandHistoryHandler(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if resp != "**Info:** Use `/start` to initialize a session first." {
		t.Errorf("expected no session message, got %q", resp)
	}
}

func TestHandleHistory_NoSessionManager(t *testing.T) {
	resp, err := commandHistoryHandler(context.Background(), &Request{ExtId: "user1"})
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if resp != "**Error:** Session manager not available." {
		t.Errorf("expected no session manager message, got %q", resp)
	}
}

func TestHandleHistory_EmptySession(t *testing.T) {
	sm := newTestSessionManager(t)

	resp, err := commandHistoryHandler(context.Background(), &Request{
		ExtId:          "user1",
		SessionManager: sm,
	})
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if !containsAny(resp, "No messages") {
		t.Errorf("expected no messages message, got %q", resp)
	}
}

func TestHandleHistory_WithMessages(t *testing.T) {
	sm := newTestSessionManager(t)
	store := sm.Store

	// Create session and add messages
	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_ = store.AddMessage(sess.ID, memory.RoleUser, "Hello!")
	_ = store.AddMessage(sess.ID, memory.RoleAssistant, "Hi there!")

	resp, err := commandHistoryHandler(context.Background(), &Request{
		ExtId:          "user1",
		SessionManager: sm,
	})
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if !containsAny(resp, "**You**:", "**Orqen**:") {
		t.Errorf("expected formatted messages, got %q", resp)
	}
}

func TestHandleHistory_Registered(t *testing.T) {
	cmd, ok := Get("history")
	if !ok {
		t.Fatal("command 'history' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
