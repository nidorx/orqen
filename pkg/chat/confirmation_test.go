package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nidorx/orqen/pkg/engine"
)

// ── Helper: create test ChatStore ────────────────────────────────────────────

func newTestChatStore(t *testing.T) *ChatStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	store, err := NewChatStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create chat store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// ── Helper: create test ConfirmationManager ──────────────────────────────────

func newTestConfirmationManager(t *testing.T) (*confirmationManager, *ChatStore, *engine.Project) {
	t.Helper()
	store := newTestChatStore(t)
	proj := newTestProject(t)
	cm := NewConfirmationManager(store, proj).(*confirmationManager)
	return cm, store, proj
}

// helperSession creates a chat session in the store and returns its ID.
func helperSession(t *testing.T, store *ChatStore) string {
	t.Helper()
	sess, err := store.CreateSession("test-user", 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	return sess.ID
}

// ── 1. CreateEdit — basic ───────────────────────────────────────────────────

func TestCreateEdit_Basic(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Create a file
	filePath := "test.txt"
	fileAbs := filepath.Join(proj.DirAbs, filePath)
	if err := os.WriteFile(fileAbs, []byte("original content\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Propose an edit
	editID, err := cm.CreateEdit(sessionID, filePath, "new content\n", "test edit")
	if err != nil {
		t.Fatalf("CreateEdit failed: %v", err)
	}
	if editID <= 0 {
		t.Errorf("expected positive edit ID, got %d", editID)
	}

	// Verify pending edit exists
	if !cm.HasPendingEdit(sessionID) {
		t.Error("expected HasPendingEdit to return true")
	}

	edit := cm.GetPendingEdit(sessionID)
	if edit == nil {
		t.Fatal("expected GetPendingEdit to return an edit")
	}
	if edit.FilePath != filePath {
		t.Errorf("expected file path %q, got %q", filePath, edit.FilePath)
	}
	if edit.Content != "new content\n" {
		t.Errorf("expected content %q, got %q", "new content\n", edit.Content)
	}
}

// ── 2. CreateEdit — new file ────────────────────────────────────────────────

func TestCreateEdit_NewFile(t *testing.T) {
	cm, store, _ := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Propose an edit for a file that doesn't exist
	editID, err := cm.CreateEdit(sessionID, "newfile.txt", "brand new\n", "new file")
	if err != nil {
		t.Fatalf("CreateEdit failed: %v", err)
	}
	if editID <= 0 {
		t.Errorf("expected positive edit ID, got %d", editID)
	}

	// Verify diff shows all lines as additions
	edit := cm.GetPendingEdit(sessionID)
	diff := GenerateDiff("", edit.Content, edit.FilePath)
	if !strings.Contains(diff, "+brand new") {
		t.Errorf("expected diff to show additions, got:\n%s", diff)
	}
}

// ── 3. CreateEdit — identical content ───────────────────────────────────────

func TestCreateEdit_IdenticalContent(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Create a file
	filePath := "same.txt"
	if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath), []byte("unchanged\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Propose identical content
	_, err := cm.CreateEdit(sessionID, filePath, "unchanged\n", "no change")
	if err == nil {
		t.Fatal("expected error for identical content, got nil")
	}
	if !strings.Contains(err.Error(), "identical") {
		t.Errorf("expected 'identical' error, got: %v", err)
	}
}

// ── 4. CreateEdit — blocked path ────────────────────────────────────────────

func TestCreateEdit_BlockedPath(t *testing.T) {
	cm, store, _ := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	_, err := cm.CreateEdit(sessionID, ".orqen/config", "secret\n", "blocked")
	if err == nil {
		t.Fatal("expected error for blocked path, got nil")
	}
	if !strings.Contains(err.Error(), "protected") {
		t.Errorf("expected 'protected' error, got: %v", err)
	}
}

// ── 5. GenerateDiff — small ─────────────────────────────────────────────────

func TestGenerateDiff_Small(t *testing.T) {
	current := "line1\nline2\nline3\nline4\nline5\n"
	proposed := "line1\nline2\nmodified\nline4\nline5\n"

	diff := GenerateDiff(current, proposed, "test.txt")

	if diff == "" {
		t.Fatal("expected non-empty diff")
	}

	// Check unified diff format
	if !strings.Contains(diff, "--- a/test.txt") {
		t.Errorf("expected '--- a/test.txt', got:\n%s", diff)
	}
	if !strings.Contains(diff, "+++ b/test.txt") {
		t.Errorf("expected '+++ b/test.txt', got:\n%s", diff)
	}
	if !strings.Contains(diff, "-line3") {
		t.Errorf("expected '-line3' in diff, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+modified") {
		t.Errorf("expected '+modified' in diff, got:\n%s", diff)
	}
}

// ── 6. GenerateDiff — large ─────────────────────────────────────────────────

func TestGenerateDiff_Large(t *testing.T) {
	// Create 200-line files with differences throughout
	var oldLines, newLines []string
	for i := 0; i < 200; i++ {
		oldLines = append(oldLines, "line"+string(rune('0'+i%10)))
		newLines = append(newLines, "line"+string(rune('0'+i%10)))
	}
	// Modify lines 50-60
	for i := 50; i < 60; i++ {
		newLines[i] = "MODIFIED-" + newLines[i]
	}

	current := strings.Join(oldLines, "\n")
	proposed := strings.Join(newLines, "\n")

	diff := GenerateDiff(current, proposed, "large.txt")

	lines := strings.Split(diff, "\n")
	if len(lines) > 55 { // 50 lines + a few header/trailer lines
		t.Errorf("expected diff to be truncated to ~50 lines, got %d lines", len(lines))
	}
	if strings.Contains(diff, "truncated") {
		// Good — truncation note is present
	}
}

// ── 7. FormatConfirmationMessage ─────────────────────────────────────────────

func TestFormatConfirmationMessage(t *testing.T) {
	edit := &PendingEdit{
		FilePath: "src/main.go",
		Reason:   "fix bug",
	}
	diff := "--- a/src/main.go\n+++ b/src/main.go\n@@ -1,3 +1,3 @@\n-old\n+new\n"

	msg := FormatConfirmationMessage(edit, diff)

	if !strings.Contains(msg, "src/main.go") {
		t.Errorf("expected file path in message, got:\n%s", msg)
	}
	if !strings.Contains(msg, "fix bug") {
		t.Errorf("expected reason in message, got:\n%s", msg)
	}
	if !strings.Contains(msg, diff) {
		t.Errorf("expected diff in message, got:\n%s", msg)
	}
	if !strings.Contains(msg, "yes") || !strings.Contains(msg, "ok") {
		t.Errorf("expected approval instructions, got:\n%s", msg)
	}
	if !strings.Contains(msg, "no") || !strings.Contains(msg, "cancel") {
		t.Errorf("expected rejection instructions, got:\n%s", msg)
	}
}

// ── 8. ApplyEdit — success ──────────────────────────────────────────────────

func TestApplyEdit_Success(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Create a file
	filePath := "apply.txt"
	fileAbs := filepath.Join(proj.DirAbs, filePath)
	if err := os.WriteFile(fileAbs, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create pending edit
	_, err := cm.CreateEdit(sessionID, filePath, "after\n", "apply test")
	if err != nil {
		t.Fatalf("CreateEdit failed: %v", err)
	}

	// Apply the edit
	if err := cm.ApplyEdit(sessionID); err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	// Verify file content
	data, err := os.ReadFile(fileAbs)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "after\n" {
		t.Errorf("expected content 'after\\n', got %q", string(data))
	}

	// Verify pending edit is removed
	if cm.HasPendingEdit(sessionID) {
		t.Error("expected HasPendingEdit to return false after apply")
	}
}

// ── 9. ApplyEdit — not found ────────────────────────────────────────────────

func TestApplyEdit_NotFound(t *testing.T) {
	cm, _, _ := newTestConfirmationManager(t)

	err := cm.ApplyEdit("nonexistent-session")
	if err == nil {
		t.Fatal("expected error for non-existent session, got nil")
	}
	if !strings.Contains(err.Error(), "no pending") {
		t.Errorf("expected 'no pending' error, got: %v", err)
	}
}

// ── 10. ApplyEdit — expired ─────────────────────────────────────────────────

func TestApplyEdit_Expired(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Create a file
	filePath := "expire.txt"
	if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Save pending edit directly with past timestamp
	editID, err := store.SavePendingEdit(sessionID, filePath, "new\n", "expired test")
	if err != nil {
		t.Fatalf("SavePendingEdit failed: %v", err)
	}

	// Set in-memory edit with expired timestamp
	cm.pending[sessionID] = &PendingEdit{
		ID:        editID,
		SessionID: sessionID,
		FilePath:  filePath,
		Content:   "new\n",
		Reason:    "expired test",
		CreatedAt: time.Now().Add(-20 * time.Minute), // beyond PendingEditTTL (10 min)
	}

	// Try to apply
	err = cm.ApplyEdit(sessionID)
	if err == nil {
		t.Fatal("expected expired error, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired' error, got: %v", err)
	}
}

// ── 11. RejectEdit — success ────────────────────────────────────────────────

func TestRejectEdit_Success(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Create a file and pending edit
	filePath := "reject.txt"
	if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath), []byte("original\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	_, err := cm.CreateEdit(sessionID, filePath, "changed\n", "reject test")
	if err != nil {
		t.Fatalf("CreateEdit failed: %v", err)
	}

	// Reject the edit
	if err := cm.RejectEdit(sessionID); err != nil {
		t.Fatalf("RejectEdit failed: %v", err)
	}

	// Verify pending edit is removed
	if cm.HasPendingEdit(sessionID) {
		t.Error("expected HasPendingEdit to return false after rejection")
	}

	// Verify file content is unchanged
	data, err := os.ReadFile(filepath.Join(proj.DirAbs, filePath))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "original\n" {
		t.Errorf("expected unchanged content 'original\\n', got %q", string(data))
	}
}

// ── 12. RejectEdit — not found ──────────────────────────────────────────────

func TestRejectEdit_NotFound(t *testing.T) {
	cm, _, _ := newTestConfirmationManager(t)

	err := cm.RejectEdit("nonexistent-session")
	if err == nil {
		t.Fatal("expected error for non-existent session, got nil")
	}
	if !strings.Contains(err.Error(), "no pending") {
		t.Errorf("expected 'no pending' error, got: %v", err)
	}
}

// ── 13. HasPendingEdit ──────────────────────────────────────────────────────

func TestHasPendingEdit(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID := helperSession(t, store)

	// Initially no pending edit
	if cm.HasPendingEdit(sessionID) {
		t.Error("expected HasPendingEdit to return false initially")
	}

	// Create a file
	filePath := "haspending.txt"
	if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create pending edit
	_, err := cm.CreateEdit(sessionID, filePath, "after\n", "test")
	if err != nil {
		t.Fatalf("CreateEdit failed: %v", err)
	}

	// Now should have pending edit
	if !cm.HasPendingEdit(sessionID) {
		t.Error("expected HasPendingEdit to return true")
	}

	// Apply the edit
	if err := cm.ApplyEdit(sessionID); err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	// After applying, no pending edit
	if cm.HasPendingEdit(sessionID) {
		t.Error("expected HasPendingEdit to return false after apply")
	}
}

// ── 14. IsApproval ──────────────────────────────────────────────────────────

func TestIsApproval(t *testing.T) {
	approvalCases := []string{
		"yes", "YES", "Yes", "y", "Y",
		"ok", "OK", "approve", "Apply", "APPLY",
		"do it", "go ahead",
	}
	for _, text := range approvalCases {
		if !IsApproval(text) {
			t.Errorf("IsApproval(%q) = false, expected true", text)
		}
	}

	rejectionCases := []string{
		"no", "maybe", "cancel",
	}
	for _, text := range rejectionCases {
		if IsApproval(text) {
			t.Errorf("IsApproval(%q) = true, expected false", text)
		}
	}
}

// ── 15. IsRejection ─────────────────────────────────────────────────────────

func TestIsRejection(t *testing.T) {
	rejectionCases := []string{
		"no", "NO", "No", "n", "N",
		"cancel", "reject", "discard", "skip",
		"dont", "don't",
	}
	for _, text := range rejectionCases {
		if !IsRejection(text) {
			t.Errorf("IsRejection(%q) = false, expected true", text)
		}
	}

	approvalCases := []string{
		"yes", "maybe", "apply",
	}
	for _, text := range approvalCases {
		if IsRejection(text) {
			t.Errorf("IsRejection(%q) = true, expected false", text)
		}
	}
}

// ── 16. CleanupExpiredEdits ─────────────────────────────────────────────────

func TestCleanupExpiredEdits(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)
	sessionID1 := helperSession(t, store)
	sessionID2 := helperSession(t, store)

	// Create files
	filePath1 := "cleanup1.txt"
	filePath2 := "cleanup2.txt"
	if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath1), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath2), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create one expired edit and one active edit
	editID1, _ := store.SavePendingEdit(sessionID1, filePath1, "new1\n", "expired")
	editID2, _ := store.SavePendingEdit(sessionID2, filePath2, "new2\n", "active")

	cm.pending[sessionID1] = &PendingEdit{
		ID:        editID1,
		SessionID: sessionID1,
		FilePath:  filePath1,
		Content:   "new1\n",
		Reason:    "expired",
		CreatedAt: time.Now().Add(-20 * time.Minute), // expired
	}
	cm.pending[sessionID2] = &PendingEdit{
		ID:        editID2,
		SessionID: sessionID2,
		FilePath:  filePath2,
		Content:   "new2\n",
		Reason:    "active",
		CreatedAt: time.Now(), // active
	}

	// Run cleanup
	count, err := cm.CleanupExpiredEdits()
	if err != nil {
		t.Fatalf("CleanupExpiredEdits failed: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 cleaned up edit, got %d", count)
	}

	// Verify expired edit is gone
	if cm.HasPendingEdit(sessionID1) {
		t.Error("expected expired edit to be cleaned up")
	}

	// Verify active edit is still there
	if !cm.HasPendingEdit(sessionID2) {
		t.Error("expected active edit to still exist")
	}
}

// ── 17. Concurrent edits ────────────────────────────────────────────────────

func TestConfirmationManager_ConcurrentEdits(t *testing.T) {
	cm, store, proj := newTestConfirmationManager(t)

	// Create sessions for 3 concurrent edits
	sessions := []string{
		helperSession(t, store),
		helperSession(t, store),
		helperSession(t, store),
	}

	// Create files for 3 sessions
	for _, s := range sessions {
		filePath := s + ".txt"
		if err := os.WriteFile(filepath.Join(proj.DirAbs, filePath), []byte("before\n"), 0o644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Create pending edits concurrently
	done := make(chan bool, len(sessions))
	for i, s := range sessions {
		go func(idx int, session string) {
			// Stagger to avoid SQLite lock contention
			time.Sleep(time.Duration(idx) * 50 * time.Millisecond)
			filePath := session + ".txt"
			_, err := cm.CreateEdit(session, filePath, "after-"+session+"\n", "concurrent test")
			if err != nil {
				t.Errorf("CreateEdit for %s failed: %v", session, err)
			}
			done <- true
		}(i, s)
	}

	// Wait for all goroutines
	for i := 0; i < len(sessions); i++ {
		<-done
	}

	// Verify all sessions have pending edits
	for _, s := range sessions {
		if !cm.HasPendingEdit(s) {
			t.Errorf("expected HasPendingEdit(%q) to return true", s)
		}
	}

	// Apply all edits concurrently
	applyDone := make(chan bool, len(sessions))
	for i, s := range sessions {
		go func(idx int, session string) {
			// Stagger to avoid SQLite lock contention
			time.Sleep(time.Duration(idx) * 50 * time.Millisecond)
			err := cm.ApplyEdit(session)
			if err != nil {
				t.Errorf("ApplyEdit for %s failed: %v", session, err)
			}
			applyDone <- true
		}(i, s)
	}

	for i := 0; i < len(sessions); i++ {
		<-applyDone
	}

	// Verify all edits are applied (no pending edits left)
	for _, s := range sessions {
		if cm.HasPendingEdit(s) {
			t.Errorf("expected HasPendingEdit(%q) to return false after apply", s)
		}
	}
}
