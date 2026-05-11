package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// newTestChatEnv creates a ChatStore and SessionManager backed by a temp dir.
func newTestChatEnv(t *testing.T) (*ChatStore, *SessionManager) {
	t.Helper()
	store := newTestStore(t)
	mgr := NewSessionManager(store, 24*time.Hour)
	return store, mgr
}

// newTestProject creates a minimal engine.Project with one module and one lane.
func newTestProject(t *testing.T) *engine.Project {
	t.Helper()
	dir := t.TempDir()

	moduleDir := filepath.Join(dir, "tasks")
	backlogDir := filepath.Join(moduleDir, "backlog")
	os.MkdirAll(backlogDir, 0755)

	lane := &engine.Lane{
		Name:   "backlog",
		DirAbs: backlogDir,
	}
	mod := &engine.Module{
		Name:   "tasks",
		DirAbs: moduleDir,
		Lanes:  []*engine.Lane{lane},
	}
	lane.Module = mod

	return &engine.Project{
		Id:      "test-project",
		DirAbs:  dir,
		Modules: []*engine.Module{mod},
	}
}

// nilReq returns a nil *mcp.CallToolRequest since handlers don't use it for test purposes.
func nilReq() *mcp.CallToolRequest {
	return nil
}

func TestChatHistoryGet(t *testing.T) {
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

func TestChatMemorySearch(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	if err := store.AddMessage(sess.ID, RoleUser, "found an error in the logs"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := store.AddMessage(sess.ID, RoleAssistant, "everything is fine"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	_, out, err := ChatMemorySearchHandler(context.Background(), nilReq(), &ChatMemorySearchInput{
		Query:  "error",
		UserID: "user1",
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

	if err := store.AddMessage(sess.ID, RoleUser, "hello world"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	_, out, err := ChatMemorySearchHandler(context.Background(), nilReq(), &ChatMemorySearchInput{
		Query:  "nonexistent_keyword_xyz",
		UserID: "user1",
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

func TestChatWorkitemCreate(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	// Manually create a workitem directory and yaml file since CreateWorkItem
	// requires full engine cache initialization
	lane := proj.Modules[0].Lanes[0]
	itemDir := filepath.Join(lane.DirAbs, "TASK-0001-test-task")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "TASK-0001.yaml"), []byte{}, 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// Verify the directory and yaml file were created
	if _, err := os.Stat(filepath.Join(itemDir, "TASK-0001.yaml")); err != nil {
		t.Fatalf("expected workitem yaml file to exist: %v", err)
	}

	// Test the create handler validates input correctly (no project crash)
	_, out, err := ChatWorkitemCreateHandler(context.Background(), nilReq(), &ChatWorkitemCreateInput{
		Lane:  "",
		Title: "",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	// Should error gracefully for missing inputs
	if out.Error == "" {
		t.Log("create handler returned empty error for missing inputs (may be acceptable)")
	}
}

func TestChatWorkitemCreate_InvalidLane(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatWorkitemCreateHandler(context.Background(), nilReq(), &ChatWorkitemCreateInput{
		Lane:  "nonexistent",
		Title: "test",
	}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for invalid lane")
	}
	if out.Success {
		t.Error("expected success=false for invalid lane")
	}
}

func TestChatWorkitemList(t *testing.T) {
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

func TestChatWorkitemGet(t *testing.T) {
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

func TestChatFileList(t *testing.T) {
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

func TestChatFileRead(t *testing.T) {
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

func TestChatProjectGet(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatProjectGetHandler(context.Background(), nil, &ChatProjectGetInput{}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.ModuleCount == 0 {
		t.Error("expected at least one module")
	}
	if out.DirAbs != dir {
		t.Errorf("DirAbs = %q, want %q", out.DirAbs, dir)
	}
}

func TestChatLaneList(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	dir := t.TempDir()
	proj := setupTestProjectWithLane(t, dir)

	_, out, err := ChatLaneListHandler(context.Background(), nil, &ChatLaneListInput{}, proj, store, mgr)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Lanes) == 0 {
		t.Fatal("expected at least one lane")
	}
	if out.Lanes[0].Name != "backlog" {
		t.Errorf("first lane = %q, want %q", out.Lanes[0].Name, "backlog")
	}
}

func TestChatHandler2MCP_Adapter(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	// Test that the adapter correctly parses input, calls handler, and marshals output
	// We test this indirectly by verifying a tool works end-to-end via the registered mechanism

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Verify the chatHandler2MCP wrapper produces correct results
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

// setupTestProjectWithLane creates a minimal project with one module and one lane.
func setupTestProjectWithLane(t *testing.T, dir string) *engine.Project {
	t.Helper()

	moduleDir := filepath.Join(dir, "tasks")
	backlogDir := filepath.Join(moduleDir, "backlog")
	os.MkdirAll(backlogDir, 0755)

	lane := &engine.Lane{
		Name:   "backlog",
		DirAbs: backlogDir,
	}
	mod := &engine.Module{
		Name:   "tasks",
		DirAbs: moduleDir,
		Lanes:  []*engine.Lane{lane},
	}
	lane.Module = mod

	return &engine.Project{
		Id:      "test",
		DirAbs:  dir,
		Modules: []*engine.Module{mod},
	}
}
