package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/nidorx/orqen/pkg/engine"
)

// Telegram max message length in characters
const telegramMaxMessageLength = 4096

// ── TelegramChannel — Implements Channel Interface ──────────────────────────

// TelegramChannel bridges AgentSession and Telegram. The agent sends messages
// to the user through this channel.
type TelegramChannel struct {
	bot    *bot.Bot
	chatID int64 // Telegram chat ID
}

// Send delivers the final agent response to the user via Telegram.
// Splits text if > 4096 chars (Telegram message limit).
// ENVIA PARA USUARIO: delivers final agent response via Telegram
func (tc *TelegramChannel) Send(ctx context.Context, sessionID, text string) error {
	chunks := splitMessage(text, telegramMaxMessageLength)
	for _, chunk := range chunks {
		_, err := tc.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: tc.chatID,
			Text:   chunk,
		})
		if err != nil {
			return fmt.Errorf("telegram send: %w", err)
		}
	}
	return nil
}

// OnProgress delivers streaming progress messages to the user.
// ENVIA PARA USUARIO: streaming progress (thinking, tool calls)
// Examples: "🔄 Thinking...", "🔧 Calling tool: read_file..."
func (tc *TelegramChannel) OnProgress(ctx context.Context, sessionID, text string) error {
	if text == "" {
		return nil
	}
	_, err := tc.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: tc.chatID,
		Text:   text,
	})
	return err
}

// ── TelegramBot Struct ──────────────────────────────────────────────────────

// TelegramBot holds the bot configuration and project references.
// Exported fields are accessed by command handlers (cmd_*.go).
type TelegramBot struct {
	bot            *bot.Bot
	Project        *engine.Project
	ChatStore      *ChatStore
	SessionManager *SessionManager
	confirmMgr     ConfirmationManager
	chatMCPURL     string // URL to chat MCP server
	agentName      string // Agent to use for chat conversations

	// Agent sessions keyed by Telegram chat ID (one per user)
	agentSessions   map[int64]*AgentSession
	agentSessionsMu sync.Mutex
}

// ── Bot Initialization ──────────────────────────────────────────────────────

// NewTelegramBot creates a new Telegram bot instance.
func NewTelegramBot(
	token string,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
	confirmMgr ConfirmationManager,
	chatMCPURL string,
	agentName string,
) (*TelegramBot, error) {
	b, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("chat: create telegram bot: %w", err)
	}

	tb := &TelegramBot{
		bot:            b,
		Project:        proj,
		ChatStore:      chatStore,
		SessionManager: sessionMgr,
		confirmMgr:     confirmMgr,
		chatMCPURL:     chatMCPURL,
		agentName:      agentName,
		agentSessions:  make(map[int64]*AgentSession),
	}

	// Register handler for all text messages
	b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, tb.handleMessage)

	return tb, nil
}

// ── Message Handler ─────────────────────────────────────────────────────────

// handleMessage is the main entry point for all incoming Telegram messages.
func (tb *TelegramBot) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.Text == "" {
		// Non-text message (photo, document, etc.)
		if update != nil && update.Message != nil {
			// ENVIA PARA USUARIO: "I can only process text messages."
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "I can only process text messages.",
			})
		}
		return
	}

	text := update.Message.Text
	chatID := update.Message.Chat.ID

	// Get or create chat session
	session, err := tb.SessionManager.GetOrCreateSession(fmt.Sprintf("%d", chatID))
	if err != nil {
		slog.ErrorContext(ctx, "chat: get or create session", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ An internal error occurred.",
		})
		return
	}

	// Route based on content
	if strings.HasPrefix(text, "/") {
		tb.handleCommand(ctx, session, text, chatID)
	} else {
		tb.handleChatMessage(ctx, session, text, chatID)
	}
}

// ── Command Handler ─────────────────────────────────────────────────────────

// handleCommand routes a command to the appropriate handler.
func (tb *TelegramBot) handleCommand(ctx context.Context, session *ChatSession, text string, chatID int64) {
	// Parse command: strip leading '/', split on first space → (cmdName, args)
	cmdName, args, ok := ParseCommand(text)
	if !ok {
		_, _ = tb.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Invalid command format.",
		})
		return
	}

	// Look up command in registry
	cmd, found := GetCommand(cmdName)
	if !found {
		_, _ = tb.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Unknown command: /%s\nType /help for available commands.", cmdName),
		})
		return
	}

	// Execute command handler
	userID := fmt.Sprintf("%d", chatID)
	response, err := cmd.Handler(ctx, args, tb, userID)
	if err != nil {
		slog.ErrorContext(ctx, "chat: command handler error",
			"command", cmdName, "error", err)
		// ENVIA PARA USUARIO: error message
		_, _ = tb.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Error executing /%s: %v", cmdName, err),
		})
		return
	}

	// ENVIA PARA USUARIO: response
	if response != "" {
		tb.sendFormatted(ctx, chatID, response)
	}
}

// ── Chat Message Handler (AI Agent) ─────────────────────────────────────────

// handleChatMessage routes a non-command message to the AI agent via AgentSession.
func (tb *TelegramBot) handleChatMessage(ctx context.Context, session *ChatSession, text string, chatID int64) {
	// Get or create AgentSession for this user
	as := tb.getOrCreateAgentSession(session.ID, chatID)

	// Save user message to store
	if err := tb.ChatStore.AddMessage(session.ID, RoleUser, text); err != nil {
		slog.ErrorContext(ctx, "chat: save user message", "error", err)
	}

	// Call AgentSession.Prompt — internally Channel.Send(response) ENVIA PARA USUARIO
	response, err := as.Prompt(ctx, text)
	if err != nil {
		slog.ErrorContext(ctx, "chat: agent prompt error", "error", err)
		// ENVIA PARA USUARIO: user-friendly error message
		_, _ = tb.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Error processing your request: " + formatUserFriendlyError(err),
		})
		return
	}

	// Response already sent via Channel.Send() inside Prompt()
	// Log for debugging
	if response != "" {
		slog.DebugContext(ctx, "chat: agent response delivered", "length", len(response))
	}
}

// ── AgentSession Management ─────────────────────────────────────────────────

// getOrCreateAgentSession returns an existing AgentSession for the chat ID,
// or creates a new one.
func (tb *TelegramBot) getOrCreateAgentSession(chatSessionID string, chatID int64) *AgentSession {
	tb.agentSessionsMu.Lock()
	defer tb.agentSessionsMu.Unlock()

	// Look up existing session
	if as, exists := tb.agentSessions[chatID]; exists {
		return as
	}

	// Create new AgentSession
	channel := &TelegramChannel{
		bot:    tb.bot,
		chatID: chatID,
	}

	// Skip AgentSession creation if Project is not loaded
	if tb.Project == nil {
		return nil
	}

	as := NewAgentSession(
		tb.Project,
		tb.agentName,
		tb.chatMCPURL,
		tb.ChatStore,
		chatSessionID,
		channel,
		tb.confirmMgr,
	)

	tb.agentSessions[chatID] = as
	return as
}

// closeAgentSession closes and removes the AgentSession for the given chat ID.
func (tb *TelegramBot) closeAgentSession(chatID int64) error {
	tb.agentSessionsMu.Lock()
	defer tb.agentSessionsMu.Unlock()

	as, exists := tb.agentSessions[chatID]
	if !exists {
		return nil
	}

	delete(tb.agentSessions, chatID)
	return as.Close()
}

// ── Start / Stop ────────────────────────────────────────────────────────────

// Start begins bot polling in a goroutine.
func (tb *TelegramBot) Start(ctx context.Context) error {
	// Start bot polling — this blocks, so call in goroutine
	go tb.bot.Start(context.Background())
	slog.Info("chat: telegram bot started")
	return nil
}

// Stop shuts down the bot and closes all agent sessions.
func (tb *TelegramBot) Stop() error {
	// Close all agent sessions
	tb.agentSessionsMu.Lock()
	sessions := make([]*AgentSession, 0, len(tb.agentSessions))
	for _, as := range tb.agentSessions {
		sessions = append(sessions, as)
	}
	tb.agentSessions = make(map[int64]*AgentSession)
	tb.agentSessionsMu.Unlock()

	for _, as := range sessions {
		_ = as.Close()
	}

	// Stop bot
	_, _ = tb.bot.Close(context.Background())
	slog.Info("chat: telegram bot stopped")
	return nil
}

// ── Helper Functions ────────────────────────────────────────────────────────

// sendFormatted sends a message to the user, splitting long messages.
// ENVIA PARA USUARIO: formatted message (splits if > 4096 chars)
func (tb *TelegramBot) sendFormatted(ctx context.Context, chatID int64, text string) {
	chunks := splitMessage(text, telegramMaxMessageLength)
	for _, chunk := range chunks {
		_, _ = tb.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   chunk,
		})
	}
}

// splitMessage splits text into chunks of maxLen characters.
// Tries to split on natural boundaries (newlines) to avoid cutting words.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Try to split on last newline within maxLen
		splitAt := maxLen
		for i := maxLen - 1; i >= 0; i-- {
			if text[i] == '\n' {
				splitAt = i + 1
				break
			}
		}

		// If no newline found, hard cut at maxLen
		if splitAt == 0 {
			splitAt = maxLen
		}

		chunks = append(chunks, text[:splitAt])
		text = text[splitAt:]
	}

	return chunks
}

// formatUserFriendlyError converts an internal error into a user-friendly message.
func formatUserFriendlyError(err error) string {
	if err == nil {
		return "An internal error occurred."
	}

	msg := err.Error()

	// Strip technical details
	if strings.Contains(msg, "sqlite") || strings.Contains(msg, "database") {
		return "A database error occurred. Please try again."
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return "The request timed out. Please try again."
	}
	if strings.Contains(msg, "connection") || strings.Contains(msg, "network") {
		return "A network error occurred. Please check your connection and try again."
	}
	if strings.Contains(msg, "panic") || strings.Contains(msg, "stack") {
		return "An internal error occurred. Please try again."
	}

	// Return first 200 chars of error message
	if len(msg) > 200 {
		return msg[:200] + "..."
	}
	return msg
}
