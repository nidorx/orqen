package memory

import (
	"fmt"
	"time"
)

// SessionManager wraps ChatStore with higher-level session operations.
type SessionManager struct {
	Store *ChatStore
	ttl   time.Duration
}

// NewSessionManager creates a SessionManager for the given store and TTL.
// The database is already scoped to the project, so no projectID is needed.
func NewSessionManager(store *ChatStore, ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = SessionTTL
	}
	return &SessionManager{Store: store, ttl: ttl}
}

// GetOrCreateSession returns an active session for the user, or creates one.
// Expired sessions are deactivated first.
func (sm *SessionManager) GetOrCreateSession(extId string) (*ChatSession, error) {
	// Deactivate any expired sessions for this user
	if _, err := sm.Store.ExpireOldSessions(extId, sm.ttl); err != nil {
		return nil, fmt.Errorf("chat: expire old sessions in GetOrCreate: %w", err)
	}

	// Try to find an active session
	sess, err := sm.Store.GetActiveSession(extId)
	if err != nil {
		return nil, fmt.Errorf("chat: get active session: %w", err)
	}
	if sess != nil {
		return sess, nil
	}

	// No active session - create a new one
	sess, err = sm.Store.CreateSession(extId, sm.ttl)
	if err != nil {
		return nil, fmt.Errorf("chat: create session in GetOrCreate: %w", err)
	}
	return sess, nil
}

// GetSessionHistory returns up to `limit` messages for the given session.
// If limit <= 0, HistoryLimit is used.
func (sm *SessionManager) GetSessionHistory(sessionID string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = HistoryLimit
	}
	return sm.Store.GetHistory(sessionID, limit)
}

// GetFullHistoryForContext returns ALL messages for the session in chronological
// order, with no limit. Used to build conversation context for the ACP agent.
func (sm *SessionManager) GetFullHistoryForContext(sessionID string) ([]ChatMessage, error) {
	return sm.Store.GetHistory(sessionID, -1)
}

// SearchConversations searches the user's messages by keyword.
// If limit <= 0, SearchLimit is used.
func (sm *SessionManager) SearchConversations(extId, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = SearchLimit
	}
	return sm.Store.Search(extId, query, limit)
}

// NewSession expires all current active sessions for the user and creates a
// brand new one. Returns the new session.
func (sm *SessionManager) NewSession(extId string) (*ChatSession, error) {
	// Expire all current active sessions
	if _, err := sm.Store.ExpireAllSessions(extId); err != nil {
		return nil, fmt.Errorf("chat: expire all sessions in NewSession: %w", err)
	}

	// Create new session
	sess, err := sm.Store.CreateSession(extId, sm.ttl)
	if err != nil {
		return nil, fmt.Errorf("chat: create session in NewSession: %w", err)
	}
	return sess, nil
}
