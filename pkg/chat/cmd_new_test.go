package chat

import (
	"context"
	"testing"
)

func TestHandleNew_NoUserID(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleNew(context.Background(), "", bot, "")
	if err != nil {
		t.Fatalf("handleNew: %v", err)
	}
	if resp != "Use /start to initialize a session first." {
		t.Errorf("expected no session message, got %q", resp)
	}
}

func TestHandleNew_NoSessionManager(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleNew(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleNew: %v", err)
	}
	if resp != "Session manager not available." {
		t.Errorf("expected no session manager message, got %q", resp)
	}
}

func TestHandleNew_Registered(t *testing.T) {
	cmd, ok := GetCommand("new")
	if !ok {
		t.Fatal("command 'new' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
