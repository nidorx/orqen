package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatFileEdit_CreatesPendingEdit(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	testFile := "edit-me.go"
	original := "package main\n"
	os.WriteFile(filepath.Join(dir, testFile), []byte(original), 0644)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_, out, err := ChatFileEditHandler(context.Background(), nil, &ChatFileEditInput{
		Path:      testFile,
		Content:   "package main\n\nfunc main() {}\n",
		Reason:    "add main function",
		SessionID: sess.ID,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("edit handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.EditID <= 0 {
		t.Errorf("expected positive edit_id, got %d", out.EditID)
	}
	if out.FilePath != testFile {
		t.Errorf("file_path = %q, want %q", out.FilePath, testFile)
	}

	// Verify pending edit was saved
	pe, err := store.GetPendingEdit(out.EditID)
	if err != nil {
		t.Fatalf("GetPendingEdit: %v", err)
	}
	if pe == nil {
		t.Fatal("expected pending edit to exist")
	}
	if pe.Content != "package main\n\nfunc main() {}\n" {
		t.Errorf("pending edit content = %q, want %q", pe.Content, "package main\n\nfunc main() {}\n")
	}
}

func TestChatFileEdit_GeneratesDiff(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	testFile := "diff-test.go"
	original := "package main\nfunc old() {}\n"
	os.WriteFile(filepath.Join(dir, testFile), []byte(original), 0644)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_, out, err := ChatFileEditHandler(context.Background(), nil, &ChatFileEditInput{
		Path:      testFile,
		Content:   "package main\nfunc new() {}\n",
		SessionID: sess.ID,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("edit handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Diff == "" {
		t.Fatal("expected diff preview")
	}
	if !strings.Contains(out.Diff, "-func old() {}") {
		t.Errorf("diff should contain removed line, got:\n%s", out.Diff)
	}
	if !strings.Contains(out.Diff, "+func new() {}") {
		t.Errorf("diff should contain added line, got:\n%s", out.Diff)
	}
}

func TestChatFileEdit_BlockedPaths(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	blockedPaths := []string{".orqen/config", ".git/HEAD"}
	for _, p := range blockedPaths {
		_, out, err := ChatFileEditHandler(context.Background(), nil, &ChatFileEditInput{
			Path:      p,
			Content:   "hacked",
			SessionID: sess.ID,
		}, proj, store, mgr)
		if err != nil {
			t.Fatalf("edit handler error for %s: %v", p, err)
		}
		if out.Error == "" {
			t.Errorf("expected error for blocked path %q", p)
		}
	}
}

func TestChatFileEdit_MissingPath(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_, out, err := ChatFileEditHandler(context.Background(), nil, &ChatFileEditInput{
		Path:      "",
		Content:   "some content",
		SessionID: sess.ID,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("edit handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing path")
	}
}

func TestChatFileEdit_MissingContent(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_, out, err := ChatFileEditHandler(context.Background(), nil, &ChatFileEditInput{
		Path:      "test.go",
		Content:   "",
		SessionID: sess.ID,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("edit handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing content")
	}
}

func TestChatFileEdit_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	_, out, err := ChatFileEditHandler(context.Background(), nil, &ChatFileEditInput{
		Path:      "test.go",
		Content:   "content",
		SessionID: sess.ID,
	}, nil, store, mgr)
	if err != nil {
		t.Fatalf("edit handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}
