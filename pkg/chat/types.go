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

// ChatConfig holds the top-level chat configuration, parsed from orqen.yaml
// under the "chat:" key.
type ChatConfig struct {
	Agent    string         `yaml:"agent"`
	Telegram TelegramConfig `yaml:"telegram"`
}

// TelegramConfig holds the Telegram bot token configuration.
type TelegramConfig struct {
	Token string `yaml:"token"`
}

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
// MessageEvent is a placeholder until the Telegram library is integrated.
type CommandHandler func(ctx context.Context, bot *TelegramBot, msg MessageEvent) error

// TelegramBot is a forward declaration; the full struct lives in bot.go.
type TelegramBot struct{}

// MessageEvent is a placeholder for the Telegram message event type.
type MessageEvent struct {
	Text string
}

// CommandDef registers a command name, description, and its handler.
type CommandDef struct {
	Name        string
	Description string
	Handler     CommandHandler
}
