package chat

import (
	"context"
	"testing"
)

func TestHandleStart(t *testing.T) {
	bot := &TelegramBot{}
	resp, err := handleStart(context.Background(), "", bot, "user1")
	if err != nil {
		t.Fatalf("handleStart: %v", err)
	}
	if resp == "" {
		t.Fatal("expected non-empty response")
	}
	if !containsAny(resp, "Welcome", "Orqen", "/help") {
		t.Errorf("response missing expected content: %s", resp)
	}
}

func TestHandleStart_Registered(t *testing.T) {
	cmd, ok := GetCommand("start")
	if !ok {
		t.Fatal("command 'start' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
	if cmd.Description == "" {
		t.Error("expected non-empty description")
	}
}
