package cmd

import (
	"context"
	"testing"
)

func TestHandleNew_NoUserID(t *testing.T) {
	resp, err := newCommandHandler(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("handleNew: %v", err)
	}
	if resp != "**Info:** Use `/start` to initialize a session first." {
		t.Errorf("expected no session message, got %q", resp)
	}
}

func TestHandleNew_NoSessionManager(t *testing.T) {
	resp, err := newCommandHandler(context.Background(), &Request{ExtId: "user1"})
	if err != nil {
		t.Fatalf("handleNew: %v", err)
	}
	if resp != "**Error:** Session manager not available." {
		t.Errorf("expected no session manager message, got %q", resp)
	}
}

func TestHandleNew_Registered(t *testing.T) {
	cmd, ok := Get("new")
	if !ok {
		t.Fatal("command 'new' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
