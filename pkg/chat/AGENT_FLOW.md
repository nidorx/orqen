# Chat Agent Flow — Architecture

## Overview

The chat agent enables interactive, multi-turn conversations between users and an ACP agent subprocess,
mediated through the chat MCP server. It is channel-agnostic (Telegram, CLI, Web — same core logic).

## Architecture Diagram

```
                  ┌─────────────┐
                  │  Telegram   │  (future: Web, CLI)
                  │  Bot        │
                  └──────┬──────┘
                         │ Send(ctx, sessionID, text)
                         │ OnProgress(ctx, sessionID, text)  (optional)
                         ▼
              ┌──────────────────────┐
              │    Channel (iface)   │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐     ┌──────────────────────┐
              │   AgentSession       │────►│  ACP Subprocess      │
              │  - acpSessionId      │     │  (1 per project+agent)│
              │  - queue (chan)      │     │  cached, idle 5min   │
              │  - currentCancel     │     └──────────┬───────────┘
              │  - confirmationMgr   │                │
              └──────┬───────────────┘                │
                     │                                │
              ┌──────▼──────┐               ┌────────▼──────────┐
              │ Confirmation│               │ Chat MCP Server    │
              │  Manager    │               │ (tools for agent)  │
              └─────────────┘               └───────────────────┘
```

## Component Details

### 1. `Channel` — Generic I/O Interface

Abstraction for any communication channel (Telegram, CLI, Web). The agent knows nothing about channels.

```go
type Channel interface {
    // Send delivers the final agent response to the user.
    Send(ctx context.Context, sessionID, text string) error

    // OnProgress delivers intermediate progress messages (optional implementation).
    // Examples: "🔄 Thinking...", "🔧 Calling tool: read_file..."
    // May be a no-op for channels that don't support streaming.
    OnProgress(ctx context.Context, sessionID, text string) error
}
```

### 2. `AgentSession` — Persistent ACP Session

One ACP session per chat session. Maintains conversation context across multiple prompt turns.

```go
type AgentSession struct {
    mu            sync.Mutex
    proj          *engine.Project
    agentName     string
    command       []string
    chatMCPURL    string
    conn          *acp.ClientSideConnection // nil = subprocess not started
    acpSessionID  string                    // empty = no ACP session created yet
    chatStore     *ChatStore
    chatSessionID string   // chat session ID (SQLite)
    channel       Channel  // output channel
    confirmMgr    *ConfirmationManager

    // Prompt queue
    queue       chan promptRequest
    currentCancel context.CancelFunc // cancel for the prompt currently running
    closed      bool
    wg          sync.WaitGroup       // waits for queue worker to finish

    // System prompt tracking
    isFirstPrompt bool // true = next prompt should include system instructions
}

type promptRequest struct {
    prompt string
    result chan promptResult
}

type promptResult struct {
    response string
    err      error
}
```

#### Methods

| Method | Behavior |
|--------|----------|
| `NewAgentSession(proj, agentName, chatMCPURL, chatStore, chatSessionID, channel, confirmMgr)` | Creates session. Does NOT start subprocess (lazy start). |
| `Prompt(ctx, text) → (response, error)` | Checks confirmation intercept first. If approval/rejection → handles it and returns immediately. Otherwise enqueues prompt and blocks on `result` channel. Queue is FIFO. |
| `Cancel() → error` | Sends `session/cancel` via ACP if a prompt is currently executing. Sets `stopReason: "cancelled"`. |
| `Close() → error` | Marks session closed, waits for queue to drain, closes ACP session, schedules subprocess idle timer (5 min). |

### 3. Subprocess Manager (Global)

Manages the ACP agent subprocess lifecycle across sessions.

```go
var (
    agents      = map[string]*agentProcess{}  // key: hash(projectID + agentName)
    agentsMu    sync.Mutex
    idleTimers  = map[string]*time.Timer{}    // 5-minute countdown
    idleTimeout = 5 * time.Minute
)

type agentProcess struct {
    cmd      *exec.Cmd
    conn     *acp.ClientSideConnection
    client   *chatACPClient
    sessions map[string]*AgentSession  // active sessions using this subprocess
    mu       sync.Mutex
}
```

#### Lifecycle

```
Process Creation:
  getOrCreateProcess()
    → hash(projectID + agentName) → lookup in agents map
    → not found: spawn subprocess, initialize ACP connection
    → cache and return

Session Using Process:
  AgentSession.Prompt()
    → ensureProcess()
    → if subprocess is idle (timer running) → cancelIdle()
    → if subprocess doesn't exist → create it
    → as.sessions[sessionId] = this session

Session Close:
  AgentSession.Close()
    → remove from as.sessions
    → if as.sessions is empty → scheduleIdle(hash, 5min)

Idle Timer Expires:
  → kill subprocess (cmd.Process.Kill())
  → remove from agents map
  → close connection

Next Session After Idle:
  → subprocess is gone → spawn new one
```

### 4. Prompt Execution Flow

```
User sends message
    │
    ▼
AgentSession.Prompt(text)
    │
    ├── HasPendingEdit? ──YES──┐
    │                          │
    │   NO                     ▼
    │                    IsApproval(text)? ──YES──► ApplyEdit() → Send("✅ Applied") → return
    │                    IsRejection(text)? ──YES──► RejectEdit() → Send("❌ Discarded") → return
    │                    Neither ──► fall through to enqueue
    │
    ▼
enqueue prompt into queue channel (FIFO)
    │
    ▼
queue worker (goroutine) dequeues
    │
    ▼
ensureProcess() — start subprocess if needed
    │
    ▼
ensureSession() — create ACP session if first prompt
    │
    ▼
build fullPrompt:
    if isFirstPrompt:
        fullPrompt = "<system>...instructions...</system>\n\n<message>...user text...</message>"
        isFirstPrompt = false
    else:
        fullPrompt = user text  (agent retains context via sessionId)
    │
    ▼
agent.conn.Prompt(ctx, acpSessionID, fullPrompt)
    │
    ├── SessionUpdate notifications arrive during processing
    │   ├── AgentMessageChunk → accumulate response text
    │   ├── AgentThoughtChunk → OnProgress("🔄 Thinking...")
    │   ├── ToolCall → OnProgress("🔧 Calling tool: {title}...")
    │   └── ToolCallUpdate → OnProgress("✅ Tool completed: {title}")
    │
    ▼
Prompt completes → extract accumulated response
    │
    ▼
Save to ChatStore:
    chatStore.AddMessage(chatSessionID, RoleUser, userText)
    chatStore.AddMessage(chatSessionID, RoleAssistant, response)
    │
    ▼
channel.Send(ctx, chatSessionID, response)
    │
    ▼
req.result ← promptResult{response, nil}
    │
    ▼
User receives response
```

### 5. Confirmation Intercept

When the agent calls `chat_file_edit` via MCP, the tool handler creates a `PendingEdit` in `ChatStore`.
The agent's response includes a note about pending approval.

On the user's **next** message:

1. `AgentSession.Prompt()` checks `ConfirmationManager.HasPendingEdit(chatSessionID)`
2. If pending edit exists:
   - `IsApproval(text)` → `ApplyEdit()` → `channel.Send("✅ Edit applied successfully.")` → returns (does NOT forward to agent)
   - `IsRejection(text)` → `RejectEdit()` → `channel.Send("❌ Edit discarded.")` → returns (does NOT forward to agent)
   - Neither → falls through to normal prompt enqueueing
3. If no pending edit → normal prompt enqueueing

This means **confirmation takes priority** over the agent. The user's "yes"/"no" is consumed by the
confirmation system and never reaches the agent.

### 6. System Prompt

Injected as the **first prompt** to the ACP session. After that, the agent maintains context via the
session ID (no need to resend history).

#### Content (in English)

```
You are an interactive AI assistant for the Orqen project management system.

## Project Context
{Dynamic: project modules, lanes, active workitems summary}

## Available Tools
You have access to the following tools via MCP:
- chat_history_get: Get recent conversation history
- chat_memory_search: Search past conversations
- chat_workitem_create: Create a workitem in a lane
- chat_workitem_list: List workitems
- chat_workitem_get: Get workitem details
- chat_file_list: List files in the project
- chat_file_read: Read a file's content
- chat_file_edit: Propose a file edit (requires user approval before applying)
- chat_project_get: Get project overview
- chat_lane_list: List available lanes

## Rules
1. Always read files before proposing changes — never edit blindly.
2. Use chat_file_edit for any file modifications. The user must approve before changes are applied.
3. Be concise in your responses. Use tools to gather information before answering.
4. When creating workitems, use descriptive names and provide clear descriptions.
5. If you're unsure about something, ask the user for clarification.
6. Respect the project structure — do not modify .orqen/ or .git/ directories.
```

### 7. ACP Protocol Reference

| ACP Method | Usage |
|------------|-------|
| `initialize` | Called once when subprocess starts |
| `session/new` | Creates a new session (called once per chat session) |
| `session/prompt` | Sends user message; one call per turn |
| `session/update` | Streams chunks during prompt processing |
| `session/cancel` | Interrupts current prompt turn |
| `session/del` | Closes the session (when user does `/new` or session expires) |

**Conversation continuity:** After a `session/prompt` completes, the next `session/prompt` on the
same `sessionId` builds on prior context. No history payload needed from the client.

### 8. Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| One ACP subprocess per project+agent, cached | Process startup is expensive; reuse across sessions |
| Subprocess idle timeout: 5 minutes | Free resources when no active sessions |
| One ACP session per chat session | Maintains conversation continuity |
| Prompt queue (FIFO, blocking) | Respects ACP turn model — one prompt at a time |
| System prompt in first prompt only | ACP session retains context; no need to resend |
| Confirmation intercept before agent | "yes"/"no" applies to pending edit, not forwarded |
| Channel is generic abstraction | Same agent logic for Telegram, CLI, Web |
| OnProgress optional | Some channels support streaming, others don't |
| `session/cancel` for `/stop` | ACP-native interrupt mechanism |
