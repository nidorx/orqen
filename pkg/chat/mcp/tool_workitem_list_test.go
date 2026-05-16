package mcp

import (
	"context"
	"testing"
)

func TestChatWorkitemList_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	// Test the handler doesn't crash without cache initialization
	_, listOut, err := ChatWorkitemListHandler(context.Background(), nilReq(), &ChatWorkitemListInput{
		Limit: 20,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("list handler error: %v", err)
	}
	// Without cache init, no items found - but handler should not panic
	_ = listOut
}

func TestChatWorkitemList_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatWorkitemListHandler(context.Background(), nilReq(), &ChatWorkitemListInput{
		Limit: 20,
	}, nil, store, mgr)
	if err != nil {
		t.Fatalf("list handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}

func TestChatWorkitemList_WithLaneFilter(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatWorkitemListHandler(context.Background(), nilReq(), &ChatWorkitemListInput{
		Lane:  "backlog",
		Limit: 10,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("list handler error: %v", err)
	}
	// Should not panic, may return 0 items without cache init
	_ = out
}
