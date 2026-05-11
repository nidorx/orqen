package chat

import (
	"context"
	"testing"
)

func TestHandleHistory_NoUserID(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleHistory(context.Background(), "", bot, "")
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if resp != "Use /start to initialize a session first." {
		t.Errorf("expected no session message, got %q", resp)
	}
}

func TestHandleHistory_NoSessionManager(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleHistory(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if resp != "Session manager not available." {
		t.Errorf("expected no session manager message, got %q", resp)
	}
}

func TestHandleHistory_EmptySession(t *testing.T) {
	sm := newTestSessionManager(t)
	store := sm.store
	bot := &TelegramBot{
		ChatStore:      store,
		SessionManager: sm,
	}

	resp, err := handleHistory(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if !containsAny(resp, "No messages") {
		t.Errorf("expected no messages message, got %q", resp)
	}
}

func TestHandleHistory_WithMessages(t *testing.T) {
	sm := newTestSessionManager(t)
	store := sm.store
	bot := &TelegramBot{
		ChatStore:      store,
		SessionManager: sm,
	}

	// Create session and add messages
	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_ = store.AddMessage(sess.ID, RoleUser, "Hello!")
	_ = store.AddMessage(sess.ID, RoleAssistant, "Hi there!")

	resp, err := handleHistory(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleHistory: %v", err)
	}
	if !containsAny(resp, "You:", "Orqen:") {
		t.Errorf("expected formatted messages, got %q", resp)
	}
}

func TestHandleHistory_Registered(t *testing.T) {
	cmd, ok := GetCommand("history")
	if !ok {
		t.Fatal("command 'history' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
