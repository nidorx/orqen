package chat

import (
	"context"
	"testing"
)

func TestChatHistoryGet_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Add 5 messages
	for i := 1; i <= 5; i++ {
		if err := store.AddMessage(sess.ID, RoleUser, "msg"); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	// Call tool with limit=2
	_, out, err := ChatHistoryGetHandler(context.Background(), nilReq(), &ChatHistoryGetInput{
		SessionID: sess.ID,
		Limit:     2,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(out.Messages))
	}
}

func TestChatHistoryGet_DefaultLimit(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Add 30 messages
	for i := 0; i < 30; i++ {
		if err := store.AddMessage(sess.ID, RoleUser, "msg"); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	// Call tool with no limit (default: 20)
	_, out, err := ChatHistoryGetHandler(context.Background(), nilReq(), &ChatHistoryGetInput{
		SessionID: sess.ID,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Messages) != 20 {
		t.Errorf("expected 20 messages (default limit), got %d", len(out.Messages))
	}
}

func TestChatHistoryGet_MissingSessionID(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatHistoryGetHandler(context.Background(), nilReq(), &ChatHistoryGetInput{
		SessionID: "",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing session_id")
	}
}

func TestChatHistoryGet_Adapter(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Test that the adapter correctly parses input, calls handler, and marshals output
	handler := chatHandler2MCP(proj, store, mgr, ChatHistoryGetHandler)

	result, output, err := handler(context.Background(), nilReq(), &ChatHistoryGetInput{
		SessionID: sess.ID,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("adapter handler error: %v", err)
	}
	if result != nil {
		// result can be nil for content-based responses
	}
	if len(output.Messages) != 0 {
		t.Errorf("expected 0 messages (empty session), got %d", len(output.Messages))
	}
}
