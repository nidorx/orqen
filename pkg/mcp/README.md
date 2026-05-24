# Architecture — pkg/mcp

> **For developers and AI agents.** This document describes the MCP package internals and invocation flow.

## Responsibility

The `mcp` package is the **bridge between AI agents and Orqen's filesystem-backed project state**. It:

1. **Exposes operations as MCP tools** — Each tool (create item, move item, search items, manage attributes, etc.) is a well-defined function with typed input/output, discoverable by any MCP-compatible agent.
2. **Proxies agent-side calls to the host process** — When an agent runs in a subprocess, the stdio server forwards tool calls to the main process over HTTP.
3. **Serves tools directly for remote agents** — When agents connect directly (e.g., running alongside Orqen), the Streamable HTTP handler dispatches tool calls without a proxy layer.

The package does **not** manage project state — it delegates to `pkg/engine`. It is purely a **protocol layer**: receive MCP tool calls, execute the corresponding project operation, return structured results.

## Files

| File | Responsibility |
|------|----------------|
| `server.go` | Server creation (`createServer`), tool registration map (`tools`), and handler adapter (`projectHandler2MCP`) |
| `server_stdio.go` | Entry point for the stdio subprocess (`StartStdio`). Creates an MCP client that connects to the host's Streamable HTTP endpoint and registers all tools as proxies |
| `server_http.go` | Entry point for the host's HTTP server (`ServerHttp`). Creates an `http.Handler` using `mcp.NewStreamableHTTPHandler` with all tools registered directly |
| `tool_*.go` | Individual tool implementations — each defines `Input`/`Output` structs, a tool name constant, an `init()` registration, and a handler function |
| `utils.go` | Shared utilities (e.g., `findTargetModuleBy` for module resolution) |
| `*_test.go` | Comprehensive test coverage for all tools with shared test helpers (`test_helpers_test.go`) |

## Tools

The package exposes **11 MCP tools** for work item and project management:

### Work Item Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnItem` | `orqen_item` | `ItemHandler` | Returns the current work item, lane, module, and project context for a running agent job. Requires `workitem_id`. |
| `tnItemMove` | `orqen_item_move` | `ItemMoveHandler` | Moves a work item directory from one lane to another within a module. Updates internal state to reflect the new lane position. |
| `tnItemCreate` | `orqen_item_create` | `ItemCreateHandler` | Creates a new work item in a specific lane of a module. Creates the directory following naming conventions (MOD_TYPE-NNNN-name) and an empty `.yaml` file. |
| `tnItemSearch` | `orqen_item_search` | `ItemSearchHandler` | Searches for work items in a module or lane, optionally filtered by a condition SQL-like DSL string. Returns full WorkItem objects. |

### Work Item Attribute Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnItemAttrsSet` | `orqen_item_attrs_set` | `ItemAttrsSetHandler` | Updates attributes on a work item. Merges the provided attributes into the work item's existing attributes and persists them to disk. |
| `tnItemAttrsDel` | `orqen_item_attrs_del` | `ItemAttrsDelHandler` | Removes specified attribute keys from a work item and persists the changes to disk. The "dependencies" key cannot be removed. |
| `tnItemAttrsSchema` | `orqen_item_attrs_schema` | `ItemAttrSchemaHandler` | Returns all observed workitem attributes and their unique values (domains) across all workitems in a module. Use this to understand what metadata fields exist. |

### Work Item Dependencies

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnItemDependencies` | `orqen_item_dependencies` | `ItemDependenciesHandler` | Checks dependency status for the current work item. Resolves them to actual work items with their status. |

### Project & Lane Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnLaneList` | `orqen_lane_list` | `LaneListHandler` | Lists all lanes in a module with their configuration, purpose, item counts, and availability. Use this to understand lane structure before creating or moving items. |
| `tnProjectInfo` | `orqen_project_info` | `ProjectInfoHandler` | Returns the full project structure: modules, lanes, item counts, and configuration. Use this to understand the overall project layout. |

## Invocation Flow

### The Three Roles

Orqen uses MCP in two distinct roles within a single process:

```
┌───────────────────────────────────────────────────────────────────┐
│                    Orqen Main Process                             │
│                                                                   │
│  ┌─────────────────────────┐          ┌────────────────────────┐  │
│  │  Streamable HTTP Server │◄─────────│  ACP Agent Subprocess  │  │
│  │  (server_http.go)       │  HTTP    │  (server_stdio.go)     │  │
│  │  /mcp/http endpoint     │  POST    │  stdin/stdout          │  │
│  │  Real tool handlers     │          │  Proxy tool handlers   │  │
│  └─────────────────────────┘          └───────────┬────────────┘  │
│                                         ▲         │               │
│                                         │         │               │
│                            ┌────────────┴─────────┴───────────┐   │
│                            │        ACP Agent                 │   │
│                            │  (spawned by agent.Exec)         │   │
│                            │  Reads/writes via stdin/stdout   │   │
│                            └──────────────────────────────────┘   │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

### Role 1: Host (Main Process) — `server_http.go`

The main Orqen process starts an HTTP server that registers the MCP endpoint:

```go
func ServerHttp(proj *engine.Project) http.Handler {
    server := createServer()
    
    // Register all 11 tools directly with project reference
    addTool(server, tnItemStatus, ItemStatusHandler, proj)
    addTool(server, tnItemMove, ItemMoveHandler, proj)
    addTool(server, tnItemCreate, ItemCreateHandler, proj)
    addTool(server, tnItemSearch, ItemSearchHandler, proj)
    addTool(server, tnItemAttrsSet, ItemAttrsSetHandler, proj)
    addTool(server, tnItemAttrsDel, ItemAttrsDelHandler, proj)
    addTool(server, tnItemAttrsSchema, ItemAttrSchemaHandler, proj)
    addTool(server, tnItemDependencies, ItemDependenciesHandler, proj)
    addTool(server, tnLaneList, LaneListHandler, proj)
    addTool(server, tnProjectInfo, ProjectInfoHandler, proj)
    
    return mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
        return server
    }, nil)
}
```

Each HTTP request to `/mcp/http` creates an independent MCP session. The handler dispatches `tools/call` requests to the registered tool functions, which operate on the shared `*engine.Project`.

### Role 2: Proxy (Stdio Subprocess) — `server_stdio.go`

When the main process invokes an ACP agent, it spawns a **subprocess** of itself:

```go
// main.go — agent invoker
agent.Exec(cwd, prompt, command, []acp.McpServer{
    {
        Stdio: &acp.McpServerStdio{
            Name:    "orqen",
            Command: orqenExec,
            Args:    []string{"--mcp", "--port=6180", "--workitem=TASK-0001-xxx"},
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

The `*mcp.ClientSession` from step 3 is **shared across all tool proxies**. Each proxy function follows the same pattern:

```go
func sseProxy[In InputWithWorkItemID, Out any](
    tool string, _ mcp.ToolHandlerFor[In, Out], cs *mcp.ClientSession, workItemID string,
) mcp.ToolHandlerFor[In, Out] {
    return func(ctx, req, input) (result, output, err) {
        input.SetWorkItemID(workItemID)                // auto-inject workitem_id
        result, err = cs.CallTool(ctx, &CallToolParams{Name: tool, Arguments: input})
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
                                   args: {workitem_seq: 1, ...}}

 3                                                          ItemMoveHandler()
                                                              → item.MoveTo("doing")
                                                              → ItemMoveOutput{Success: true}

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
addTool(server, tnMoveItem, ItemMoveHandler, proj)
```

The handler receives the real `*engine.Project` reference and operates on it directly.

### Proxy Registration (Stdio — `server_stdio.go`)

```go
addToolProxy(server, tnItemMove, ItemMoveHandler, session, workItemID)
```

The handler is wrapped by `sseProxy`, which:
1. Auto-injects the `workItemID` into the input via `input.SetWorkItemID(workItemID)`
2. Forwards the call to the host via `cs.CallTool()`
3. Returns the result (or error) back to the agent

### Generic Pattern

All tools follow the same registration pattern in both host and proxy modes. The `addTool` and `addToolProxy` functions are generic wrappers that adapt the project handlers to the MCP SDK's `ToolHandlerFor` interface.

## WorkItemID Auto-Injection

Most tools accept an optional `module` parameter, but when called from an agent subprocess, the module should be inferred from the agent's current job. The `InputWithWorkItemID` interface enables this:

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

The proxy wrapper calls `SetWorkItemID(workItemID)` before forwarding, so the agent never needs to pass `workitem_id` explicitly.

## Module Resolution

Tools use `findTargetModuleBy()` to resolve the target module:

1. **Explicit module parameter** — If `module` is provided, use it directly
2. **WorkItemID resolution** — If `workitem_id` is provided, scan all modules/lanes to find the item's module
3. **Single module fallback** — If there's only one module in the project, use it
4. **Ambiguous** — If none of the above succeed, return `nil` (handler reports error)

This allows agents to call tools without explicitly specifying the module when the context is clear.

## Error Handling

### Transport Errors

When the proxy's `cs.CallTool()` fails (e.g., HTTP connection error), the error is returned upstream. The SDK's `toolForErr` wrapper converts it into a `CallToolResult` with `IsError: true`, which the agent sees as a tool failure.

### Tool Execution Errors

When a tool handler returns an error (e.g., "module not found"), the handler returns `(nil, output, err)`. The SDK's `toolForErr` wrapper converts this into a `CallToolResult` with the error embedded in the `Content` field — not a protocol error, but a **tool-level** error that the agent can decide how to handle.

### Nil Result Handling

When a tool handler returns `result == nil` but `err == nil` (successful execution with no content), the SDK creates a default `CallToolResult` and populates it from the typed `Out` value via `structuredContent`.

## Testing

All tools have comprehensive test coverage in `*_test.go` files. The test suite uses shared helpers:

- **`setupTestProject(t)`** — Creates a temporary project with `task` and `adr` modules, multiple lanes, and sample work items
- **`callHandler(...)`** — Generic wrapper to invoke handlers and extract typed output
- **`ptr(v)`** — Helper to create pointer values for optional fields

Tests cover:
- Happy path operations (create, move, search, attribute management)
- Error scenarios (nil project, missing parameters, invalid values)
- Edge cases (ambiguous module resolution, invalid conditions, kebab-case validation)
- File system verification (directory creation, file existence, YAML content)

---

**Orqen © 2026 — Execution layer for AI workflows**
