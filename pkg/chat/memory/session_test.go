package memory

import (
	"testing"
	"time"
)

func newTestSessionManager(t *testing.T) *SessionManager {
	t.Helper()
	store := newTestStore(t)
	return NewSessionManager(store, 24*time.Hour)
}

func TestGetOrCreateSession_NewUser(t *testing.T) {
	sm := newTestSessionManager(t)

	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.ExtID != "user1" {
		t.Errorf("UserID = %q, want %q", sess.ExtID, "user1")
	}
	if !sess.Active {
		t.Error("expected session to be active")
	}
}

func TestGetOrCreateSession_ExistingUser(t *testing.T) {
	sm := newTestSessionManager(t)

	sess1, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession 1: %v", err)
	}

	sess2, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession 2: %v", err)
	}

	if sess1.ID != sess2.ID {
		t.Errorf("expected same session ID, got %q and %q", sess1.ID, sess2.ID)
	}
}

func TestGetOrCreateSession_ExpiredSession(t *testing.T) {
	sm := newTestSessionManager(t)

	// Create a session with a very short TTL so it expires quickly
	store := sm.Store
	_, err := store.CreateSession("user1", 1*time.Second)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// GetOrCreateSession should create a new one since the old is expired
	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession after expiry: %v", err)
	}
	if sess == nil {
		t.Fatal("expected new session")
	}
	if !sess.Active {
		t.Error("expected new session to be active")
	}

	// Verify the old session was deactivated
	active, err := store.GetActiveSession("user1")
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active == nil {
		t.Fatal("expected active session")
	}
	// The returned active session should be the newly created one
	if active.ID != sess.ID {
		t.Errorf("active session ID = %q, want %q", active.ID, sess.ID)
	}
}

// func TestGetOrCreateSession_MultipleSessions(t *testing.T) {
// 	sm := newTestSessionManager(t)
// 	store := sm.store

// 	// Create an active session
// 	activeSess, err := store.CreateSession("user1", 24*time.Hour)
// 	if err != nil {
// 		t.Fatalf("CreateSession active: %v", err)
// 	}

// 	// Create an expired session (insert with past expires_at)
// 	pastExpiry := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
// 	_, err = store.db.Exec(
// 		`INSERT INTO chat_sessions (id, ext_id, created_at, expires_at, active)
// 		 VALUES ('expired-1', 'user1', datetime('now'), ?, 1)`,
// 		pastExpiry,
// 	)
// 	if err != nil {
// 		t.Fatalf("insert expired session: %v", err)
// 	}

// 	sess, err := sm.GetOrCreateSession("user1")
// 	if err != nil {
// 		t.Fatalf("GetOrCreateSession: %v", err)
// 	}

// 	// Should return the active session, not create a new one
// 	if sess.ID != activeSess.ID {
// 		t.Errorf("expected active session %q, got %q", activeSess.ID, sess.ID)
// 	}
// }

func TestGetSessionHistory(t *testing.T) {
	sm := newTestSessionManager(t)
	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Add 5 messages
	for i := 1; i <= 5; i++ {
		if err := sm.Store.AddMessage(sess.ID, RoleUser, "msg"); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	// Default limit (HistoryLimit = 50, so all 5 returned)
	msgs, err := sm.GetSessionHistory(sess.ID, 0)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages, got %d", len(msgs))
	}
}

func TestGetSessionHistory_CustomLimit(t *testing.T) {
	sm := newTestSessionManager(t)
	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	for i := 1; i <= 10; i++ {
		if err := sm.Store.AddMessage(sess.ID, RoleUser, "msg"); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	msgs, err := sm.GetSessionHistory(sess.ID, 3)
	if err != nil {
		t.Fatalf("GetSessionHistory limit=3: %v", err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}
}

func TestGetFullHistoryForContext(t *testing.T) {
	sm := newTestSessionManager(t)
	sess, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Add more messages than HistoryLimit
	for i := 1; i <= 60; i++ {
		if err := sm.Store.AddMessage(sess.ID, RoleUser, "msg"); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	msgs, err := sm.GetFullHistoryForContext(sess.ID)
	if err != nil {
		t.Fatalf("GetFullHistoryForContext: %v", err)
	}
	if len(msgs) != 60 {
		t.Errorf("expected 60 messages (no limit), got %d", len(msgs))
	}
}

func TestSearchConversations(t *testing.T) {
	sm := newTestSessionManager(t)

	sess1, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession user1: %v", err)
	}
	sess2, err := sm.GetOrCreateSession("user2")
	if err != nil {
		t.Fatalf("GetOrCreateSession user2: %v", err)
	}

	_ = sess1 // sessions created, now add distinct messages
	if err := sm.Store.AddMessage(sess1.ID, RoleUser, "unique keyword alpha"); err != nil {
		t.Fatalf("AddMessage user1: %v", err)
	}
	if err := sm.Store.AddMessage(sess2.ID, RoleUser, "different content beta"); err != nil {
		t.Fatalf("AddMessage user2: %v", err)
	}

	results, err := sm.SearchConversations("user1", "keyword", 10)
	if err != nil {
		t.Fatalf("SearchConversations: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for user1, got %d", len(results))
	}
	if results[0].ExtID != "user1" {
		t.Errorf("result UserID = %q, want %q", results[0].ExtID, "user1")
	}
}

func TestNewSession(t *testing.T) {
	sm := newTestSessionManager(t)

	// Create an active session
	sess1, err := sm.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Create a new session
	sess2, err := sm.NewSession("user1")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if sess2.ID == sess1.ID {
		t.Error("expected different session ID")
	}
	if !sess2.Active {
		t.Error("expected new session to be active")
	}

	// Verify old session is deactivated
	oldSess, err := sm.Store.LoadSession(sess1.ID)
	if err != nil {
		t.Fatalf("LoadSession old: %v", err)
	}
	if oldSess == nil {
		t.Fatal("expected old session to still exist (but inactive)")
	}
	if oldSess.Active {
		t.Error("expected old session to be inactive")
	}
}

func TestNewSession_NoPriorSession(t *testing.T) {
	sm := newTestSessionManager(t)

	sess, err := sm.NewSession("user1")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if sess == nil {
		t.Fatal("expected new session")
	}
	if !sess.Active {
		t.Error("expected session to be active")
	}
}
