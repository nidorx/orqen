package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *ChatStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	s, err := NewChatStore(dbPath)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestNewChatStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	s, err := NewChatStore(dbPath)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	defer s.Close()

	// Database file exists
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	// WAL mode is enabled
	var journalMode string
	err = s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	// Tables exist
	tables := []string{"chat_sessions", "chat_messages", "pending_edits", "chat_messages_fts"}
	for _, tbl := range tables {
		var count int
		err := s.db.QueryRow(
			`SELECT count(*) FROM sqlite_master WHERE type IN ('table','virtual table') AND name = ?`,
			tbl,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query sqlite_master: %v", err)
		}
		if count != 1 {
			t.Errorf("table %q not found (count=%d)", tbl, count)
		}
	}
}

func TestCreateSession(t *testing.T) {
	s := newTestStore(t)
	ttl := 24 * time.Hour

	sess, err := s.CreateSession("user1", ttl)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sess.ExtID != "user1" {
		t.Errorf("UserID = %q, want %q", sess.ExtID, "user1")
	}
	if !sess.Active {
		t.Error("expected session to be active")
	}
	if sess.ExpiresAt.Before(time.Now().UTC().Add(23 * time.Hour)) {
		t.Error("expires_at should be at least ~24h in the future")
	}
}

func TestLoadSession(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	loaded, err := s.LoadSession(sess.ID)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil session")
	}
	if loaded.ID != sess.ID {
		t.Errorf("loaded ID = %q, want %q", loaded.ID, sess.ID)
	}

	// Non-existent session returns nil
	nonExist, err := s.LoadSession("nonexistent")
	if err != nil {
		t.Fatalf("LoadSession nonexistent: %v", err)
	}
	if nonExist != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestGetActiveSession(t *testing.T) {
	s := newTestStore(t)

	// No session yet - should return nil
	active, err := s.GetActiveSession("user1")
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active != nil {
		t.Error("expected nil when no session exists")
	}

	// Create a session
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	active, err = s.GetActiveSession("user1")
	if err != nil {
		t.Fatalf("GetActiveSession after create: %v", err)
	}
	if active == nil {
		t.Fatal("expected active session")
	}
	if active.ID != sess.ID {
		t.Errorf("active ID = %q, want %q", active.ID, sess.ID)
	}
}

func TestExpireSession(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := s.ExpireSession(sess.ID); err != nil {
		t.Fatalf("ExpireSession: %v", err)
	}

	// Should no longer be found as active
	active, err := s.GetActiveSession("user1")
	if err != nil {
		t.Fatalf("GetActiveSession after expire: %v", err)
	}
	if active != nil {
		t.Error("expected no active session after expiry")
	}

	// But loadable
	loaded, err := s.LoadSession(sess.ID)
	if err != nil {
		t.Fatalf("LoadSession after expire: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil session")
	}
	if loaded.Active {
		t.Error("expected session to be inactive after expiry")
	}
}

func TestExpireOldSessions(t *testing.T) {
	s := newTestStore(t)

	// Create one session that will already be expired (short TTL in the past)
	sess1, err := s.CreateSession("user1", 1*time.Second)
	if err != nil {
		t.Fatalf("CreateSession 1: %v", err)
	}
	// Wait for it to expire
	time.Sleep(1500 * time.Millisecond)

	// Create another session that's still active
	sess2, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession 2: %v", err)
	}

	n, err := s.ExpireOldSessions("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("ExpireOldSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("expired %d sessions, want 1", n)
	}

	// sess1 should be inactive
	s1, err := s.LoadSession(sess1.ID)
	if err != nil {
		t.Fatalf("LoadSession 1: %v", err)
	}
	if s1.Active {
		t.Error("sess1 should be inactive")
	}

	// sess2 should still be active
	s2, err := s.LoadSession(sess2.ID)
	if err != nil {
		t.Fatalf("LoadSession 2: %v", err)
	}
	if !s2.Active {
		t.Error("sess2 should still be active")
	}
}

func TestAddMessage(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := s.AddMessage(sess.ID, RoleUser, "hello world"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	history, err := s.GetHistory(sess.ID, 10)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
	if history[0].Role != RoleUser {
		t.Errorf("role = %q, want %q", history[0].Role, RoleUser)
	}
	if history[0].Content != "hello world" {
		t.Errorf("content = %q, want %q", history[0].Content, "hello world")
	}
}

func TestGetHistory(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Add 5 messages
	for i := 1; i <= 5; i++ {
		if err := s.AddMessage(sess.ID, RoleUser, fmt.Sprintf("msg %d", i)); err != nil {
			t.Fatalf("AddMessage %d: %v", i, err)
		}
	}

	// Get with limit=3
	msgs, err := s.GetHistory(sess.ID, 3)
	if err != nil {
		t.Fatalf("GetHistory limit=3: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// Chronological order
	if msgs[0].Content != "msg 1" {
		t.Errorf("first = %q, want %q", msgs[0].Content, "msg 1")
	}
	if msgs[2].Content != "msg 3" {
		t.Errorf("third = %q, want %q", msgs[2].Content, "msg 3")
	}
}

func TestSearch_Basic(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := s.AddMessage(sess.ID, RoleUser, "the quick brown fox"); err != nil {
		t.Fatalf("AddMessage fox: %v", err)
	}
	if err := s.AddMessage(sess.ID, RoleAssistant, "hello world"); err != nil {
		t.Fatalf("AddMessage hello: %v", err)
	}

	results, err := s.Search("user1", "fox", 10)
	if err != nil {
		t.Fatalf("Search fox: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "the quick brown fox" {
		t.Errorf("content = %q, want %q", results[0].Content, "the quick brown fox")
	}
}

func TestSearch_Sanitization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{"JWT auth", `"JWT" "auth"`},
		{"(parentheses)", `"parentheses"`},
		{`"quoted"`, `"quoted"`},
		{"word1   word2", `"word1" "word2"`},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		got := sanitizeFTS(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFTS(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSearch_FTSTriggers(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Insert and immediately search
	if err := s.AddMessage(sess.ID, RoleUser, "immediately indexed message"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	results, err := s.Search("user1", "indexed", 10)
	if err != nil {
		t.Fatalf("Search immediately: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected FTS5 trigger to index message immediately")
	}
}

func TestSearch_ScopedByUser(t *testing.T) {
	s := newTestStore(t)

	sess1, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession user1: %v", err)
	}
	sess2, err := s.CreateSession("user2", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession user2: %v", err)
	}

	if err := s.AddMessage(sess1.ID, RoleUser, "unique to user1"); err != nil {
		t.Fatalf("AddMessage user1: %v", err)
	}
	if err := s.AddMessage(sess2.ID, RoleUser, "unique to user1"); err != nil {
		t.Fatalf("AddMessage user2: %v", err)
	}

	// Search for user1 - should only see user1's session
	results1, err := s.Search("user1", "unique", 10)
	if err != nil {
		t.Fatalf("Search user1: %v", err)
	}

	// Search for user2 - should only see user2's session
	results2, err := s.Search("user2", "unique", 10)
	if err != nil {
		t.Fatalf("Search user2: %v", err)
	}

	if len(results1) == 0 {
		t.Error("user1 search returned no results")
	}
	if len(results2) == 0 {
		t.Error("user2 search returned no results")
	}

	// Verify each result belongs to the correct user
	for _, r := range results1 {
		if r.ExtID != "user1" {
			t.Errorf("user1 search result has UserID = %q, want %q", r.ExtID, "user1")
		}
	}
	for _, r := range results2 {
		if r.ExtID != "user2" {
			t.Errorf("user2 search result has UserID = %q, want %q", r.ExtID, "user2")
		}
	}
}

func TestPendingEdit_Lifecycle(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	editID, err := s.SavePendingEdit(sess.ID, "main.go", "package main\n", "add logging")
	if err != nil {
		t.Fatalf("SavePendingEdit: %v", err)
	}
	if editID <= 0 {
		t.Errorf("expected positive edit ID, got %d", editID)
	}

	// Get
	pe, err := s.GetPendingEdit(editID)
	if err != nil {
		t.Fatalf("GetPendingEdit: %v", err)
	}
	if pe == nil {
		t.Fatal("expected non-nil pending edit")
	}
	if pe.FilePath != "main.go" {
		t.Errorf("FilePath = %q, want %q", pe.FilePath, "main.go")
	}
	if pe.Content != "package main\n" {
		t.Errorf("Content = %q, want %q", pe.Content, "package main\n")
	}

	// Delete
	if err := s.DeletePendingEdit(editID); err != nil {
		t.Fatalf("DeletePendingEdit: %v", err)
	}

	// Should be gone
	gone, err := s.GetPendingEdit(editID)
	if err != nil {
		t.Fatalf("GetPendingEdit after delete: %v", err)
	}
	if gone != nil {
		t.Error("expected nil after delete")
	}
}

func TestPendingEdit_Expiry(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Insert an edit with a created_at in the past (beyond TTL)
	oldTime := time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO pending_edits (session_id, file_path, content, reason, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sess.ID, "old.go", "old content", "stale", oldTime,
	)
	if err != nil {
		t.Fatalf("insert old pending edit: %v", err)
	}

	// Also insert a fresh one that should NOT be expired
	_, err = s.SavePendingEdit(sess.ID, "new.go", "new content", "fresh")
	if err != nil {
		t.Fatalf("SavePendingEdit fresh: %v", err)
	}

	n, err := s.ExpirePendingEdits(sess.ID)
	if err != nil {
		t.Fatalf("ExpirePendingEdits: %v", err)
	}
	if n != 1 {
		t.Errorf("expired %d edits, want 1", n)
	}
}

func TestConcurrentReads(t *testing.T) {
	s := newTestStore(t)
	sess, err := s.CreateSession("user1", 24*time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Seed some messages
	for i := 0; i < 10; i++ {
		if err := s.AddMessage(sess.ID, RoleUser, fmt.Sprintf("concurrent msg %d", i)); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgs, err := s.GetHistory(sess.ID, 10)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: %v", i, err)
				return
			}
			if len(msgs) != 10 {
				errCh <- fmt.Errorf("goroutine %d: expected 10 msgs, got %d", i, len(msgs))
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}
