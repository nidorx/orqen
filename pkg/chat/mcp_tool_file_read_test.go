package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatFileRead_ValidInput(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	testFile := "test.txt"
	os.WriteFile(filepath.Join(dir, testFile), []byte("hello world\nline 2\nline 3"), 0644)

	_, out, err := ChatFileReadHandler(context.Background(), nil, &ChatFileReadInput{
		Path: testFile,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("read handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Content, "hello world") {
		t.Errorf("expected content to contain 'hello world', got: %q", out.Content)
	}
}

func TestChatFileRead_LinePagination(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	// Create a 10-line file
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, "line "+string(rune('0'+i)))
	}
	os.WriteFile(filepath.Join(dir, "paginated.txt"), []byte(strings.Join(lines, "\n")), 0644)

	_, out, err := ChatFileReadHandler(context.Background(), nil, &ChatFileReadInput{
		Path:  "paginated.txt",
		Line:  5,
		Limit: 3,
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("read handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Lines != 3 {
		t.Errorf("expected 3 lines, got %d", out.Lines)
	}
}

func TestChatFileRead_LargeFile(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	// Create a file > 50KB
	largeContent := strings.Repeat("x", 60*1024)
	os.WriteFile(filepath.Join(dir, "large.txt"), []byte(largeContent), 0644)

	_, out, err := ChatFileReadHandler(context.Background(), nil, &ChatFileReadInput{
		Path: "large.txt",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("read handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Content, "[truncated") {
		t.Error("expected large file to be truncated")
	}
	if len(out.Content) > 55*1024 {
		t.Errorf("truncated content still too large: %d bytes", len(out.Content))
	}
}

func TestChatFileRead_MissingPath(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	_, out, err := ChatFileReadHandler(context.Background(), nil, &ChatFileReadInput{
		Path: "",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("read handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for missing path")
	}
}

func TestChatFileRead_NilProject(t *testing.T) {
	store, mgr := newTestChatEnv(t)

	_, out, err := ChatFileReadHandler(context.Background(), nil, &ChatFileReadInput{
		Path: "test.txt",
	}, nil, store, mgr)
	if err != nil {
		t.Fatalf("read handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for nil project")
	}
}
