package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/utils"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	defaultAgentTimeout   = 5 * time.Minute
	subprocessIdleTimeout = 5 * time.Minute
)

// ── Channel Interface ────────────────────────────────────────────────────────

// Channel is a generic I/O interface for delivering agent responses to users.
// The agent knows nothing about channels (Telegram, CLI, Web) — it just calls
// Send/OnProgress.
type Channel interface {
	// Send delivers the final agent response to the user.
	Send(ctx context.Context, sessionID, text string) error

	// OnProgress delivers intermediate progress messages (optional).
	// Examples: "🔄 Thinking...", "🔧 Calling tool: read_file..."
	// May be a no-op for channels that don't support streaming.
	OnProgress(ctx context.Context, sessionID, text string) error
}

// NoOpChannel is a channel that discards all output. Used when no real channel is available.
type NoOpChannel struct{}

func (n *NoOpChannel) Send(ctx context.Context, sessionID, text string) error       { return nil }
func (n *NoOpChannel) OnProgress(ctx context.Context, sessionID, text string) error { return nil }

// ── Confirmation Manager Interface (forward declaration for Task 08) ─────────

// ConfirmationManager defines the interface for handling pending file edits.
// Implemented in confirmation.go (Task 08).
type ConfirmationManager interface {
	HasPendingEdit(sessionID string) bool
	ApplyEdit(sessionID string) error
	RejectEdit(sessionID string) error
}

// ── Prompt Queue Types ───────────────────────────────────────────────────────

type promptRequest struct {
	prompt string
	result chan promptResult
}

type promptResult struct {
	response string
	err      error
}

// ── AgentSession — Persistent ACP Session ────────────────────────────────────

// AgentSession represents one ACP conversation session within a chat session.
// It maintains a prompt queue (FIFO) and delegates I/O to a Channel.
type AgentSession struct {
	mu            sync.Mutex
	proj          *engine.Project
	agentName     string
	command       []string
	chatMCPURL    string
	conn          *acp.ClientSideConnection // nil = subprocess not started
	acpSessionID  string                    // empty = no ACP session created yet
	chatStore     *ChatStore
	chatSessionID string // chat session ID (SQLite)
	channel       Channel
	confirmMgr    ConfirmationManager

	// Prompt queue
	queue         chan promptRequest
	currentCancel context.CancelFunc // cancel for the prompt currently running
	currentCtx    context.Context    // context of the current prompt
	closed        bool
	wg            sync.WaitGroup // waits for queue worker to finish

	// System prompt tracking
	isFirstPrompt bool // true = next prompt should include system instructions
}

// NewAgentSession creates a new agent session. Does NOT start the subprocess
// (lazy start on first Prompt call).
func NewAgentSession(
	proj *engine.Project,
	agentName string,
	chatMCPURL string,
	chatStore *ChatStore,
	chatSessionID string,
	channel Channel,
	confirmMgr ConfirmationManager,
) *AgentSession {

	resolvedAgentName := proj.Agents.GetName(agentName)
	command := proj.Agents.GetCommand(resolvedAgentName)

	as := &AgentSession{
		proj:          proj,
		agentName:     resolvedAgentName,
		command:       command,
		chatMCPURL:    chatMCPURL,
		chatStore:     chatStore,
		chatSessionID: chatSessionID,
		channel:       channel,
		confirmMgr:    confirmMgr,
		queue:         make(chan promptRequest, 1024), // buffered to prevent sender blocking
		isFirstPrompt: true,
	}

	as.wg.Add(1)
	go as.queueWorker()

	return as
}

// Prompt sends a user message to the agent and blocks until the response is ready.
// If there is a pending edit for the session, it checks for approval/rejection first.
func (as *AgentSession) Prompt(ctx context.Context, text string) (string, error) {
	as.mu.Lock()
	if as.closed {
		as.mu.Unlock()
		return "", fmt.Errorf("agent session is closed")
	}
	confirmMgr := as.confirmMgr
	chatSessionID := as.chatSessionID
	channel := as.channel
	as.mu.Unlock()

	// Check confirmation intercept first
	if confirmMgr != nil && confirmMgr.HasPendingEdit(chatSessionID) {
		normalized := strings.ToLower(strings.TrimSpace(text))
		if isApprovalKeyword(normalized) {
			if err := confirmMgr.ApplyEdit(chatSessionID); err != nil {
				_ = channel.Send(ctx, chatSessionID, fmt.Sprintf("❌ Failed to apply edit: %v", err))
				return "", fmt.Errorf("apply edit: %w", err)
			}
			msg := "✅ Edit applied successfully."
			_ = channel.Send(ctx, chatSessionID, msg)
			return msg, nil
		}
		if isRejectionKeyword(normalized) {
			if err := confirmMgr.RejectEdit(chatSessionID); err != nil {
				_ = channel.Send(ctx, chatSessionID, fmt.Sprintf("❌ Failed to reject edit: %v", err))
				return "", fmt.Errorf("reject edit: %w", err)
			}
			msg := "❌ Edit discarded."
			_ = channel.Send(ctx, chatSessionID, msg)
			return msg, nil
		}
		// Not approval/rejection — fall through to normal prompt
	}

	// Enqueue prompt
	resultCh := make(chan promptResult, 1)
	select {
	case as.queue <- promptRequest{prompt: text, result: resultCh}:
		// Enqueued successfully
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Wait for result
	select {
	case result := <-resultCh:
		return result.response, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Cancel interrupts the current prompt turn via ACP session/cancel.
func (as *AgentSession) Cancel() error {
	as.mu.Lock()
	cancelFn := as.currentCancel
	currentCtx := as.currentCtx
	as.mu.Unlock()

	if cancelFn == nil {
		return fmt.Errorf("no active prompt to cancel")
	}

	// Send ACP cancel
	if as.conn != nil && as.acpSessionID != "" {
		_ = as.conn.Cancel(context.Background(), acp.CancelNotification{
			SessionId: acp.SessionId(as.acpSessionID),
		})
	}

	// Cancel the local context
	cancelFn()

	// Wait briefly for the prompt goroutine to finish
	<-currentCtx.Done()
	return nil
}

// Close closes the agent session, waits for the queue to drain, and schedules
// subprocess idle timeout.
func (as *AgentSession) Close() error {
	as.mu.Lock()
	if as.closed {
		as.mu.Unlock()
		return nil
	}
	as.closed = true
	close(as.queue)
	as.mu.Unlock()

	// Wait for queue worker to finish
	as.wg.Wait()

	// Close ACP session if exists
	as.mu.Lock()
	conn := as.conn
	acpSID := as.acpSessionID
	as.mu.Unlock()

	if conn != nil && acpSID != "" {
		_, _ = conn.UnstableCloseSession(context.Background(), acp.UnstableCloseSessionRequest{
			SessionId: acp.SessionId(acpSID),
		})
	}

	// Schedule subprocess idle timeout
	if len(as.command) > 0 {
		scheduleIdle(as.proj.Id, as.agentName, subprocessIdleTimeout)
	}

	return nil
}

// queueWorker processes prompts from the queue in FIFO order.
func (as *AgentSession) queueWorker() {
	defer as.wg.Done()

	for req := range as.queue {
		as.executePrompt(req)
	}
}

// executePrompt runs a single prompt turn. Called by queueWorker sequentially.
func (as *AgentSession) executePrompt(req promptRequest) {
	as.mu.Lock()
	if as.closed {
		as.mu.Unlock()
		req.result <- promptResult{err: fmt.Errorf("agent session is closed")}
		return
	}
	as.mu.Unlock()

	// Ensure subprocess is running
	process, err := getOrCreateProcess(as.proj.Id, as.agentName, as.command)
	if err != nil {
		req.result <- promptResult{err: fmt.Errorf("get process: %w", err)}
		return
	}

	// Ensure ACP session exists
	if err := as.ensureSession(process); err != nil {
		req.result <- promptResult{err: fmt.Errorf("ensure session: %w", err)}
		return
	}

	// Build the prompt
	as.mu.Lock()
	fullPrompt := req.prompt
	if as.isFirstPrompt {
		fullPrompt = fmt.Sprintf("%s\n\n<message>%s</message>", buildSystemPrompt(as.proj), req.prompt)
		as.isFirstPrompt = false
	}
	acpSID := as.acpSessionID
	conn := as.conn
	chatStore := as.chatStore
	chatSessionID := as.chatSessionID
	channel := as.channel
	as.mu.Unlock()

	// Create cancellable context for this prompt
	promptCtx, cancel := context.WithTimeout(context.Background(), defaultAgentTimeout)

	as.mu.Lock()
	as.currentCancel = cancel
	as.currentCtx = promptCtx
	as.mu.Unlock()

	// Create a client session for streaming with OnProgress callback
	clientSession := newChatClientSession(channel, chatSessionID, promptCtx)
	chatSessionSet(acp.SessionId(acpSID), clientSession)

	// Send prompt
	_, err = conn.Prompt(promptCtx, acp.PromptRequest{
		SessionId: acp.SessionId(acpSID),
		Prompt:    []acp.ContentBlock{acp.TextBlock(fullPrompt)},
	})

	// Clean up
	chatSessionDel(acp.SessionId(acpSID))

	as.mu.Lock()
	as.currentCancel = nil
	as.currentCtx = nil
	as.mu.Unlock()

	if err != nil {
		// Check if it was a cancellation
		if promptCtx.Err() == context.Canceled {
			req.result <- promptResult{response: "⚠️ Request cancelled.", err: nil}
			return
		}
		req.result <- promptResult{err: formatACPError(as.agentName, "prompt", err)}
		return
	}

	// Extract response
	response := clientSession.getResponse()
	if response == "" {
		response = "(agent completed but no response text was captured)"
	}

	// Save messages to chat store
	_ = chatStore.AddMessage(chatSessionID, RoleUser, req.prompt)
	_ = chatStore.AddMessage(chatSessionID, RoleAssistant, response)

	// Send final response via channel
	_ = channel.Send(context.Background(), chatSessionID, response)

	req.result <- promptResult{response: response, err: nil}
}

// ensureSession creates an ACP session if one doesn't exist yet.
func (as *AgentSession) ensureSession(process *agentProcess) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.acpSessionID != "" {
		return nil // already have a session
	}

	mcpServers := []acp.McpServer{
		{
			Http: &acp.McpServerHttpInline{
				Name: "Orqen Chat MCP",
				Url:  as.chatMCPURL,
			},
		},
	}

	sess, err := process.conn.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        as.proj.DirAbs,
		McpServers: mcpServers,
	})
	if err != nil {
		return formatACPError(as.agentName, "new session", err)
	}

	as.acpSessionID = string(sess.SessionId)
	as.conn = process.conn
	return nil
}

// ── Subprocess Manager (Global) ─────────────────────────────────────────────

var (
	agents     = map[string]*agentProcess{}
	agentsMu   sync.Mutex
	idleTimers = map[string]*time.Timer{}
)

type agentProcess struct {
	cmd    *exec.Cmd
	conn   *acp.ClientSideConnection
	client *chatACPClient
}

// getOrCreateProcess returns a cached agent subprocess or spawns a new one.
func getOrCreateProcess(projectID, agentName string, command []string) (*agentProcess, error) {
	agentID := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", agentName, projectID)))

	agentsMu.Lock()
	defer agentsMu.Unlock()

	// Cancel idle timer if running
	if timer, exists := idleTimers[agentID]; exists {
		timer.Stop()
		delete(idleTimers, agentID)
	}

	proc, exists := agents[agentID]
	if exists {
		return proc, nil
	}

	// Spawn new subprocess
	return startProcess(agentID, agentName, command)
}

// startProcess spawns a new ACP agent subprocess.
// Must be called with agentsMu held.
func startProcess(agentID, agentName string, command []string) (*agentProcess, error) {
	ctx := context.Background()
	logger := chatLogger(fmt.Sprintf("[%s]", agentName))

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Log("stdin pipe error: %v\n", err)
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Log("stdout pipe error: %v\n", err)
		return nil, fmt.Errorf("[%s] stdout pipe error: %w", agentName, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[%s] failed to start acp agent: %w", agentName, err)
	}

	client := &chatACPClient{
		logger:    logger,
		terminals: newChatTerminalManager(),
	}
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	conn.SetLogger(slog.Default())

	// Initialize
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
	})
	if err != nil {
		_ = cmd.Process.Kill()
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				return nil, fmt.Errorf("[%s] initialize error: %s", agentName, string(b))
			}
			return nil, fmt.Errorf("[%s] initialize error (%d): %s", agentName, re.Code, re.Message)
		}
		return nil, fmt.Errorf("[%s] initialize error: %w", agentName, err)
	}
	logger.Log("connected (protocol v%v)\n", initResp.ProtocolVersion)

	proc := &agentProcess{
		cmd:    cmd,
		conn:   conn,
		client: client,
	}

	agents[agentID] = proc
	return proc, nil
}

// scheduleIdle sets a timer to kill the subprocess after the idle timeout.
func scheduleIdle(projectID, agentName string, timeout time.Duration) {
	agentID := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", agentName, projectID)))

	agentsMu.Lock()
	defer agentsMu.Unlock()

	// Don't schedule if there are active sessions
	if _, exists := agents[agentID]; exists {
		timer := time.AfterFunc(timeout, func() {
			agentsMu.Lock()
			defer agentsMu.Unlock()
			// Double-check: maybe a new session started while waiting
			if p, stillExists := agents[agentID]; stillExists {
				if p.cmd != nil && p.cmd.Process != nil {
					_ = p.cmd.Process.Kill()
				}
				delete(agents, agentID)
			}
			delete(idleTimers, agentID)
		})
		idleTimers[agentID] = timer
	}
}

// cancelIdle cancels an existing idle timer for the agent.
func cancelIdle(projectID, agentName string) {
	agentID := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", agentName, projectID)))

	agentsMu.Lock()
	defer agentsMu.Unlock()

	if timer, exists := idleTimers[agentID]; exists {
		timer.Stop()
		delete(idleTimers, agentID)
	}
}

// ── System Prompt Builder ────────────────────────────────────────────────────

// buildSystemPrompt creates a system prompt for the chat agent with project context.
func buildSystemPrompt(proj *engine.Project) string {
	var sb strings.Builder

	sb.WriteString(`<system>
You are an interactive AI assistant for the Orqen project management system.
You communicate with users via a chat interface. Be helpful, concise, and accurate.

## Available Tools
You have access to the following tools via MCP:
`)
	sb.WriteString("- `chat_history_get`: Get recent conversation history\n")
	sb.WriteString("- `chat_memory_search`: Search past conversations with FTS5\n")
	sb.WriteString("- `chat_workitem_create`: Create a workitem in a specified lane\n")
	sb.WriteString("- `chat_workitem_list`: List workitems, optionally filtered by lane\n")
	sb.WriteString("- `chat_workitem_get`: Get details of a specific workitem\n")
	sb.WriteString("- `chat_file_list`: List files in the project directory\n")
	sb.WriteString("- `chat_file_read`: Read a file's content\n")
	sb.WriteString("- `chat_file_edit`: Propose a file edit (requires user approval before applying)\n")
	sb.WriteString("- `chat_project_get`: Project structure overview\n")
	sb.WriteString("- `chat_lane_list`: List available lanes with stats\n\n")

	sb.WriteString("## Rules\n")
	sb.WriteString("1. Always read files before proposing changes — never edit blindly.\n")
	sb.WriteString("2. Use `chat_file_edit` for any file modifications. The user must approve before changes are applied.\n")
	sb.WriteString("3. Be concise in your responses. Use tools to gather information before answering.\n")
	sb.WriteString("4. When creating workitems, use descriptive names and provide clear descriptions.\n")
	sb.WriteString("5. If you are unsure about something, ask the user for clarification.\n")
	sb.WriteString("6. Respect the project structure — do not modify `.orqen/` or `.git/` directories.\n")

	// Add project context summary
	sb.WriteString("\n## Project Context\n")
	sb.WriteString(buildProjectSummary(proj))

	sb.WriteString("\n</system>")

	return sb.String()
}

// buildProjectSummary generates a summary of the project structure.
func buildProjectSummary(proj *engine.Project) string {
	if proj == nil {
		return "No project loaded."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project ID: %s\n", proj.Id))

	if len(proj.Modules) == 0 {
		sb.WriteString("No modules configured.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Modules: %d\n", len(proj.Modules)))
	for _, mod := range proj.Modules {
		sb.WriteString(fmt.Sprintf("\n### Module: %s\n", mod.Name))

		lanes := mod.GetLanesInOrder()
		if len(lanes) == 0 {
			sb.WriteString("  No lanes.\n")
			continue
		}

		for _, lane := range lanes {
			count := lane.CountWorkItems()
			active := lane.CountActiveWorkItems()
			sb.WriteString(fmt.Sprintf("  - Lane: %s (%d items, %d active)", lane.Name, count, active))
			if lane.Purpose != "" {
				sb.WriteString(fmt.Sprintf(" — %s", lane.Purpose))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ── Confirmation Keywords ───────────────────────────────────────────────────

var approvalKeywords = map[string]bool{
	"yes": true, "y": true, "ok": true, "approve": true,
	"apply": true, "do it": true, "go ahead": true,
}

var rejectionKeywords = map[string]bool{
	"no": true, "n": true, "cancel": true, "reject": true,
	"discard": true, "skip": true, "dont": true, "don't": true,
}

func isApprovalKeyword(text string) bool {
	return approvalKeywords[text]
}

func isRejectionKeyword(text string) bool {
	return rejectionKeywords[text]
}

// ── Chat Client Session (with OnProgress) ────────────────────────────────────

var (
	chatSessions   = map[acp.SessionId]*chatClientSession{}
	chatSessionsMu sync.Mutex
)

func chatSessionDel(sid acp.SessionId) {
	chatSessionsMu.Lock()
	defer chatSessionsMu.Unlock()
	delete(chatSessions, sid)
}

func chatSessionSet(sid acp.SessionId, s *chatClientSession) {
	chatSessionsMu.Lock()
	defer chatSessionsMu.Unlock()
	chatSessions[sid] = s
}

func chatSessionGet(sid acp.SessionId) *chatClientSession {
	chatSessionsMu.Lock()
	defer chatSessionsMu.Unlock()
	return chatSessions[sid]
}

// chatClientSession captures streaming output for a single ACP chat session
// and sends progress updates to the channel.
type chatClientSession struct {
	channel       Channel
	sessionID     string
	ctx           context.Context
	agentChunks   []string
	userChunks    []string
	thoughtChunks []string
	toolCallById  map[acp.ToolCallId]*acp.SessionUpdateToolCall
	mu            sync.Mutex
}

func newChatClientSession(channel Channel, sessionID string, ctx context.Context) *chatClientSession {
	return &chatClientSession{
		channel:      channel,
		sessionID:    sessionID,
		ctx:          ctx,
		toolCallById: make(map[acp.ToolCallId]*acp.SessionUpdateToolCall),
	}
}

// getResponse returns the accumulated agent response text.
func (c *chatClientSession) getResponse() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.TrimSpace(strings.Join(c.agentChunks, ""))
}

func (c *chatClientSession) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update

	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case u.AgentThoughtChunk != nil:
		content := u.AgentThoughtChunk.Content
		if content.Text != nil {
			c.thoughtChunks = append(c.thoughtChunks, content.Text.Text)
		}
		// Send progress
		_ = c.channel.OnProgress(c.ctx, c.sessionID, "🔄 Thinking...")

	case u.UserMessageChunk != nil:
		content := u.UserMessageChunk.Content
		if content.Text != nil {
			c.userChunks = append(c.userChunks, content.Text.Text)
		}

	case u.AgentMessageChunk != nil:
		content := u.AgentMessageChunk.Content
		if content.Text != nil {
			c.agentChunks = append(c.agentChunks, content.Text.Text)
		}

	case u.ToolCall != nil:
		title := u.ToolCall.Title
		c.toolCallById[u.ToolCall.ToolCallId] = u.ToolCall
		// Send progress
		if title != "" {
			_ = c.channel.OnProgress(c.ctx, c.sessionID, fmt.Sprintf("🔧 Calling tool: %s...", title))
		} else {
			_ = c.channel.OnProgress(c.ctx, c.sessionID, "🔧 Calling tool...")
		}

	case u.ToolCallUpdate != nil:
		uToolCall := u.ToolCallUpdate
		sToolCall, exists := c.toolCallById[uToolCall.ToolCallId]

		title := ""
		if uToolCall.Title != nil {
			title = *uToolCall.Title
		}
		status := ""
		if uToolCall.Status != nil {
			status = string(*uToolCall.Status)
		}

		if exists && (status == string(acp.ToolCallStatusCompleted) || status == string(acp.ToolCallStatusFailed)) {
			delete(c.toolCallById, uToolCall.ToolCallId)
		}

		if title != "" {
			_ = c.channel.OnProgress(c.ctx, c.sessionID, fmt.Sprintf("%s Tool completed: %s", statusIcon(status), title))
		} else if exists {
			_ = c.channel.OnProgress(c.ctx, c.sessionID, fmt.Sprintf("%s Tool completed: %s", statusIcon(status), sToolCall.Title))
		}
	}

	return nil
}

func statusIcon(status string) string {
	switch status {
	case string(acp.ToolCallStatusCompleted):
		return "✅"
	case string(acp.ToolCallStatusFailed):
		return "❌"
	default:
		return "⏳"
	}
}

// ── chatACPClient — ACP Client Interface (re-implemented from pkg/agent) ─────

// chatLogger is a minimal logger for the chat agent.
type chatLogger string

func (l chatLogger) Log(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, "[chat-agent] "+format, args...)
}

// chatACPClient implements the ACP client interface for chat sessions.
type chatACPClient struct {
	logger    chatLogger
	terminals *chatTerminalManager
}

func (c *chatACPClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	for _, o := range params.Options {
		if o.Kind == acp.PermissionOptionKindAllowOnce || o.Kind == acp.PermissionOptionKindAllowAlways {
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{OptionId: o.OptionId},
				},
			}, nil
		}
	}
	if len(params.Options) > 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Selected: &acp.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId},
			},
		}, nil
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{}},
	}, nil
}

func (c *chatACPClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	cs := chatSessionGet(params.SessionId)
	if cs == nil {
		return nil
	}
	return cs.SessionUpdate(ctx, params)
}

func (c *chatACPClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}

	file, err := os.Open(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("open %s: %w", params.Path, err)
	}
	defer file.Close()

	startLine := 1
	if params.Line != nil && *params.Line > 0 {
		startLine = *params.Line
	}

	limit := 0
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var result strings.Builder
	currentLine := 0
	linesCollected := 0

	for scanner.Scan() {
		currentLine++
		if currentLine < startLine {
			continue
		}
		if limit > 0 && linesCollected >= limit {
			break
		}
		if linesCollected > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(scanner.Text())
		linesCollected++
	}

	if err := scanner.Err(); err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}

	return acp.ReadTextFileResponse{Content: result.String()}, nil
}

func (c *chatACPClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	dir := filepath.Dir(params.Path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return acp.WriteTextFileResponse{}, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write %s: %w", params.Path, err)
	}
	return acp.WriteTextFileResponse{}, nil
}

func (c *chatACPClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	env := make([]struct{ Name, Value string }, 0, len(params.Env))
	for _, e := range params.Env {
		env = append(env, struct{ Name, Value string }{Name: e.Name, Value: e.Value})
	}
	cwd := ""
	if params.Cwd != nil {
		cwd = *params.Cwd
	}
	terminalID, err := c.terminals.createSession(
		ctx, params.Command, params.Args, env, cwd,
		func() *int64 {
			if params.OutputByteLimit == nil {
				return nil
			}
			v := int64(*params.OutputByteLimit)
			return &v
		}(),
	)
	if err != nil {
		return acp.CreateTerminalResponse{}, fmt.Errorf("create terminal: %w", err)
	}
	return acp.CreateTerminalResponse{TerminalId: terminalID}, nil
}

func (c *chatACPClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	output, truncated, exitStatusInternal, err := c.terminals.terminalOutput(params.TerminalId)
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}
	resp := acp.TerminalOutputResponse{Output: output, Truncated: truncated}
	if exitStatusInternal != nil {
		resp.ExitStatus = &acp.TerminalExitStatus{
			ExitCode: exitStatusInternal.ExitCode,
			Signal:   exitStatusInternal.Signal,
		}
	}
	return resp, nil
}

func (c *chatACPClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	if err := c.terminals.releaseTerminal(params.TerminalId); err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *chatACPClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	exitCode, signal, err := c.terminals.waitForExit(ctx, params.TerminalId)
	if err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}
	return acp.WaitForTerminalExitResponse{ExitCode: exitCode, Signal: signal}, nil
}

func (c *chatACPClient) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	if err := c.terminals.killTerminal(params.TerminalId); err != nil {
		return acp.KillTerminalResponse{}, err
	}
	return acp.KillTerminalResponse{}, nil
}

// ── Chat Terminal Manager (re-implemented from pkg/agent/terminal.go) ────────

type chatTerminalManager struct {
	mu       sync.RWMutex
	sessions map[string]*chatTerminalSession
	nextID   uint64
}

type chatTerminalSession struct {
	cmd             *exec.Cmd
	stdout          *strings.Builder
	stderr          *strings.Builder
	exited          chan struct{}
	exitCode        *int
	signal          *string
	released        bool
	outputByteLimit int64
}

type chatTerminalExitStatus struct {
	ExitCode *int
	Signal   *string
}

func newChatTerminalManager() *chatTerminalManager {
	return &chatTerminalManager{
		sessions: make(map[string]*chatTerminalSession),
	}
}

func (tm *chatTerminalManager) createSession(
	ctx context.Context,
	command string,
	args []string,
	env []struct{ Name, Value string },
	cwd string,
	outputByteLimit *int64,
) (string, error) {

	id := fmt.Sprintf("%d", tm.nextID)
	tm.nextID++

	cmd := exec.CommandContext(ctx, command, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	for _, e := range env {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", e.Name, e.Value))
	}

	limit := int64(32 * 1024) // 32KB default
	if outputByteLimit != nil {
		limit = *outputByteLimit
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start terminal command: %w", err)
	}

	sess := &chatTerminalSession{
		cmd:             cmd,
		stdout:          stdout,
		stderr:          stderr,
		exited:          make(chan struct{}),
		outputByteLimit: limit,
	}

	go func() {
		_ = cmd.Wait()
		exitCode := cmd.ProcessState.ExitCode()
		sess.exitCode = &exitCode
		close(sess.exited)
	}()

	tm.mu.Lock()
	tm.sessions[id] = sess
	tm.mu.Unlock()

	return id, nil
}

func (tm *chatTerminalManager) terminalOutput(terminalID string) (output string, truncated bool, exitStatus *chatTerminalExitStatus, err error) {
	tm.mu.RLock()
	sess, exists := tm.sessions[terminalID]
	tm.mu.RUnlock()
	if !exists {
		return "", false, nil, fmt.Errorf("terminal %s not found", terminalID)
	}

	raw := sess.stdout.String()
	truncated = false
	rawLen := int64(len(raw))
	if rawLen > sess.outputByteLimit {
		raw = raw[rawLen-sess.outputByteLimit:]
		truncated = true
	}

	select {
	case <-sess.exited:
		exitStatus = &chatTerminalExitStatus{ExitCode: sess.exitCode, Signal: sess.signal}
	default:
	}

	return raw, truncated, exitStatus, nil
}

func (tm *chatTerminalManager) killTerminal(terminalID string) error {
	tm.mu.RLock()
	sess, exists := tm.sessions[terminalID]
	tm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("terminal %s not found", terminalID)
	}
	if sess.cmd.Process != nil {
		return sess.cmd.Process.Kill()
	}
	return nil
}

func (tm *chatTerminalManager) releaseTerminal(terminalID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	sess, exists := tm.sessions[terminalID]
	if !exists {
		return fmt.Errorf("terminal %s not found", terminalID)
	}
	if sess.released {
		return fmt.Errorf("terminal %s already released", terminalID)
	}
	sess.released = true
	if sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}
	delete(tm.sessions, terminalID)
	return nil
}

func (tm *chatTerminalManager) waitForExit(ctx context.Context, terminalID string) (exitCode *int, signal *string, err error) {
	tm.mu.RLock()
	sess, exists := tm.sessions[terminalID]
	tm.mu.RUnlock()
	if !exists {
		return nil, nil, fmt.Errorf("terminal %s not found", terminalID)
	}
	select {
	case <-sess.exited:
		return sess.exitCode, sess.signal, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// ── Legacy: ExecChatAgent (deprecated, kept for backward compatibility) ──────

// ExecChatAgent launches an ACP agent for a chat session.
// Deprecated: Use AgentSession instead for proper multi-turn conversation support.
func ExecChatAgent(
	ctx context.Context,
	proj *engine.Project,
	agentName string,
	sessionID string,
	prompt string,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
	chatMCPURL string,
) (response string, err error) {

	// Resolve agent command from project config
	resolvedAgentName := proj.Agents.GetName(agentName)
	command := proj.Agents.GetCommand(resolvedAgentName)
	if len(command) == 0 {
		return "", fmt.Errorf("chat: agent %q not found in project config (default: %q)",
			resolvedAgentName, proj.Agents.Default)
	}

	// Apply timeout if not already set by caller
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultAgentTimeout)
		defer cancel()
	}

	// Get or create cached agent subprocess
	process, err := getOrCreateProcess(proj.Id, resolvedAgentName, command)
	if err != nil {
		return "", fmt.Errorf("chat: get process: %w", err)
	}

	logger := chatLogger(fmt.Sprintf("[%s] [chat-session:%s]", resolvedAgentName, sessionID))

	// Build MCP server config pointing to the chat MCP HTTP server
	mcpServers := []acp.McpServer{
		{
			Http: &acp.McpServerHttpInline{
				Name: "Orqen Chat MCP",
				Url:  chatMCPURL,
			},
		},
	}

	// Create ACP session
	sess, err := process.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        proj.DirAbs,
		McpServers: mcpServers,
	})
	if err != nil {
		return "", formatACPError(resolvedAgentName, "new session", err)
	}
	logger.Log("session created: %s\n", sess.SessionId)

	// Create client session for streaming
	clientSession := &chatClientSession{
		channel:      &NoOpChannel{},
		sessionID:    sessionID,
		ctx:          ctx,
		toolCallById: make(map[acp.ToolCallId]*acp.SessionUpdateToolCall),
	}
	chatSessionSet(sess.SessionId, clientSession)
	defer chatSessionDel(sess.SessionId)

	// Attempt LoadSession if supported, otherwise prepend history
	loadSessionSupported := false
	_, loadErr := process.conn.LoadSession(ctx, acp.LoadSessionRequest{
		SessionId: acp.SessionId(sessionID),
	})
	if loadErr == nil {
		loadSessionSupported = true
		logger.Log("loaded prior session via LoadSession\n")
	} else {
		logger.Log("LoadSession not supported or failed (%v), prepending history\n", loadErr)
	}

	// Build prompt
	fullPrompt := prompt
	if !loadSessionSupported {
		messages, histErr := sessionMgr.GetFullHistoryForContext(sessionID)
		if histErr != nil {
			logger.Log("warning: failed to load history: %v\n", histErr)
		} else if len(messages) > 0 {
			formattedHistory := FormatHistoryForPrompt(messages)
			fullPrompt = fmt.Sprintf(
				"<conversation_history>\n%s\n</conversation_history>\n\n<new_message>\n%s\n</new_message>",
				formattedHistory,
				prompt,
			)
		}
	}

	// Send prompt
	_, err = process.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: sess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(fullPrompt)},
	})
	if err != nil {
		return "", formatACPError(resolvedAgentName, "prompt", err)
	}

	// Extract response from agent session
	response = clientSession.getResponse()
	if response == "" {
		response = "(agent completed but no response text was captured)"
	}

	// Save messages to chat store
	if saveErr := chatStore.AddMessage(sessionID, RoleUser, prompt); saveErr != nil {
		logger.Log("warning: failed to save user message: %v\n", saveErr)
	}
	if saveErr := chatStore.AddMessage(sessionID, RoleAssistant, response); saveErr != nil {
		logger.Log("warning: failed to save assistant message: %v\n", saveErr)
	}

	logger.Log("finished\n")

	return response, nil
}

// formatACPError formats an ACP error consistently with pkg/agent/exec.go.
func formatACPError(agentName, operation string, err error) error {
	if re, ok := err.(*acp.RequestError); ok {
		if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
			return fmt.Errorf("[%s] %s error: %s", agentName, operation, string(b))
		}
		return fmt.Errorf("[%s] %s error (%d): %s", agentName, operation, re.Code, re.Message)
	}
	return fmt.Errorf("[%s] %s error: %w", agentName, operation, err)
}
