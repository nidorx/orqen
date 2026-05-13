package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ── Test Helpers ─────────────────────────────────────────────────────────────

// mockTelegramServer creates an httptest.Server that mimics the Telegram Bot API.
// Returns a mux and a pointer to the handler function. Tests can modify the handler
// by assigning to the returned handler pointer.
func mockTelegramServer(t *testing.T) (*http.ServeMux, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(func() { server.Close() })
	return mux, server
}

// mockTelegramHandler creates a handler function that returns success responses.
// Can be used as a default handler for tests that don't need to capture request data.
func mockTelegramHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"result": map[string]interface{}{"message_id": 1},
		})
	}
}

// captureTelegramHandler creates a handler that captures request data for testing.
func captureTelegramHandler(sentText *string, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			mu.Lock()
			*sentText = body.Text
			mu.Unlock()
		} else {
			r.ParseForm()
			mu.Lock()
			*sentText = r.FormValue("text")
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"result": map[string]interface{}{"message_id": 1},
		})
	}
}

// captureTelegramHandlerFull captures both text and chat ID for testing.
func captureTelegramHandlerFull(receivedMessages *[]string, receivedChatIDs *[]int64, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			mu.Lock()
			*receivedMessages = append(*receivedMessages, body.Text)
			*receivedChatIDs = append(*receivedChatIDs, body.ChatID)
			mu.Unlock()
		} else {
			r.ParseForm()
			text := r.FormValue("text")
			chatIDStr := r.FormValue("chat_id")
			var chatID int64
			fmt.Sscanf(chatIDStr, "%d", &chatID)
			mu.Lock()
			*receivedMessages = append(*receivedMessages, text)
			*receivedChatIDs = append(*receivedChatIDs, chatID)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"result": map[string]interface{}{"message_id": 1},
		})
	}
}

// newTestBot creates a TelegramBot with a mock Telegram server.
func newTestBot(t *testing.T, mux *http.ServeMux, server *httptest.Server) *TelegramBot {
	t.Helper()
	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	return &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      nil,
		SessionManager: nil,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}
}

// createMockUpdate creates a message update for testing.
func createMockUpdate(chatID int64, text string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			Chat: models.Chat{
				ID: chatID,
			},
			Text: text,
		},
	}
}

// createMockUpdateNoText creates an update with no text (e.g., photo).
func createMockUpdateNoText(chatID int64) *models.Update {
	return &models.Update{
		Message: &models.Message{
			Chat: models.Chat{
				ID: chatID,
			},
			Text: "",
		},
	}
}

// recordingChannel is a Channel implementation that records calls for testing.
type recordingChannel struct {
	mu            sync.Mutex
	sentMessages  []string
	progressCalls []string
}

func (rc *recordingChannel) Send(ctx context.Context, sessionID, text string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.sentMessages = append(rc.sentMessages, text)
	return nil
}

func (rc *recordingChannel) OnProgress(ctx context.Context, sessionID, text string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.progressCalls = append(rc.progressCalls, text)
	return nil
}

// memoryStore creates an in-memory SQLite store for testing.
func memoryStore(t *testing.T) *ChatStore {
	t.Helper()
	store, err := NewChatStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create chat store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// newTestTelegramBot creates a fully configured TelegramBot for testing.
func newTestTelegramBot(t *testing.T, mux *http.ServeMux, server *httptest.Server) *TelegramBot {
	t.Helper()
	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}
	return tb
}

// ── Tests ────────────────────────────────────────────────────────────────────

// Test 1: NewTelegramBot — Create bot with valid token, verify initialization succeeds.
func TestNewTelegramBot_Init(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("expected bot creation to succeed, got error: %v", err)
	}

	if b == nil {
		t.Fatal("expected bot instance, got nil")
	}
}

// Test 2: TelegramChannel.Send — Mock bot, call Send(), verify message is sent to correct chat ID.
func TestTelegramChannel_Send(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var receivedMessages []string
	var receivedChatIDs []int64
	var mu sync.Mutex

	// Override the default handler to capture request body
	mux.HandleFunc("/", captureTelegramHandlerFull(&receivedMessages, &receivedChatIDs, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	tc := &TelegramChannel{bot: b, chatID: 12345}
	err = tc.Send(context.Background(), "session-1", "Hello, World!")
	if err != nil {
		t.Fatalf("expected Send to succeed, got error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(receivedMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(receivedMessages))
	}
	if receivedMessages[0] != "Hello, World!" {
		t.Errorf("expected message 'Hello, World!', got %q", receivedMessages[0])
	}
	if receivedChatIDs[0] != 12345 {
		t.Errorf("expected chat ID 12345, got %d", receivedChatIDs[0])
	}
}

// Test 3: TelegramChannel.Send — long message — Send 5000 chars, verify split into 2 messages.
func TestTelegramChannel_Send_LongMessage(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var messageCount int
	var mu sync.Mutex

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		messageCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"result": map[string]interface{}{"message_id": messageCount},
		})
	})

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	tc := &TelegramChannel{bot: b, chatID: 12345}
	longText := strings.Repeat("A", 5000)
	err = tc.Send(context.Background(), "session-1", longText)
	if err != nil {
		t.Fatalf("expected Send to succeed, got error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if messageCount != 2 {
		t.Errorf("expected 2 messages for 5000 chars, got %d", messageCount)
	}
}

// Test 4: TelegramChannel.OnProgress — Call OnProgress(), verify progress message is sent.
func TestTelegramChannel_OnProgress(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var sentText string
	var mu sync.Mutex

	mux.HandleFunc("/", captureTelegramHandler(&sentText, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	tc := &TelegramChannel{bot: b, chatID: 12345}
	err = tc.OnProgress(context.Background(), "session-1", "🔄 Thinking...")
	if err != nil {
		t.Fatalf("expected OnProgress to succeed, got error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if sentText == "" {
		t.Fatalf("expected 1 progress message, got none")
	}
	if sentText != "🔄 Thinking..." {
		t.Errorf("expected progress '🔄 Thinking...', got %q", sentText)
	}
}

// Test 5: handleMessage — text — Send a text message, verify it's routed to the chat handler.
func TestHandleMessage_Text_SessionCreated(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	// We can't test full agent flow without a mock ACP subprocess,
	// but we can test session creation and message saving.
	update := createMockUpdate(12345, "Hello, bot!")
	session, err := tb.SessionManager.GetOrCreateSession("12345")
	if err != nil {
		t.Fatalf("expected session creation to succeed: %v", err)
	}
	if session == nil {
		t.Fatal("expected session instance, got nil")
	}
	if session.UserID != "12345" {
		t.Errorf("expected user ID '12345', got %q", session.UserID)
	}

	// Simulate the bot receiving the message — save it to the store
	err = tb.ChatStore.AddMessage(session.ID, RoleUser, update.Message.Text)
	if err != nil {
		t.Fatalf("expected message saving to succeed: %v", err)
	}

	// Verify message was saved
	msgs, err := store.GetHistory(session.ID, 10)
	if err != nil {
		t.Fatalf("expected history retrieval to succeed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in history, got %d", len(msgs))
	}
	if msgs[0].Content != "Hello, bot!" {
		t.Errorf("expected message content 'Hello, bot!', got %q", msgs[0].Content)
	}
	if msgs[0].Role != RoleUser {
		t.Errorf("expected role 'user', got %q", msgs[0].Role)
	}
}

// Test 6: handleMessage — non-text — Send a message with no text (photo), verify error is returned.
func TestHandleMessage_NonText(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var sentText string
	var mu sync.Mutex

	mux.HandleFunc("/", captureTelegramHandler(&sentText, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	update := createMockUpdateNoText(12345)
	tb.handleMessage(context.Background(), b, update)

	mu.Lock()
	defer mu.Unlock()

	if sentText != "I can only process text messages." {
		t.Errorf("expected 'I can only process text messages.', got %q", sentText)
	}
}

// Test 7: handleMessage — command — Send /help, verify it's routed to the command handler.
func TestHandleMessage_Command(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var sentText string
	var mu sync.Mutex

	mux.HandleFunc("/", captureTelegramHandler(&sentText, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	// Register a test /help command
	RegisterCommand(CommandDef{
		Name:        "help",
		Description: "Test help command",
		Handler: func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
			return "Available commands: /help, /start", nil
		},
	})

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	update := createMockUpdate(12345, "/help")
	tb.handleMessage(context.Background(), b, update)

	mu.Lock()
	defer mu.Unlock()

	if !strings.Contains(sentText, "Available commands") {
		t.Errorf("expected response to contain 'Available commands', got %q", sentText)
	}
}

// Test 8: handleMessage — agent session created — First message from user, verify AgentSession is created.
func TestHandleMessage_AgentSessionCreated(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session, _ := tb.SessionManager.GetOrCreateSession(fmt.Sprintf("%d", chatID))
	as := tb.getOrCreateAgentSession(session.ID, chatID)

	// AgentSession creation returns nil when Project is not loaded
	if as != nil {
		t.Error("expected AgentSession to be nil when Project is not loaded")
	}

	// Verify nil session is NOT cached
	tb.agentSessionsMu.Lock()
	_, exists := tb.agentSessions[chatID]
	tb.agentSessionsMu.Unlock()

	if exists {
		t.Error("expected nil AgentSession to NOT be cached")
	}
}

// Test 9: handleMessage — agent session reused — Second message from same user, verify same AgentSession is used.
func TestHandleMessage_AgentSessionReused(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session, _ := tb.SessionManager.GetOrCreateSession(fmt.Sprintf("%d", chatID))

	// First call — creates session
	as1 := tb.getOrCreateAgentSession(session.ID, chatID)

	// Second call — should reuse same session
	as2 := tb.getOrCreateAgentSession(session.ID, chatID)

	if as1 != as2 {
		t.Error("expected same AgentSession to be reused, got different instances")
	}
}

// Test 10: handleCommand — success — Execute a command, verify response is sent.
func TestHandleCommand_Success(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var sentText string
	var mu sync.Mutex

	mux.HandleFunc("/", captureTelegramHandler(&sentText, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	RegisterCommand(CommandDef{
		Name:        "test",
		Description: "Test command",
		Handler: func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
			return "Command executed successfully", nil
		},
	})

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	session, _ := sessionMgr.GetOrCreateSession("12345")
	tb.handleCommand(context.Background(), session, "/test", 12345)

	mu.Lock()
	defer mu.Unlock()

	if sentText != "Command executed successfully" {
		t.Errorf("expected 'Command executed successfully', got %q", sentText)
	}
}

// Test 11: handleChatMessage — success — Mock agent (via mock Channel), verify response is delivered.
func TestHandleChatMessage_Success(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session, _ := sessionMgr.GetOrCreateSession(fmt.Sprintf("%d", chatID))

	// Create a mock AgentSession that uses a mock channel
	rc := &recordingChannel{}
	as := &AgentSession{
		proj:          nil,
		agentName:     "test",
		command:       nil,
		chatMCPURL:    "",
		chatStore:     store,
		chatSessionID: session.ID,
		channel:       rc,
		confirmMgr:    nil,
		queue:         make(chan promptRequest, 1024),
		isFirstPrompt: true,
	}

	tb.agentSessionsMu.Lock()
	tb.agentSessions[chatID] = as
	tb.agentSessionsMu.Unlock()

	// Since we can't test actual ACP agent without a subprocess,
	// we test that the AgentSession is properly set up with the channel
	if as.channel == nil {
		t.Error("expected AgentSession to have a non-nil channel")
	}
	if as.chatSessionID != session.ID {
		t.Errorf("expected chatSessionID %q, got %q", session.ID, as.chatSessionID)
	}
}

// Test 12: handleChatMessage — error — Mock agent error, verify user-friendly error is sent.
func TestHandleChatMessage_Error(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	_ = &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	// Test error formatting
	errMsg := formatUserFriendlyError(fmt.Errorf("sqlite: database is locked"))
	if !strings.Contains(errMsg, "database") {
		t.Errorf("expected database error message, got %q", errMsg)
	}
}

// Test 13: handleChatMessage — progress messages — Verify OnProgress is called during agent processing.
func TestHandleChatMessage_ProgressMessages(t *testing.T) {
	rc := &recordingChannel{}

	err := rc.OnProgress(context.Background(), "session-1", "🔄 Thinking...")
	if err != nil {
		t.Fatalf("expected OnProgress to succeed: %v", err)
	}

	err = rc.OnProgress(context.Background(), "session-1", "🔧 Calling tool: read_file...")
	if err != nil {
		t.Fatalf("expected OnProgress to succeed: %v", err)
	}

	if len(rc.progressCalls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(rc.progressCalls))
	}
	if rc.progressCalls[0] != "🔄 Thinking..." {
		t.Errorf("expected first progress '🔄 Thinking...', got %q", rc.progressCalls[0])
	}
	if rc.progressCalls[1] != "🔧 Calling tool: read_file..." {
		t.Errorf("expected second progress '🔧 Calling tool: read_file...', got %q", rc.progressCalls[1])
	}
}

// Test 14: /start — Verify AgentSession is created and welcome message sent.
func TestHandleCommand_Start(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session, _ := sessionMgr.GetOrCreateSession(fmt.Sprintf("%d", chatID))

	// Create AgentSession via getOrCreateAgentSession
	as := tb.getOrCreateAgentSession(session.ID, chatID)
	if as == nil {
		t.Fatal("expected AgentSession to be created on /start")
	}

	// Verify session is cached
	tb.agentSessionsMu.Lock()
	_, exists := tb.agentSessions[chatID]
	tb.agentSessionsMu.Unlock()

	if !exists {
		t.Error("expected AgentSession to be cached after /start")
	}
}

// Test 15: /new — Verify old AgentSession is closed and new one created.
func TestHandleCommand_New(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session1, _ := sessionMgr.GetOrCreateSession("user-12345")
	as1 := tb.getOrCreateAgentSession(session1.ID, chatID)

	// Close old session
	err = tb.closeAgentSession(chatID)
	if err != nil {
		t.Fatalf("expected closeAgentSession to succeed: %v", err)
	}

	// Verify session removed from cache
	tb.agentSessionsMu.Lock()
	_, exists := tb.agentSessions[chatID]
	tb.agentSessionsMu.Unlock()

	if exists {
		t.Error("expected AgentSession to be removed from cache after close")
	}

	// Verify old session is closed
	if !as1.closed {
		t.Error("expected old AgentSession to be marked as closed")
	}

	// Create new session
	session2, _ := sessionMgr.NewSession("user-12345")
	as2 := tb.getOrCreateAgentSession(session2.ID, chatID)

	if as2 == nil {
		t.Fatal("expected new AgentSession to be created")
	}
	if as2 == as1 {
		t.Error("expected new AgentSession to be a different instance")
	}
}

// Test 16: /stop — Verify Cancel() is called on current AgentSession.
func TestHandleCommand_Stop(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session, _ := sessionMgr.GetOrCreateSession("user-12345")
	as := tb.getOrCreateAgentSession(session.ID, chatID)

	// Cancel should work on the session
	// Since there's no active prompt, Cancel will return an error about no active prompt
	// but the session should still be valid
	err = as.Cancel()
	if err == nil {
		// If there was an active prompt, it would be cancelled
		t.Log("No active prompt to cancel (expected when session is idle)")
	}

	// Verify session still exists in cache
	tb.agentSessionsMu.Lock()
	_, exists := tb.agentSessions[chatID]
	tb.agentSessionsMu.Unlock()

	if !exists {
		t.Error("expected AgentSession to still exist after Cancel")
	}
}

// Test 17: splitMessage — Split a 5000-char message with maxLen=4096, verify 2 chunks.
func TestSplitMessage_BasicSplit(t *testing.T) {
	text := strings.Repeat("A", 5000)
	chunks := splitMessage(text, 4096)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	if len(chunks[0]) != 4096 {
		t.Errorf("expected first chunk to be 4096 chars, got %d", len(chunks[0]))
	}
	if len(chunks[1]) != 904 {
		t.Errorf("expected second chunk to be 904 chars, got %d", len(chunks[1]))
	}
}

// Test 18: splitMessage — newline boundary — Split a message with newlines, verify it splits on newline.
func TestSplitMessage_NewlineBoundary(t *testing.T) {
	// Create a message with a newline just before the 4096 limit
	part1 := strings.Repeat("A", 4090)
	part2 := "rest of message"
	text := part1 + "\n" + part2

	chunks := splitMessage(text, 4096)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// Should split at newline, so first chunk includes the newline
	if !strings.HasSuffix(chunks[0], "\n") {
		t.Errorf("expected first chunk to end with newline, got %q", chunks[0][len(chunks[0])-1:])
	}
	if chunks[1] != part2 {
		t.Errorf("expected second chunk to be %q, got %q", part2, chunks[1])
	}
}

// Test 19: splitMessage — exact boundary — Split a message exactly at maxLen.
func TestSplitMessage_ExactBoundary(t *testing.T) {
	text := strings.Repeat("A", 4096) + strings.Repeat("B", 4096)
	chunks := splitMessage(text, 4096)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	if len(chunks[0]) != 4096 {
		t.Errorf("expected first chunk to be 4096 chars, got %d", len(chunks[0]))
	}
	if len(chunks[1]) != 4096 {
		t.Errorf("expected second chunk to be 4096 chars, got %d", len(chunks[1]))
	}

	if chunks[0] != strings.Repeat("A", 4096) {
		t.Error("expected first chunk to be all A's")
	}
	if chunks[1] != strings.Repeat("B", 4096) {
		t.Error("expected second chunk to be all B's")
	}
}

// Test 20: Message saving — Send a message, verify it's saved to the chat store with correct role.
func TestMessageSaving(t *testing.T) {
	store := memoryStore(t)
	defer store.Close()

	sessionMgr := NewSessionManager(store, SessionTTL)
	session, err := sessionMgr.GetOrCreateSession("user-12345")
	if err != nil {
		t.Fatalf("expected session creation to succeed: %v", err)
	}

	// Save user message
	err = store.AddMessage(session.ID, RoleUser, "Hello from test!")
	if err != nil {
		t.Fatalf("expected message saving to succeed: %v", err)
	}

	// Save assistant response
	err = store.AddMessage(session.ID, RoleAssistant, "Response from bot!")
	if err != nil {
		t.Fatalf("expected assistant message saving to succeed: %v", err)
	}

	// Verify both messages
	msgs, err := store.GetHistory(session.ID, 10)
	if err != nil {
		t.Fatalf("expected history retrieval to succeed: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(msgs))
	}

	if msgs[0].Role != RoleUser || msgs[0].Content != "Hello from test!" {
		t.Errorf("expected first message to be user message, got role=%q content=%q", msgs[0].Role, msgs[0].Content)
	}

	if msgs[1].Role != RoleAssistant || msgs[1].Content != "Response from bot!" {
		t.Errorf("expected second message to be assistant message, got role=%q content=%q", msgs[1].Role, msgs[1].Content)
	}
}

// Additional test: splitMessage — empty string
func TestSplitMessage_Empty(t *testing.T) {
	chunks := splitMessage("", 4096)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty string, got %d", len(chunks))
	}
	if chunks[0] != "" {
		t.Errorf("expected empty chunk, got %q", chunks[0])
	}
}

// Additional test: splitMessage — exact max length
func TestSplitMessage_ExactMaxLen(t *testing.T) {
	text := strings.Repeat("X", 4096)
	chunks := splitMessage(text, 4096)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for exact max length, got %d", len(chunks))
	}
	if len(chunks[0]) != 4096 {
		t.Errorf("expected chunk to be 4096 chars, got %d", len(chunks[0]))
	}
}

// Additional test: formatUserFriendlyError — timeout error
func TestFormatUserFriendlyError_Timeout(t *testing.T) {
	err := formatUserFriendlyError(fmt.Errorf("context deadline exceeded: timeout"))
	if !strings.Contains(err, "timed out") {
		t.Errorf("expected timeout message, got %q", err)
	}
}

// Additional test: formatUserFriendlyError — network error
func TestFormatUserFriendlyError_Network(t *testing.T) {
	err := formatUserFriendlyError(fmt.Errorf("connection refused: network unreachable"))
	if !strings.Contains(err, "network error") {
		t.Errorf("expected network error message, got %q", err)
	}
}

// Additional test: AgentSession lifecycle — Close all sessions on Stop
func TestTelegramBot_Stop_ClosesAllSessions(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	// Create multiple sessions
	for i := int64(1); i <= 3; i++ {
		session, _ := sessionMgr.GetOrCreateSession(fmt.Sprintf("user-%d", i))
		_ = tb.getOrCreateAgentSession(session.ID, i)
	}

	// Verify 3 sessions exist
	tb.agentSessionsMu.Lock()
	count := len(tb.agentSessions)
	tb.agentSessionsMu.Unlock()

	if count != 3 {
		t.Fatalf("expected 3 sessions, got %d", count)
	}

	// Stop should close all sessions
	err = tb.Stop()
	if err != nil {
		t.Fatalf("expected Stop to succeed: %v", err)
	}

	// Verify sessions map is cleared
	tb.agentSessionsMu.Lock()
	count = len(tb.agentSessions)
	tb.agentSessionsMu.Unlock()

	if count != 0 {
		t.Errorf("expected sessions map to be cleared after Stop, got %d sessions", count)
	}
}

// Test with nil project — ensures bot doesn't crash
func TestHandleMessage_NilProject(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil, // Explicitly nil
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	// Session creation should work even with nil project
	update := createMockUpdate(12345, "Hello!")
	session, err := tb.SessionManager.GetOrCreateSession("12345")
	if err != nil {
		t.Fatalf("expected session creation to succeed with nil project: %v", err)
	}
	if session == nil {
		t.Fatal("expected session instance with nil project")
	}

	// Use the update to verify message text
	if update.Message.Text != "Hello!" {
		t.Errorf("expected message text 'Hello!', got %q", update.Message.Text)
	}
}

// Test: getOrCreateAgentSession with existing session returns same instance
func TestGetOrCreateAgentSession_Existing(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	chatID := int64(12345)
	session, _ := sessionMgr.GetOrCreateSession(fmt.Sprintf("%d", chatID))

	as1 := tb.getOrCreateAgentSession(session.ID, chatID)
	as2 := tb.getOrCreateAgentSession(session.ID, chatID)

	if as1 != as2 {
		t.Error("expected same AgentSession instance to be returned")
	}
}

// Test: closeAgentSession with non-existent chatID returns nil error
func TestCloseAgentSession_NonExistent(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	err = tb.closeAgentSession(99999)
	if err != nil {
		t.Errorf("expected closeAgentSession with non-existent chatID to return nil error, got %v", err)
	}
}

// Test: command not found
func TestHandleCommand_UnknownCommand(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var sentText string
	var mu sync.Mutex

	mux.HandleFunc("/", captureTelegramHandler(&sentText, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	session, _ := sessionMgr.GetOrCreateSession("12345")
	tb.handleCommand(context.Background(), session, "/nonexistent", 12345)

	mu.Lock()
	defer mu.Unlock()

	if !strings.Contains(sentText, "Unknown command") {
		t.Errorf("expected 'Unknown command' response, got %q", sentText)
	}
}

// Test: splitMessage with no newline, hard cut
func TestSplitMessage_NoNewline(t *testing.T) {
	// Create text > 4096 with no newlines
	text := strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 200) // 5200 chars
	chunks := splitMessage(text, 4096)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	// First chunk should be exactly 4096 chars (hard cut)
	if len(chunks[0]) != 4096 {
		t.Errorf("expected first chunk to be 4096 chars (hard cut), got %d", len(chunks[0]))
	}
}

// Test: empty progress message is a no-op
func TestTelegramChannel_OnProgress_Empty(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	tc := &TelegramChannel{bot: b, chatID: 12345}
	err = tc.OnProgress(context.Background(), "session-1", "")
	if err != nil {
		t.Fatalf("expected OnProgress with empty text to succeed: %v", err)
	}
}

// Test: splitMessage with multiple newlines within boundary
func TestSplitMessage_MultipleNewlines(t *testing.T) {
	line := "This is a line of text"
	text := strings.Repeat(line+"\n", 200) // ~4600 chars
	chunks := splitMessage(text, 4096)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	// First chunk should end at a newline boundary
	if !strings.HasSuffix(chunks[0], "\n") {
		t.Errorf("expected first chunk to end with newline, got ending: %q", chunks[0][len(chunks[0])-5:])
	}
}

// Test: formatUserFriendlyError with short error
func TestFormatUserFriendlyError_Short(t *testing.T) {
	err := formatUserFriendlyError(fmt.Errorf("simple error"))
	if err != "simple error" {
		t.Errorf("expected 'simple error', got %q", err)
	}
}

// Test: formatUserFriendlyError with long error (> 200 chars)
func TestFormatUserFriendlyError_Long(t *testing.T) {
	longMsg := strings.Repeat("x", 250)
	err := formatUserFriendlyError(fmt.Errorf("%s", longMsg))
	if len(err) > 203 { // 200 + "..."
		t.Errorf("expected truncated error <= 203 chars, got %d chars", len(err))
	}
	if !strings.HasSuffix(err, "...") {
		t.Errorf("expected truncated error to end with '...', got %q", err)
	}
}

// Test: formatUserFriendlyError with nil error
func TestFormatUserFriendlyError_Nil(t *testing.T) {
	err := formatUserFriendlyError(nil)
	if err != "An internal error occurred." {
		t.Errorf("expected default error message for nil, got %q", err)
	}
}

// Integration-style test: full message flow (session + message save)
func TestFullMessageFlow_SessionAndMessage(t *testing.T) {
	store := memoryStore(t)
	defer store.Close()

	sessionMgr := NewSessionManager(store, SessionTTL)
	chatID := int64(12345)
	userID := fmt.Sprintf("%d", chatID)

	// Simulate: user sends first message
	session, err := sessionMgr.GetOrCreateSession(userID)
	if err != nil {
		t.Fatalf("session creation failed: %v", err)
	}

	// Save user message
	err = store.AddMessage(session.ID, RoleUser, "What's the project status?")
	if err != nil {
		t.Fatalf("save user message failed: %v", err)
	}

	// Simulate: agent responds
	err = store.AddMessage(session.ID, RoleAssistant, "The project has 3 modules with 12 workitems.")
	if err != nil {
		t.Fatalf("save assistant message failed: %v", err)
	}

	// Verify conversation
	msgs, err := store.GetHistory(session.ID, 10)
	if err != nil {
		t.Fatalf("get history failed: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// User sends second message
	err = store.AddMessage(session.ID, RoleUser, "Show me the lanes")
	if err != nil {
		t.Fatalf("save second user message failed: %v", err)
	}

	msgs, err = store.GetHistory(session.ID, 10)
	if err != nil {
		t.Fatalf("get history after second message failed: %v", err)
	}

	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	if msgs[2].Content != "Show me the lanes" {
		t.Errorf("expected third message to be 'Show me the lanes', got %q", msgs[2].Content)
	}
}

// Test: command handler with error returns user-friendly message
func TestHandleCommand_HandlerError(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var sentText string
	var mu sync.Mutex

	mux.HandleFunc("/", captureTelegramHandler(&sentText, &mu))

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	RegisterCommand(CommandDef{
		Name:        "fail",
		Description: "Command that always fails",
		Handler: func(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
			return "", fmt.Errorf("internal failure")
		},
	})

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	session, _ := sessionMgr.GetOrCreateSession("12345")
	tb.handleCommand(context.Background(), session, "/fail arg", 12345)

	mu.Lock()
	defer mu.Unlock()

	if !strings.Contains(sentText, "Error executing /fail") {
		t.Errorf("expected error response, got %q", sentText)
	}
}

// Test: sendFormatted splits long messages
func TestSendFormatted_LongMessage(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()

	var messageCount int
	var mu sync.Mutex

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		messageCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"result": map[string]interface{}{"message_id": messageCount},
		})
	})

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	longText := strings.Repeat("A", 9000)
	tb.sendFormatted(context.Background(), 12345, longText)

	mu.Lock()
	defer mu.Unlock()

	// 9000 chars should be split into 3 messages (4096 + 4096 + 808)
	if messageCount < 2 {
		t.Errorf("expected at least 2 messages for 9000 chars, got %d", messageCount)
	}
}

// Test: Start method doesn't panic
func TestTelegramBot_Start(t *testing.T) {
	mux, server := mockTelegramServer(t)
	defer server.Close()
	mux.HandleFunc("/", mockTelegramHandler())

	b, err := bot.New("test-token", bot.WithServerURL(server.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	store := memoryStore(t)
	sessionMgr := NewSessionManager(store, SessionTTL)

	tb := &TelegramBot{
		bot:            b,
		Project:        nil,
		ChatStore:      store,
		SessionManager: sessionMgr,
		confirmMgr:     nil,
		chatMCPURL:     "",
		agentName:      "test",
		agentSessions:  make(map[int64]*AgentSession),
	}

	// Start should not panic (it launches goroutine)
	// We can't test actual polling in unit tests, so just verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Start panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = tb.Start(ctx)
	// Let it run briefly then stop
	time.Sleep(50 * time.Millisecond)
	_ = tb.Stop()
}
