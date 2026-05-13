package chat

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
)

// ChatService integrates the chat subsystem with Orqen's service lifecycle.
// It manages the chat memory store, session manager, confirmation manager,
// Telegram bot, and MCP HTTP server.
type ChatService struct {
	mu         sync.Mutex
	bot        *TelegramBot
	proj       *engine.Project
	config     *engine.Chat
	started    bool
	chatStore  *ChatStore
	sessionMgr *SessionManager
	confirmMgr ConfirmationManager
	mcpHandler http.Handler
	chatMCPURL string
}

var (
	chatsMu sync.Mutex
	chats   = map[string]*ChatService{}
)

func Get(projectId string) *ChatService {
	chatsMu.Lock()
	defer chatsMu.Unlock()
	return chats[projectId]
}

// New creates a new ChatService with the given project and configuration.
// Dependencies are NOT initialized here — they are deferred to OnStart().
func New(proj *engine.Project) *ChatService {
	chatsMu.Lock()
	defer chatsMu.Unlock()
	if proj, exists := chats[proj.Id]; exists {
		return proj
	}

	chat := &ChatService{
		proj:   proj,
		config: proj.Chat,
	}

	chats[proj.Id] = chat

	return chat
}

// Name returns the service name for logging and identification.
func (s *ChatService) Name() string { return "chat" }

// OnStart initializes all chat components and starts the Telegram bot.
func (s *ChatService) OnStart() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	// Validate agent configuration
	if err := s.validateAgent(); err != nil {
		return err
	}

	// Check if Telegram token is configured
	if s.config.Telegram.Token == "" {
		slog.Warn("Telegram token not configured — chat service running without Telegram bot.")
	}

	// Initialize ChatStore
	dbPath := filepath.Join(s.proj.DirAbs, ".orqen", "chat.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("chat: create .orqen directory: %w", err)
	}

	store, err := NewChatStore(dbPath)
	if err != nil {
		return fmt.Errorf("chat: initialize store: %w", err)
	}
	s.chatStore = store

	// Initialize SessionManager
	s.sessionMgr = NewSessionManager(s.chatStore, SessionTTL)

	// Initialize ConfirmationManager
	s.confirmMgr = NewConfirmationManager(s.chatStore, s.proj)

	// Build Chat MCP URL
	httpCfg := conf.GetHttpServer()
	port := 8080 // default
	if httpCfg != nil && httpCfg.Port != 0 {
		port = httpCfg.Port
	}
	s.chatMCPURL = fmt.Sprintf("http://127.0.0.1:%d/chat/mcp/%s", port, s.proj.Id)

	// Create Chat MCP Server
	s.mcpHandler = NewChatMCPServer(s.proj, s.chatStore, s.sessionMgr)

	// Initialize Telegram Bot (if token configured)
	if s.config.Telegram.Token != "" {
		bot, err := NewTelegramBot(
			s.config.Telegram.Token,
			s.proj,
			s.chatStore,
			s.sessionMgr,
			s.confirmMgr,
			s.chatMCPURL,
			s.config.Agent,
		)
		if err != nil {
			return fmt.Errorf("chat: initialize telegram bot: %w", err)
		}
		s.bot = bot

		// Start bot in goroutine
		if err := bot.Start(nil); err != nil {
			return fmt.Errorf("chat: start telegram bot: %w", err)
		}
	}

	s.started = true

	telegramStatus := "enabled"
	if s.config.Telegram.Token == "" {
		telegramStatus = "disabled"
	}
	slog.Info("Chat service started",
		"telegram", telegramStatus,
		"agent", s.config.Agent,
		"mcp", s.chatMCPURL,
	)

	return nil
}

// OnStop shuts down all chat components and cleans up resources.
func (s *ChatService) OnStop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	// Stop Telegram bot (if running)
	if s.bot != nil {
		if err := s.bot.Stop(); err != nil {
			slog.Error("chat: stop telegram bot", "error", err)
		}
	}

	// Cleanup expired edits
	if s.confirmMgr != nil {
		if cm, ok := s.confirmMgr.(*confirmationManager); ok {
			_, _ = cm.CleanupExpiredEdits()
		}
	}

	// Close chat store
	if s.chatStore != nil {
		if err := s.chatStore.Close(); err != nil {
			slog.Error("chat: close store", "error", err)
		}
	}

	s.started = false
	slog.Info("Chat service stopped")
	return nil
}

// GetMCPHandler returns the chat MCP HTTP handler for route registration.
func (s *ChatService) GetMCPHandler() http.Handler {
	return s.mcpHandler
}

// GetProjectID returns the project ID associated with this chat service.
func (s *ChatService) GetProjectID() string {
	return s.proj.Id
}

// validateAgent checks that the configured chat agent exists in the project's
// agent list. If config.Agent is empty, it falls back to the first agent.
func (s *ChatService) validateAgent() error {
	if s.config.Agent == "" {
		// Use first agent from project
		if s.proj.Agents.Clients != nil && len(s.proj.Agents.Clients) > 0 {
			// Get first key from map
			for name := range s.proj.Agents.Clients {
				s.config.Agent = name
				slog.Info("Chat agent not configured, using first available agent", "agent", name)
				break
			}
		}
		if s.config.Agent == "" {
			return fmt.Errorf("chat: no agents defined in project")
		}
		return nil
	}

	// Validate specified agent exists
	if _, exists := s.proj.Agents.Clients[s.config.Agent]; !exists {
		return fmt.Errorf("chat: agent %q not found in project agents", s.config.Agent)
	}

	slog.Info("Chat agent validated", "agent", s.config.Agent)
	return nil
}
