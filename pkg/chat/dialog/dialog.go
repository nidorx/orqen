package dialog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nidorx/orqen/pkg/chat/agent"
	"github.com/nidorx/orqen/pkg/chat/cmd"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

type Handler func(ctx context.Context, msg *Message)

// Connector is a generic I/O interface for delivering responses to users.
type Connector interface {
	Set(handler Handler)
	Start(ctx context.Context) error
	Stop()

	// Send delivers the final bot response to the user.
	Send(ctx context.Context, res *Response)

	// OnProgress delivers intermediate progress messages (optional).
	// Examples: "🔄 Thinking...", "🔧 Calling tool: read_file..."
	// May be a no-op for channels that don't support streaming.
	OnProgress(ctx context.Context, res *Response)
}

type Dialog struct {
	project    *engine.Project
	sessionMgr *memory.SessionManager
	confirmMgr memory.ConfirmationManager
	chatMCPURL string // URL to chat MCP server
	agentName  string // Agent to use for chat conversations

	connectors      []Connector
	agentSessions   map[string]*agent.Session // Agent sessions keyed by Telegram chat ID (one per user)
	agentSessionsMu sync.Mutex
}

func New(
	project *engine.Project,
	sessionMgr *memory.SessionManager,
	confirmMgr memory.ConfirmationManager,
	chatMCPURL string, // URL to chat MCP server
	agentName string, // Agent to use for chat conversations
) *Dialog {
	return &Dialog{
		project:       project,
		sessionMgr:    sessionMgr,
		confirmMgr:    confirmMgr,
		chatMCPURL:    chatMCPURL,
		agentName:     agentName,
		agentSessions: make(map[string]*agent.Session),
	}
}

func (d *Dialog) Register(connector Connector) {
	connector.Set(func(ctx context.Context, msg *Message) {
		if msg.Meta == nil {
			msg.Meta = make(map[string]any)
		}
		msg.Time = time.Now()
		msg.channel = &channel{connector: connector, msg: msg}
		d.handleMessage(ctx, msg)
	})
	d.connectors = append(d.connectors, connector)
}

// Start begins bot polling in a goroutine.
func (d *Dialog) Start(ctx context.Context) error {
	for _, connector := range d.connectors {
		if err := connector.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop shuts down the bot and closes all agent sessions.
func (d *Dialog) Stop() error {
	// Close all agent sessions
	d.agentSessionsMu.Lock()
	sessions := make([]*agent.Session, 0, len(d.agentSessions))
	for _, as := range d.agentSessions {
		sessions = append(sessions, as)
	}
	d.agentSessions = make(map[string]*agent.Session)
	d.agentSessionsMu.Unlock()

	for _, as := range sessions {
		_ = as.Close()
	}

	for _, connector := range d.connectors {
		connector.Stop()
	}

	return nil
}

// handleMessage is the main entry point for all incoming Telegram messages.
func (d *Dialog) handleMessage(ctx context.Context, msg *Message) {
	if msg == nil || msg.Text == "" {
		// Non-text message (photo, document, etc.)
		if msg != nil {
			msg.channel.Send(ctx, &Response{
				Message: msg,
				Text:    "I can only process text messages.",
			})
		}
		return
	}

	// Get or create chat session
	session, err := d.sessionMgr.GetOrCreateSession(msg.ChatID)
	if err != nil {
		slog.ErrorContext(ctx, "chat: get or create session", "error", err)
		msg.channel.Send(ctx, &Response{
			Text: "❌ An internal error occurred.",
		})
		return
	}
	msg.session = session

	// Route based on content
	if strings.HasPrefix(msg.Text, "/") {
		d.handleCommand(ctx, msg)
	} else {
		d.handleChatMessage(ctx, msg)
	}
}

// handleCommand routes a command to the appropriate handler.
func (d *Dialog) handleCommand(ctx context.Context, msg *Message) {
	// Parse command: strip leading '/', split on first space → (cmdName, args)
	cmdName, args, ok := cmd.Parse(msg.Text)
	if !ok {
		msg.channel.Send(ctx, &Response{
			Text: "❌ Invalid command format.",
		})
		return
	}

	// Look up command in registry
	command, found := cmd.Get(cmdName)
	if !found {
		msg.channel.Send(ctx, &Response{
			Text: fmt.Sprintf("Unknown command: /%s\nType /help for available commands.", cmdName),
		})
		return
	}

	text, err := command.Handler(ctx, &cmd.Request{
		ExtId:          msg.ChatID,
		Content:        args,
		Project:        d.project,
		SessionManager: d.sessionMgr,
	})
	if err != nil {
		slog.ErrorContext(ctx, "chat: command handler error", "command", cmdName, "error", err)
		// SEND TO USER: error message
		msg.channel.Send(ctx, &Response{
			Text: fmt.Sprintf("❌ Error executing /%s: %v", cmdName, err),
		})
		return
	}

	// SEND TO USER: response
	if text != "" {
		msg.channel.Send(ctx, &Response{
			Text: text,
		})
	}
}

// handleChatMessage routes a non-command message to the AI agent via AgentSession.
func (d *Dialog) handleChatMessage(ctx context.Context, msg *Message) {
	// Get or create AgentSession for this user
	as := d.getOrCreateAgentSession(msg.session.ID)

	// Save user message to store
	if err := d.sessionMgr.Store.AddMessage(msg.session.ID, memory.RoleUser, msg.Text); err != nil {
		slog.ErrorContext(ctx, "chat: save user message", "error", err)
	}

	// Call AgentSession.Prompt - internally Channel.Send(response) SEND TO USER
	response, err := as.Prompt(ctx, msg.Text, &agentChannel{channel: msg.channel})
	if err != nil {
		slog.ErrorContext(ctx, "chat: agent prompt error", "error", err)
		// SEND TO USER: user-friendly error message
		msg.channel.Send(ctx, &Response{
			Text: "❌ Error processing your request: " + formatUserFriendlyError(err),
		})
		return
	}

	// Response already sent via Channel.Send() inside Prompt()
	// Log for debugging
	if response != "" {
		slog.DebugContext(ctx, "chat: agent response delivered", "length", len(response))
	}
}

// getOrCreateAgentSession returns an existing AgentSession for the chat ID,
// or creates a new one.
func (d *Dialog) getOrCreateAgentSession(sessionID string) *agent.Session {
	// Skip AgentSession creation if Project is not loaded
	if d.project == nil {
		return nil
	}

	d.agentSessionsMu.Lock()
	defer d.agentSessionsMu.Unlock()

	// Look up existing session
	if as, exists := d.agentSessions[sessionID]; exists {
		return as
	}

	as := agent.NewAgentSession(
		d.project,
		d.agentName,
		d.chatMCPURL,
		d.sessionMgr.Store,
		sessionID,
		d.confirmMgr,
	)

	d.agentSessions[sessionID] = as
	return as
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
