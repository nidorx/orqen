package cmd

import (
	"context"
	"testing"
)

func TestHandleSearch_NoQuery(t *testing.T) {
	resp, err := searchCommandHandler(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}
	if resp != "**Usage:** `/search <query>`" {
		t.Errorf("expected usage message, got %q", resp)
	}
}

func TestHandleSearch_NoUserSession(t *testing.T) {
	resp, err := searchCommandHandler(context.Background(), &Request{Content: "test query"})
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}
	if resp != "**Info:** Search requires a user session. Use `/start` first." {
		t.Errorf("expected no session message, got %q", resp)
	}
}

func TestHandleSearch_NoChatStore(t *testing.T) {
	resp, err := searchCommandHandler(context.Background(), &Request{ExtId: "user1", Content: "test query"})
	if err != nil {
		t.Fatalf("handleSearch: %v", err)
	}
	if resp != "**Error:** Chat store not available." {
		t.Errorf("expected no store message, got %q", resp)
	}
}

func TestHandleSearch_Registered(t *testing.T) {
	cmd, ok := Get("search")
	if !ok {
		t.Fatal("command 'search' not registered")
	}
	if cmd.Handler == nil {
		t.Error("expected non-nil handler")
	}
}
