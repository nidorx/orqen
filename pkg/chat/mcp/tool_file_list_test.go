package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatFileList_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	// Create some files
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	_, out, err := ChatFileListHandler(context.Background(), nil, &ChatFileListInput{
		Recursive: true,
		Limit:     50,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("list handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Entries) == 0 {
		t.Fatal("expected file entries")
	}
}

func TestChatFileList_PathFilter(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello"), 0644)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644)

	_, out, err := ChatFileListHandler(context.Background(), nil, &ChatFileListInput{
		Path:      "src",
		Recursive: true,
		Limit:     50,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("list handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	// All entries should be under src/
	for _, e := range out.Entries {
		if !strings.HasPrefix(e.Path, "src") {
			t.Errorf("entry %q not under src/", e.Path)
		}
	}
}

func TestChatFileList_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatFileListHandler(context.Background(), nil, &ChatFileListInput{
		Recursive: true,
		Limit:     50,
	}, nil, store, mgr)
	if err != nil {
		t.Fatalf("list handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}
