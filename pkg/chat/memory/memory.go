package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	HistoryLimit   = 50
	SearchLimit    = 10
	SessionTTL     = 24 * time.Hour
	PendingEditTTL = 10 * time.Minute
)

// ChatSession represents a persistent conversation session for a single user.
type ChatSession struct {
	ID        string    `json:"id"`
	ExtID     string    `json:"ext_id"` // Telegram chat ID as string
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `json:"active"`
}

// MessageRole indicates the origin of a chat message.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// ChatMessage represents a single message within a chat session.
type ChatMessage struct {
	ID        int64       `json:"id"`
	SessionID string      `json:"session_id"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
}

// SearchResult holds a single FTS5 search hit with session metadata.
type SearchResult struct {
	MessageID int64     `json:"message_id"`
	SessionID string    `json:"session_id"`
	ExtID     string    `json:"ext_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Rank      float64   `json:"rank"`
	CreatedAt time.Time `json:"created_at"`
}

// PendingEdit holds a proposed file edit awaiting user confirmation.
type PendingEdit struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	FilePath  string    `json:"file_path"`
	Content   string    `json:"content"` // Full proposed file content (not a diff)
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired returns true if the pending edit has exceeded PendingEditTTL.
func (p *PendingEdit) IsExpired() bool {
	return time.Since(p.CreatedAt) > PendingEditTTL
}

// ChatStore

// ChatStore is a SQLite-backed store for chat sessions, messages, and pending edits.
type ChatStore struct {
	db *sql.DB
}

// NewChatStore opens (or creates) the chat database at dbPath and runs schema
// initialization. WAL mode is enabled for concurrent reads.
func NewChatStore(dbPath string) (*ChatStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("chat: open database: %w", err)
	}

	// Performance pragmas
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("chat: pragma %q: %w", p, err)
		}
	}

	s := &ChatStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("chat: migration: %w", err)
	}
	return s, nil
}

// Close releases the underlying database connection.
func (s *ChatStore) Close() error {
	return s.db.Close()
}

// Migration

func (s *ChatStore) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS chat_sessions (
			id         TEXT PRIMARY KEY,
			ext_id     TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL,
			active     INTEGER DEFAULT 1
		);

		CREATE TABLE IF NOT EXISTS chat_messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES chat_sessions(id)
		);

		CREATE TABLE IF NOT EXISTS pending_edits (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			file_path  TEXT NOT NULL,
			content    TEXT NOT NULL,
			reason     TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES chat_sessions(id)
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS chat_messages_fts USING fts5(
			content,
			content_rowid=id,
			tokenize='porter unicode61'
		);

		CREATE TRIGGER IF NOT EXISTS chat_messages_fts_insert
		AFTER INSERT ON chat_messages
		BEGIN
			INSERT INTO chat_messages_fts(rowid, content) VALUES (new.id, new.content);
		END;

		CREATE TRIGGER IF NOT EXISTS chat_messages_fts_delete
		AFTER DELETE ON chat_messages
		BEGIN
			INSERT INTO chat_messages_fts(chat_messages_fts, rowid, content)
			VALUES('delete', old.id, old.content);
		END;

		CREATE TRIGGER IF NOT EXISTS chat_messages_fts_update
		AFTER UPDATE ON chat_messages
		BEGIN
			INSERT INTO chat_messages_fts(chat_messages_fts, rowid, content)
			VALUES('delete', old.id, old.content);
			INSERT INTO chat_messages_fts(rowid, content) VALUES (new.id, new.content);
		END;
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("chat: create schema: %w", err)
	}
	return nil
}

// Session operations

// CreateSession generates a new UUID, stores the session with expires_at = now+ttl,
// and returns the resulting ChatSession.
func (s *ChatStore) CreateSession(extID string, ttl time.Duration) (*ChatSession, error) {
	now := time.Now().UTC()
	sessionID := uuid.New().String()
	expiresAt := now.Add(ttl)

	sess := &ChatSession{
		ID:        sessionID,
		ExtID:     extID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Active:    true,
	}

	_, err := s.db.Exec(
		`INSERT INTO chat_sessions (id, ext_id, created_at, expires_at, active)
		 VALUES (?, ?, ?, ?, 1)`,
		sessionID, extID, now.Format(time.RFC3339),
		expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("chat: create session: %w", err)
	}
	return sess, nil
}

// LoadSession fetches a session by its ID. Returns nil if not found.
func (s *ChatStore) LoadSession(sessionID string) (*ChatSession, error) {
	var sess ChatSession
	var active int
	err := s.db.QueryRow(
		`SELECT id, ext_id, created_at, expires_at, active
		 FROM chat_sessions WHERE id = ?`,
		sessionID,
	).Scan(&sess.ID, &sess.ExtID, &sess.CreatedAt, &sess.ExpiresAt, &active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: load session: %w", err)
	}
	sess.Active = active == 1
	return &sess, nil
}

// GetActiveSession returns the most recent active (non-expired) session for the
// given external id. Returns nil if none found.
func (s *ChatStore) GetActiveSession(extID string) (*ChatSession, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var sess ChatSession
	var active int
	err := s.db.QueryRow(
		`SELECT id, ext_id, created_at, expires_at, active
		 FROM chat_sessions
		 WHERE ext_id = ? AND active = 1 AND expires_at > ?
		 ORDER BY created_at DESC LIMIT 1`,
		extID, now,
	).Scan(&sess.ID, &sess.ExtID, &sess.CreatedAt, &sess.ExpiresAt, &active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: get active session: %w", err)
	}
	sess.Active = active == 1
	return &sess, nil
}

// ExpireSession sets active = 0 for the given session.
func (s *ChatStore) ExpireSession(sessionID string) error {
	res, err := s.db.Exec(
		`UPDATE chat_sessions SET active = 0 WHERE id = ?`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("chat: expire session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("chat: expire session rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("chat: expire session: session %q not found", sessionID)
	}
	return nil
}

// ExpireOldSessions deactivates all sessions for the given external id whose expires_at
// is before now. Returns the count of affected rows.
func (s *ChatStore) ExpireOldSessions(extId string, ttl time.Duration) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE chat_sessions SET active = 0
		 WHERE ext_id = ? AND active = 1 AND expires_at <= ?`,
		extId, now,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: expire old sessions: %w", err)
	}
	return res.RowsAffected()
}

// ExpireAllSessions deactivates all active sessions for the given external id,
// regardless of expiry time. Used when the user explicitly requests a new session.
func (s *ChatStore) ExpireAllSessions(extId string) (int64, error) {
	res, err := s.db.Exec(
		`UPDATE chat_sessions SET active = 0
		 WHERE ext_id = ? AND active = 1`,
		extId,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: expire all sessions: %w", err)
	}
	return res.RowsAffected()
}

// Message operations

// AddMessage inserts a single message into the store. The FTS5 trigger keeps
// the virtual table in sync automatically.
func (s *ChatStore) AddMessage(sessionID string, role MessageRole, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO chat_messages (session_id, role, content, created_at)
		 VALUES (?, ?, ?, ?)`,
		sessionID, string(role), content, now,
	)
	if err != nil {
		return fmt.Errorf("chat: add message: %w", err)
	}
	return nil
}

// AddMessages inserts multiple messages in a single transaction.
func (s *ChatStore) AddMessages(sessionID string, msgs []ChatMessage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("chat: add messages begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO chat_messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("chat: add messages prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range msgs {
		createdAt := m.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		if _, err := stmt.Exec(sessionID, string(m.Role), m.Content, createdAt.Format(time.RFC3339)); err != nil {
			return fmt.Errorf("chat: add messages exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("chat: add messages commit: %w", err)
	}
	return nil
}

// GetHistory returns up to `limit` messages for the given session in
// chronological order. If limit <= 0, HistoryLimit is used.
// If limit == -1, no limit is applied (all messages returned).
func (s *ChatStore) GetHistory(sessionID string, limit int) ([]ChatMessage, error) {
	if limit == -1 {
		// No limit - fetch all messages
		return s.getHistoryUnlimited(sessionID)
	}
	if limit <= 0 {
		limit = HistoryLimit
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, created_at
		 FROM chat_messages
		 WHERE session_id = ?
		 ORDER BY created_at ASC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: get history query: %w", err)
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: get history scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: get history iterate: %w", err)
	}
	return msgs, nil
}

// getHistoryUnlimited returns all messages for the session with no LIMIT clause.
func (s *ChatStore) getHistoryUnlimited(sessionID string) ([]ChatMessage, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, created_at
		 FROM chat_messages
		 WHERE session_id = ?
		 ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: get history unlimited query: %w", err)
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: get history unlimited scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: get history unlimited iterate: %w", err)
	}
	return msgs, nil
}

// Search

// sanitizeFTS escapes FTS5 special characters by wrapping each whitespace-separated
// token in double quotes, preventing syntax errors from user input.
func sanitizeFTS(query string) string {
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return ""
	}
	escaped := make([]string, len(parts))
	for i, p := range parts {
		// Strip FTS5 special characters that could break the MATCH syntax
		clean := strings.NewReplacer(
			`"`, "", `\`, "", `(`, "", `)`, "",
		).Replace(p)
		escaped[i] = `"` + clean + `"`
	}
	return strings.Join(escaped, " ")
}

// Search performs an FTS5 search across the given user's messages.
// Results are ordered by FTS5 relevance rank (best first) and limited.
func (s *ChatStore) Search(extId string, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = SearchLimit
	}

	sanitized := sanitizeFTS(query)
	if sanitized == "" {
		return nil, nil
	}

	rows, err := s.db.Query(
		`SELECT m.id, m.session_id, cs.ext_id, m.role, m.content, fts.rank, m.created_at
		 FROM chat_messages_fts fts
		 JOIN chat_messages m ON m.id = fts.rowid
		 JOIN chat_sessions cs ON cs.id = m.session_id
		 WHERE fts.content MATCH ? AND cs.ext_id = ?
		 ORDER BY fts.rank
		 LIMIT ?`,
		sanitized, extId, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.MessageID, &r.SessionID, &r.ExtID, &r.Role, &r.Content, &r.Rank, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: search scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: search iterate: %w", err)
	}
	return results, nil
}

// Pending Edit operations

// SavePendingEdit inserts a pending edit and returns its auto-increment ID.
func (s *ChatStore) SavePendingEdit(sessionID, filePath, content, reason string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO pending_edits (session_id, file_path, content, reason, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, filePath, content, reason, now,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: save pending edit: %w", err)
	}
	return res.LastInsertId()
}

// GetPendingEdit fetches a pending edit by its ID. Returns nil if not found.
func (s *ChatStore) GetPendingEdit(editID int64) (*PendingEdit, error) {
	var pe PendingEdit
	err := s.db.QueryRow(
		`SELECT id, session_id, file_path, content, reason, created_at
		 FROM pending_edits WHERE id = ?`,
		editID,
	).Scan(&pe.ID, &pe.SessionID, &pe.FilePath, &pe.Content, &pe.Reason, &pe.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: get pending edit: %w", err)
	}
	return &pe, nil
}

// DeletePendingEdit removes a pending edit by ID.
func (s *ChatStore) DeletePendingEdit(editID int64) error {
	res, err := s.db.Exec(`DELETE FROM pending_edits WHERE id = ?`, editID)
	if err != nil {
		return fmt.Errorf("chat: delete pending edit: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("chat: delete pending edit rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("chat: delete pending edit: edit %d not found", editID)
	}
	return nil
}

// ExpirePendingEdits deletes all pending edits for a session whose created_at is
// older than PendingEditTTL. Returns the count of deleted rows.
func (s *ChatStore) ExpirePendingEdits(sessionID string) (int64, error) {
	cutoff := time.Now().UTC().Add(-PendingEditTTL).Format(time.RFC3339)
	res, err := s.db.Exec(
		`DELETE FROM pending_edits WHERE session_id = ? AND created_at <= ?`,
		sessionID, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: expire pending edits: %w", err)
	}
	return res.RowsAffected()
}

// cleanupStoreEdits deletes all expired pending edits from the database.
// Returns the count of deleted rows.
func (s *ChatStore) CleanupStoreEdits() (int64, error) {
	cutoff := time.Now().UTC().Add(-PendingEditTTL).Format(time.RFC3339)
	res, err := s.db.Exec(
		`DELETE FROM pending_edits WHERE created_at <= ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: cleanup expired edits: %w", err)
	}
	return res.RowsAffected()
}
