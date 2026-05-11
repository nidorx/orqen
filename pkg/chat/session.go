package chat

import (
	"fmt"
	"strings"
	"time"
)

const (
	// maxMessageFormatLen is the per-message character limit when formatting
	// history for the ACP agent prompt.
	maxMessageFormatLen = 2000

	// maxTotalPromptLen is the maximum total length of the formatted prompt.
	// When exceeded, we keep the first headLen and last tailLen characters.
	maxTotalPromptLen = 8000
	headLen           = 2000
	tailLen           = 6000
)

// SessionManager wraps ChatStore with higher-level session operations.
type SessionManager struct {
	store *ChatStore
	ttl   time.Duration
}

// NewSessionManager creates a SessionManager for the given store and TTL.
// The database is already scoped to the project, so no projectID is needed.
func NewSessionManager(store *ChatStore, ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = SessionTTL
	}
	return &SessionManager{store: store, ttl: ttl}
}

// GetOrCreateSession returns an active session for the user, or creates one.
// Expired sessions are deactivated first.
func (sm *SessionManager) GetOrCreateSession(userID string) (*ChatSession, error) {
	// Deactivate any expired sessions for this user
	if _, err := sm.store.ExpireOldSessions(userID, sm.ttl); err != nil {
		return nil, fmt.Errorf("chat: expire old sessions in GetOrCreate: %w", err)
	}

	// Try to find an active session
	sess, err := sm.store.GetActiveSession(userID)
	if err != nil {
		return nil, fmt.Errorf("chat: get active session: %w", err)
	}
	if sess != nil {
		return sess, nil
	}

	// No active session — create a new one
	sess, err = sm.store.CreateSession(userID, sm.ttl)
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
	return sm.store.GetHistory(sessionID, limit)
}

// GetFullHistoryForContext returns ALL messages for the session in chronological
// order, with no limit. Used to build conversation context for the ACP agent.
func (sm *SessionManager) GetFullHistoryForContext(sessionID string) ([]ChatMessage, error) {
	return sm.store.GetHistory(sessionID, -1)
}

// SearchConversations searches the user's messages by keyword.
// If limit <= 0, SearchLimit is used.
func (sm *SessionManager) SearchConversations(userID, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = SearchLimit
	}
	return sm.store.Search(userID, query, limit)
}

// NewSession expires all current active sessions for the user and creates a
// brand new one. Returns the new session.
func (sm *SessionManager) NewSession(userID string) (*ChatSession, error) {
	// Expire all current active sessions
	if _, err := sm.store.ExpireAllSessions(userID); err != nil {
		return nil, fmt.Errorf("chat: expire all sessions in NewSession: %w", err)
	}

	// Create new session
	sess, err := sm.store.CreateSession(userID, sm.ttl)
	if err != nil {
		return nil, fmt.Errorf("chat: create session in NewSession: %w", err)
	}
	return sess, nil
}

// FormatHistoryForPrompt formats a list of messages into a string suitable for
// an ACP agent system prompt.
//
// Rules:
//   - "system" role messages are skipped (injected separately).
//   - Individual messages longer than maxMessageFormatLen are truncated with
//     "...[truncated]...".
//   - If the total output exceeds maxTotalPromptLen, we keep the first headLen
//     and last tailLen characters to preserve recent context.
func FormatHistoryForPrompt(messages []ChatMessage) string {
	var sb strings.Builder

	for _, m := range messages {
		if m.Role == RoleSystem {
			continue
		}

		content := m.Content
		if len(content) > maxMessageFormatLen {
			content = content[:maxMessageFormatLen] + "...[truncated]..."
		}

		fmt.Fprintf(&sb, "%s: %s\n", m.Role, content)
	}

	result := sb.String()

	// If total exceeds max, keep head + tail
	if len(result) > maxTotalPromptLen {
		result = result[:headLen] + "\n...[middle truncated]...\n" + result[len(result)-tailLen:]
	}

	return result
}
