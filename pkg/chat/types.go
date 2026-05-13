package chat

import (
	"context"
	"time"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	SessionTTL    = 24 * time.Hour
	HistoryLimit  = 50
	SearchLimit   = 10
	MaxMessageLen = 4096 // Telegram max message length
)

const PendingEditTTL = 10 * time.Minute

// ── Configuration ────────────────────────────────────────────────────────────

// ── Session & Message types ──────────────────────────────────────────────────

// ChatSession represents a persistent conversation session for a single user.
type ChatSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"` // Telegram chat ID as string
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

// ── Pending Edit ─────────────────────────────────────────────────────────────

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

// ── Command Routing ──────────────────────────────────────────────────────────

// CommandHandler is the signature for deterministic command handlers.
// Returns an error if the command fails; response is sent via the bot.
type CommandHandler func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error)

// CommandDef registers a command name, description, and its handler.
type CommandDef struct {
	Name        string
	Description string
	Handler     CommandHandler
}
