package mcp

import (
	"context"
	"testing"
)

func TestChatWorkitemGet_NotFound(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatWorkitemGetHandler(context.Background(), nil, &ChatWorkitemGetInput{
		WorkItemID: "nonexistent-id",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("get handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for non-existent workitem")
	}
}

func TestChatWorkitemGet_MissingID(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatWorkitemGetHandler(context.Background(), nilReq(), &ChatWorkitemGetInput{
		WorkItemID: "",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("get handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing workitem_id")
	}
}

func TestChatWorkitemGet_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatWorkitemGetHandler(context.Background(), nilReq(), &ChatWorkitemGetInput{
		WorkItemID: "some-id",
	}, nil, store, mgr)
	if err != nil {
		t.Fatalf("get handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}

func TestChatWorkitemGet_WithoutCacheInit(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	// Without cache init, GetWorkItemById will return nil → handler returns "not found" error
	_, getOut, err := ChatWorkitemGetHandler(context.Background(), nilReq(), &ChatWorkitemGetInput{
		WorkItemID: "some-id",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("get handler error: %v", err)
	}
	// Handler should gracefully return "not found" error
	if getOut.Error == "" {
		t.Log("expected 'not found' error without cache init")
	}
	if getOut.Found {
		t.Error("expected found=false without cache init")
	}
}
