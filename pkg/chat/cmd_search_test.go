package chat

import (
	"context"
	"testing"
)

func TestHandleSearch_NoQuery(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleSearch(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}
	if resp != "Usage: /search <query>" {
		t.Errorf("expected usage message, got %q", resp)
	}
}

func TestHandleSearch_NoUserSession(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleSearch(context.Background(), "test query", bot, "")
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}
	if resp != "Search requires a user session. Use /start first." {
		t.Errorf("expected no session message, got %q", resp)
	}
}

func TestHandleSearch_NoChatStore(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleSearch(context.Background(), "test query", bot, "user1")
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}
	if resp != "Chat store not available." {
		t.Errorf("expected no store message, got %q", resp)
	}
}

func TestHandleSearch_Registered(t *testing.T) {
	cmd, ok := GetCommand("search")
	if !ok {
		t.Fatal("command 'search' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
