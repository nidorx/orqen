# Architecture — pkg/mcp

> **For developers and AI agents.** This document describes the MCP package internals and invocation flow.

## Responsibility

The `mcp` package is the **bridge between AI agents and Orqen's filesystem-backed project state**. It:

1. **Exposes operations as MCP tools** — Each tool (create item, move item, list lanes, etc.) is a well-defined function with typed input/output, discoverable by any MCP-compatible agent.
2. **Proxies agent-side calls to the host process** — When an agent runs in a subprocess, the stdio server forwards tool calls to the main process over HTTP.
3. **Serves tools directly for remote agents** — When agents connect directly (e.g., running alongside Orqen), the Streamable HTTP handler dispatches tool calls without a proxy layer.

The package does **not** manage project state — it delegates to `pkg/project`. It is purely a **protocol layer**: receive MCP tool calls, execute the corresponding project operation, return structured results.

## Files

| File | Responsibility |
|------|----------------|
| `server.go` | Server creation (`createServer`), tool registration helpers (`addTool`, `addToolProject`, `addToolProxy`, `addToolProjecProxy`, `addToolProjecProxyWithJobId`), and proxy functions (`sseProxy`, `sseProxyWithJobId`) |
| `server_stdio.go` | Entry point for the stdio subprocess (`StartStdio`). Creates an MCP client that connects to the host's Streamable HTTP endpoint and registers all tools as proxies |
| `server_http.go` | Entry point for the host's HTTP server (`ServerHttp`). Creates an `http.Handler` using `mcp.NewStreamableHTTPHandler` with all tools registered directly |
| `tool_*.go` | Individual tool implementations — each defines `Input`/`Output` structs, a tool name constant, an `init()` registration, and a handler function |
| `utils.go` | Shared utilities (e.g., `findModuleByJobID`) |

## Invocation Flow

### The Three Roles

Orqen uses MCP in two distinct roles within a single process:

```
┌──────────────────────────────────────────────────────────────────┐
│                    Orqen Main Process                             │
│                                                                    │
│  ┌─────────────────────────┐          ┌────────────────────────┐  │
│  │  Streamable HTTP Server  │◄─────────│  ACP Agent Subprocess   │  │
│  │  (server_http.go)        │  HTTP    │  (server_stdio.go)      │  │
│  │  /mcp/http endpoint      │  POST    │  stdin/stdout           │  │
│  │  Real tool handlers      │          │  Proxy tool handlers    │  │
│  └─────────────────────────┘          └───────────┬────────────┘  │
│                                      ▲             │                │
│                                      │             │                │
│                            ┌─────────┴─────────────┴──────────┐   │
│                            │        ACP Agent                  │   │
│                            │  (spawned by agent.Exec)          │   │
│                            │  Reads/writes via stdin/stdout    │   │
│                            └──────────────────────────────────┘   │
│                                                                    │
└──────────────────────────────────────────────────────────────────┘
```

### Role 1: Host (Main Process) — `server_http.go`

The main Orqen process starts an HTTP server (`pkg/service/http/http_service.go`) that registers the MCP endpoint at `/mcp/http`:

```
main.go → service.Start() → http_service.New() → mux.Handle("/mcp/http", mcp.ServerHttp(proj))
```

`ServerHttp()` creates an `mcp.Server`, registers all 10 tools directly (no proxy), and wraps it with `mcp.NewStreamableHTTPHandler`:

```go
func ServerHttp(proj *engine.Project) http.Handler {
    server := createServer()
    addToolProject(server, tnMoveItem, MoveItemHandler, proj)
    // ... 9 more tools
    return mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
        return server
    }, nil)
}
```

Each HTTP request to `/mcp/http` creates an independent MCP session. The handler dispatches `tools/call` requests to the registered tool functions, which operate on the shared `*engine.Project`.

### Role 2: Proxy (Stdio Subprocess) — `server_stdio.go`

When the main process invokes an ACP agent, it spawns a **subprocess** of itself with `--mcp --port=6180 --workitem=<workitem_id> --project=<project_id>`:

```go
// main.go — agent invoker
agent.Exec(cwd, prompt, command, []acp.McpServer{
    {
        Stdio: &acp.McpServerStdio{
            Name:    "orqen",
            Command: orqenExec,
            Args:    []string{"--mcp", "--port=6180", "--job=TASK-0001-xxx"},
        },
    },
})
```

The subprocess enters `StartStdio()`:

```
1. Create a new mcp.Server (empty, no project reference)
2. Create an mcp.Client
3. Connect to the host's Streamable HTTP endpoint via StreamableClientTransport
4. Register all tools as PROXIES — each proxy calls cs.CallTool() on the shared session
5. Run the server over StdioTransport (stdin/stdout)
```

The `*mcp.ClientSession` from step 3 is **shared across all 10 tool proxies**. Each proxy function follows the same pattern:

```go
func sseProxyWithJobId[In InputWithJobId, Out any](tool string, h, cs, workItemID) mcp.ToolHandlerFor[In, Out] {
    return func(ctx, req, input) (result, output, err) {
        input.SetWorkItemID(workItemID)                // auto-inject workitem_id
        result, err = cs.CallTool(ctx, &CallToolParams{Name: tool, Arguments: input})
        if err != nil {
            logError(err, result)                      // log to .ignore/debug/mcp_error.txt
        }
        return
    }
}
```

### Role 3: Agent (ACP Subprocess)

The agent (e.g., Qwen Code, Claude, Gemini) runs as a child process of the stdio subprocess. It communicates via **stdin/stdout** using the MCP protocol over the stdio transport.

The agent's view:

```
1. Agent starts, receives MCP server config via ACP initialization
2. Agent sends tool call request → stdout (JSON-RPC over stdio)
3. Stdio subprocess receives on stdin, proxies to host via HTTP POST
4. Host executes tool, returns result via HTTP response
5. Stdio subprocess writes result → stdout
6. Agent receives tool result, continues execution
```

The agent is **unaware of the proxy**. From its perspective, it calls `orqen_move_item` and gets a result. The transport layer (stdio → HTTP → stdio) is transparent.

### Full Call Sequence

```
Step  Agent (ACP)              Stdio Subprocess           Host Process
────  ────────────────         ──────────────────         ────────────
 1    ── tools/call ──────────▶
      (stdout/stdin)

 2                             ── POST /mcp/http ───────▶
                                  {method: "tools/call",
                                   name: "orqen_move_item",
                                   args: {item_id: 1, ...}}

 3                                                          MoveItemHandler()
                                                              → os.Rename(src, dst)
                                                              → MoveItemOutput{Success: true}

 4                             ◀── HTTP 200 ────────────────
                                  {content: [...],
                                   structuredContent: {success: true}}

 5    ◀── tools/call result ──
       (stdout/stdin)

 6    Agent processes result, continues
```

## Transport: Why Streamable HTTP (not SSE)

The previous implementation used SSE (Server-Sent Events) transport, which has a fundamental flaw in the context of Orqen's architecture:

**The problem with SSE:**
- SSE requires a **hanging GET** connection that stays open for the session lifetime
- Go's `http.Server` has a `WriteTimeout` (60s) that applies to the **entire response duration**
- After 60s of inactivity on the SSE stream, the server kills the connection
- If a tool call arrives after the SSE stream died, the tool **executes successfully** but the response **cannot be delivered** back to the proxy
- The proxy receives `EOF` / `connection closed` and reports a tool failure to the agent

**Why Streamable HTTP fixes it:**
- Each tool call is an **independent** HTTP POST request → POST response pair
- No long-lived streaming connection
- `WriteTimeout` only applies to the actual response body write (milliseconds), not to idle time
- A failed tool call does not affect subsequent calls — no shared connection state
- The agent's subprocess lifecycle is naturally bounded (one turn = one stdio session)

## Tool Registration

Tools are registered in two ways depending on the role:

### Direct Registration (Host — `server_http.go`)

```go
addToolProject(server, tnMoveItem, MoveItemHandler, proj)
```

The handler receives the real `*engine.Project` reference and operates on it directly.

### Proxy Registration (Stdio — `server_stdio.go`)

```go
addToolProjecProxyWithJobId(server, tnMoveItem, MoveItemHandler, session, jobId)
```

The handler is wrapped by `sseProxyWithJobId`, which:
1. Auto-injects the `jobId` into the input via `input.SetWorkItemID(workItemID)`
2. Forwards the call to the host via `cs.CallTool()`
3. Logs errors to `.ignore/debug/mcp_error.txt`
4. Returns the result (or error) back to the agent

### Generic Proxy (`sseProxy`)

For tools that don't need auto-injected `jobId`, the simpler `sseProxy` is used:

```go
addToolProxy(server, tnSomeTool, someHandler, session)
```

This forwards the call as-is without modification.

## JobId Auto-Injection

Most tools accept an optional `module` parameter, but when called from an agent subprocess, the module should be inferred from the agent's current job. The `InputWithJobId` interface enables this:

```go
type InputWithWorkItemID interface {
    SetWorkItemID(workItemID string)
}
```

Tool input structs that need auto-injection implement this method:

```go
func (i *MoveItemInput) SetWorkItemID(workItemID string) {
    i.WorkItemID = &workItemID
}
```

The proxy wrapper calls `SetWorkItemID(jobId)` before forwarding, so the agent never needs to pass `workitem_id` explicitly.

## Error Handling

### Transport Errors

When the proxy's `cs.CallTool()` fails (e.g., HTTP connection error), the error is logged to `.ignore/debug/mcp_error.txt` via `logError()`:

```go
func logError(err error, result *mcp.CallToolResult) {
    // Appends: ERROR, STACK, RESULT (as JSON), and two trailing newlines
}
```

The error is then returned upstream. The SDK's `toolForErr` wrapper converts it into a `CallToolResult` with `IsError: true`, which the agent sees as a tool failure.

### Tool Execution Errors

When a tool handler returns an error (e.g., "module not found"), the handler returns `(nil, output, err)`. The SDK's `toolForErr` wrapper converts this into a `CallToolResult` with the error embedded in the `Content` field — not a protocol error, but a **tool-level** error that the agent can decide how to handle.

### Nil Result Handling

When a tool handler returns `result == nil` but `err == nil` (successful execution with no content), the SDK creates a default `CallToolResult` and populates it from the typed `Out` value via `structuredContent`.

---

**Orqen © 2026 — Execution layer for AI workflows**
