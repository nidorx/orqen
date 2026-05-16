package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/agent"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

const (
	defaultAgentTimeout   = 5 * time.Minute
	subprocessIdleTimeout = 5 * time.Minute
)

// Channel is a generic I/O interface for delivering agent responses to users.
// The agent knows nothing about channels (Telegram, CLI, Web) - it just calls
// Send/OnProgress.
type Channel interface {
	// Send delivers the final agent response to the user.
	Send(ctx context.Context, text string)

	// OnProgress delivers intermediate progress messages (optional).
	// Examples: "🔄 Thinking...", "🔧 Calling tool: read_file..."
	// May be a no-op for channels that don't support streaming.
	OnProgress(ctx context.Context, text string)
}

type promptRequest struct {
	prompt  string
	channel Channel
	result  chan promptResult
}

type promptResult struct {
	response string
	err      error
}

// Session represents one ACP conversation session within a chat session.
// It maintains a prompt queue (FIFO) and delegates I/O to a Channel.
type Session struct {
	mu            sync.Mutex
	agent         *agent.Agent
	project       *engine.Project
	agentName     string
	command       []string
	chatMCPURL    string
	logger        agent.Logger
	acpSessionId  acp.SessionId
	chatStore     *memory.ChatStore
	chatSessionId string // chat session ID (SQLite)
	confirmMgr    memory.ConfirmationManager

	closed        bool
	queue         chan promptRequest
	wg            sync.WaitGroup     // waits for queue worker to finish
	currentCtx    context.Context    // context of the current prompt
	currentCancel context.CancelFunc // cancel for the prompt currently running
	isFirstPrompt bool               // true = next prompt should include system instructions
}

// NewAgentSession creates a new agent session. Does NOT start the subprocess
// (lazy start on first Prompt call).
func NewAgentSession(
	project *engine.Project,
	agentName string,
	chatMCPURL string,
	chatStore *memory.ChatStore,
	sessionID string,
	confirmMgr memory.ConfirmationManager,
) *Session {

	resolvedAgentName := project.Agents.GetName(agentName)
	command := project.Agents.GetCommand(resolvedAgentName)

	as := &Session{
		project:       project,
		agentName:     resolvedAgentName,
		command:       command,
		chatMCPURL:    chatMCPURL,
		chatStore:     chatStore,
		chatSessionId: sessionID,
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
func (as *Session) Prompt(ctx context.Context, text string, channel Channel) (string, error) {
	as.mu.Lock()
	if as.closed {
		as.mu.Unlock()
		return "", fmt.Errorf("agent session is closed")
	}
	as.mu.Unlock()

	// Check confirmation intercept first
	if as.confirmMgr != nil && as.confirmMgr.HasPendingEdit(as.chatSessionId) {
		normalized := strings.ToLower(strings.TrimSpace(text))
		if isApprovalKeyword(normalized) {
			if err := as.confirmMgr.ApplyEdit(as.chatSessionId); err != nil {
				channel.Send(ctx, fmt.Sprintf("❌ Failed to apply edit: %v", err))
				return "", fmt.Errorf("apply edit: %w", err)
			}
			msg := "✅ Edit applied successfully."
			channel.Send(ctx, msg)
			return msg, nil
		}
		if isRejectionKeyword(normalized) {
			if err := as.confirmMgr.RejectEdit(as.chatSessionId); err != nil {
				channel.Send(ctx, fmt.Sprintf("❌ Failed to reject edit: %v", err))
				return "", fmt.Errorf("reject edit: %w", err)
			}
			msg := "❌ Edit discarded."
			channel.Send(ctx, msg)
			return msg, nil
		}
		// Not approval/rejection - fall through to normal prompt
	}

	// Enqueue prompt
	resultCh := make(chan promptResult, 1)
	select {
	case as.queue <- promptRequest{prompt: text, channel: channel, result: resultCh}:
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
func (as *Session) Cancel() error {
	as.mu.Lock()
	cancelFn := as.currentCancel
	currentCtx := as.currentCtx
	as.mu.Unlock()

	if cancelFn == nil {
		return fmt.Errorf("no active prompt to cancel")
	}

	// Send ACP cancel
	if as.agent != nil && as.acpSessionId != "" {
		_ = as.agent.Cancel(context.Background(), acp.CancelNotification{
			SessionId: acp.SessionId(as.acpSessionId),
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
func (as *Session) Close() error {
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
	if as.agent != nil && as.acpSessionId != "" {
		_, _ = as.agent.CloseSession(context.Background(), acp.CloseSessionRequest{
			SessionId: acp.SessionId(as.acpSessionId),
		})

	}
	as.mu.Unlock()

	// Schedule subprocess idle timeout
	if as.agent != nil {
		as.agent.ScheduleIdle(subprocessIdleTimeout)
	}

	return nil
}

// queueWorker processes prompts from the queue in FIFO order.
func (as *Session) queueWorker() {
	defer as.wg.Done()

	for req := range as.queue {
		as.executePrompt(req)
	}
}

// executePrompt runs a single prompt turn. Called by queueWorker sequentially.
func (as *Session) executePrompt(req promptRequest) {
	as.mu.Lock()
	if as.closed {
		as.mu.Unlock()
		req.result <- promptResult{err: fmt.Errorf("agent session is closed")}
		return
	}
	as.mu.Unlock()

	// Ensure subprocess is running
	agt, err := agent.GetAgent(as.project.Id, as.agentName, as.command)
	if err != nil {
		req.result <- promptResult{err: fmt.Errorf("get agent: %w", err)}
		return
	}

	// Ensure ACP session exists
	if err := as.ensureSession(agt); err != nil {
		req.result <- promptResult{err: fmt.Errorf("ensure session: %w", err)}
		return
	}

	// Build the prompt
	as.mu.Lock()
	fullPrompt := req.prompt
	if as.isFirstPrompt {
		fullPrompt = fmt.Sprintf("%s\n\n<message>%s</message>", buildSystemPrompt(as.project), req.prompt)
		as.isFirstPrompt = false
	}

	chatStore := as.chatStore
	as.mu.Unlock()

	// Create cancellable context for this prompt
	promptCtx, cancel := context.WithTimeout(context.Background(), defaultAgentTimeout)

	as.mu.Lock()
	as.currentCancel = cancel
	as.currentCtx = promptCtx
	as.mu.Unlock()

	// Create a client session for streaming with OnProgress callback
	clientSession := agent.ClientSessionNew(as.logger, onSessionUpdateFn(req.channel))
	agent.ClientSessionSet(as.acpSessionId, clientSession)
	defer agent.ClientSessionDel(as.acpSessionId)

	// Send prompt
	_, err = as.agent.Prompt(promptCtx, acp.PromptRequest{
		SessionId: as.acpSessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(fullPrompt)},
	})

	// Clean up

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
	response := clientSession.AgentChunk.Content()
	if response == "" {
		response = "(agent completed but no response text was captured)"
	}

	// Save messages to chat store
	_ = chatStore.AddMessage(as.chatSessionId, memory.RoleUser, req.prompt)
	_ = chatStore.AddMessage(as.chatSessionId, memory.RoleAssistant, response)

	// Send final response via channel
	req.channel.Send(context.Background(), response)

	req.result <- promptResult{response: response, err: nil}
}

// ensureSession creates an ACP session if one doesn't exist yet.
func (as *Session) ensureSession(agt *agent.Agent) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.acpSessionId != "" {
		return nil // already have a session
	}

	mcpServers := []acp.McpServer{
		{
			Http: &acp.McpServerHttpInline{
				Name:    "Orqen Chat MCP",
				Url:     as.chatMCPURL,
				Headers: make([]acp.HttpHeader, 0),
			},
		},
	}

	logger := agent.NewLogger(as.agentName, "[chat]")

	sess, err := agt.NewSession(context.Background(), acp.NewSessionRequest{
		Cwd:        as.project.DirAbs,
		McpServers: mcpServers,
	})
	if err != nil {
		return formatACPError(as.agentName, "new session", err)
	}
	logger.Log("session created: %s\n", sess.SessionId)

	as.logger = logger
	as.acpSessionId = sess.SessionId
	as.agent = agt
	return nil
}

func onSessionUpdateFn(channel Channel) func(ctx context.Context, params acp.SessionNotification) {
	return func(ctx context.Context, params acp.SessionNotification) {
		u := params.Update

		switch {
		case u.AgentThoughtChunk != nil:
			channel.OnProgress(ctx, "🔄 Thinking...")
		}
	}
}

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
	sb.WriteString("1. Always read files before proposing changes - never edit blindly.\n")
	sb.WriteString("2. Use `chat_file_edit` for any file modifications. The user must approve before changes are applied.\n")
	sb.WriteString("3. Be concise in your responses. Use tools to gather information before answering.\n")
	sb.WriteString("4. When creating workitems, use descriptive names and provide clear descriptions.\n")
	sb.WriteString("5. If you are unsure about something, ask the user for clarification.\n")
	sb.WriteString("6. Respect the project structure - do not modify `.orqen/` or `.git/` directories.\n")

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
				sb.WriteString(fmt.Sprintf(" - %s", lane.Purpose))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
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
