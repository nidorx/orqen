package mcp

import (
	"context"
	"testing"

	"github.com/nidorx/orqen/pkg/chat/memory"
)

func TestChatMemorySearch_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	if err := store.AddMessage(sess.ID, memory.RoleUser, "found an error in the logs"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := store.AddMessage(sess.ID, memory.RoleAssistant, "everything is fine"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	_, out, err := ChatMemorySearchHandler(context.Background(), nilReq(), &ChatMemorySearchInput{
		Query: "error",
		ExtId: "user1",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Results) == 0 {
		t.Fatal("expected search results containing 'error'")
	}
}

func TestChatMemorySearch_NoResults(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	if err := store.AddMessage(sess.ID, memory.RoleUser, "hello world"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	_, out, err := ChatMemorySearchHandler(context.Background(), nilReq(), &ChatMemorySearchInput{
		Query: "nonexistent_keyword_xyz",
		ExtId: "user1",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(out.Results))
	}
}

func TestChatMemorySearch_MissingQuery(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatMemorySearchHandler(context.Background(), nilReq(), &ChatMemorySearchInput{
		Query: "",
		ExtId: "user1",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing query")
	}
}

func TestChatMemorySearch_MissingUserID(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatMemorySearchHandler(context.Background(), nilReq(), &ChatMemorySearchInput{
		Query: "test",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing user_id")
	}
}
