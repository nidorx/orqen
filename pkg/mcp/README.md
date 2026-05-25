# Architecture — pkg/mcp

> **For developers and AI agents.** This document describes the MCP package internals and invocation flow.

## Responsibility

The `mcp` package is the **bridge between AI agents and Orqen's filesystem-backed project state**. It:

1. **Exposes operations as MCP tools** — Each tool (create item, move item, search items, manage attributes, etc.) is a well-defined function with typed input/output, discoverable by any MCP-compatible agent.
2. **Serves tools directly for agents** — When agents connect via HTTP, the Streamable HTTP handler dispatches tool calls without a proxy layer.

The package does **not** manage project state — it delegates to `pkg/engine`. It is purely a **protocol layer**: receive MCP tool calls, execute the corresponding project operation, return structured results.

## Files

| File | Responsibility |
|------|----------------|
| `server.go` | Server creation (`createServer`), shared tool map (`tools`), and handler adapter (`projectHandler2MCP`) |
| `server_http.go` | Entry point for the host's HTTP server (`ServerHttp`). Creates an `http.Handler` using `mcp.NewStreamableHTTPHandler` with all tools registered directly |
| `tool_workitem*.go` | Work item operations: get, create, move, search, attributes (set/del/schema), dependencies |
| `tool_lane_list.go` | Lane listing operation |
| `tool_project_info.go` | Project structure inspection |
| `tool_fs_*.go` | Filesystem operations: copy, move, list, tree, find, grep, diff |
| `tool_dynamic.go` | Dynamic user-defined tools from `orqen.yaml` configuration |
| `*_test.go` | Comprehensive test coverage for all tools with shared test helpers (`test_helpers_test.go`) |

## Tools

The package exposes **18+ MCP tools** across five categories:

### Work Item Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnWorkitem` | `item` | `WorkitemHandler` | Returns the current work item, lane, module, and project context for a running agent job. |
| `tnWorkitemMove` | `workitem_move` | `WorkitemMoveHandler` | Moves a work item directory from one lane to another within a module. Updates internal state to reflect the new lane position. |
| `tnWorkitemCreate` | `workitem_create` | `WorkitemCreateHandler` | Creates a new work item in a specific lane of a module. Creates the directory following naming conventions (MOD_TYPE-NNNN-name) and an empty `.yaml` file. |
| `tnWorkitemSearch` | `workitem_search` | `WorkitemSearchHandler` | Searches for work items in a module or lane, optionally filtered by a condition SQL-like DSL string. Returns full WorkItem objects. |

### Work Item Attribute Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnWorkitemAttrsSet` | `workitem_attrs_set` | `WorkitemAttrsSetHandler` | Updates attributes on a work item. Merges the provided attributes into the work item's existing attributes and persists them to disk. |
| `tnWorkitemAttrsDel` | `workitem_attrs_del` | `WorkitemAttrsDelHandler` | Removes specified attribute keys from a work item and persists the changes to disk. The "dependencies" key cannot be removed. |
| `tnWorkitemAttrsSchema` | `workitem_attrs_schema` | `WorkitemAttrSchemaHandler` | Returns all observed workitem attributes and their unique values (domains) across all workitems in a module. Use this to understand what metadata fields exist. |

### Work Item Dependencies

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnWorkitemDependencies` | `workitem_dependencies` | `WorkitemDependenciesHandler` | Checks dependency status for the current work item. Resolves them to actual work items with their status. |

### Filesystem Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnFsCopy` | `fs_copy` | `FsCopyHandler` | Copies a file or directory within a module. |
| `tnFsMove` | `fs_move` | `FsMoveHandler` | Moves or renames a file or directory within a module. |
| `tnFsList` | `fs_list` | `FsListHandler` | Lists the contents of a directory within a module. |
| `tnFsTree` | `fs_tree` | `FsTreeHandler` | Displays a recursive tree view of a directory within a module. |
| `tnFsFind` | `fs_find` | `FsFindHandler` | Finds files by name or glob pattern within a module. |
| `tnFsGrep` | `fs_grep` | `FsGrepHandler` | Searches file contents by regex pattern within a module. |
| `tnFsDiff` | `fs_diff` | `FsDiffHandler` | Shows the diff between the current and committed state of a file. |

### Project & Lane Operations

| Tool Constant | Tool Name | Handler | Description |
|---------------|-----------|---------|-------------|
| `tnLaneList` | `lane_list` | `LaneListHandler` | Lists all lanes in a module with their configuration, purpose, item counts, and availability. Use this to understand lane structure before creating or moving items. |
| `tnProjectInfo` | `project_info` | `ProjectInfoHandler` | Returns the full project structure: modules, lanes, item counts, and configuration. Use this to understand the overall project layout. |

### Dynamic Tools (User-Defined)

Dynamic tools are registered from the `tools` section in `orqen.yaml`. Each tool definition creates an MCP tool with a schema based on the `args` field. Tools execute shell commands with parameter injection via `$param_name` wildcards.

**Configuration Example:**
```yaml
tools:
  my_tool:
    command: ["./script.sh", "--a", "$param_a", "--b", "$param_b"]
    windows: ["script.bat", "--a", "$param_a", "--b", "$param_b"]
    timeout: 30
    description: "Runs my custom script"
    args:
      param_a: "First parameter"
      param_b: "Second parameter"
```

**Fields:**
- `command`: Default command array (executed when no OS-specific override matches)
- `windows`/`darwin`/`linux`: OS-specific command overrides
- `timeout`: Execution timeout in seconds (default: 30)
- `description`: Tool description shown to MCP agents
- `args`: Map of parameter names to descriptions (all treated as required strings)

**Parameter Injection:**
Parameters are injected via exact `$param_name` wildcard substitution in command arguments. Only exact tokens (e.g., `$param_a`) are replaced — partial matches like `prefix_$param_a` are NOT supported.

**Registration:**
Dynamic tools are registered via `RegisterDynamicTools()` in `server_http.go` after filesystem tools. Tool names that conflict with built-in tools are ignored with a warning.

## Invocation Flow

### The Host Role

Orqen's MCP server runs in a single role — the **Host**:

```
┌───────────────────────────────────────────────────────────────────┐
│                    Orqen Main Process                             │
│                                                                   │
│  ┌─────────────────────────┐          ┌────────────────────────┐  │
│  │  Streamable HTTP Server │◄─────────│  ACP Agent (external)  │  │
│  │  (server_http.go)       │  HTTP    │  (spawned by agent.Exec)│  │
│  │  /mcp/http endpoint     │  POST    │  Connects via HTTP      │  │
│  │  Real tool handlers     │          │  Calls tools directly   │  │
│  └─────────────────────────┘          └────────────────────────┘  │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

### Role: Host (Main Process) — `server_http.go`

The main Orqen process starts an HTTP server that registers the MCP endpoint:

```go
func ServerHttp(proj *engine.Project) http.Handler {
    server := createServer()

    // Register all 18 tools directly with project reference
    addTool(server, tnWorkitem, WorkitemHandler, proj)
    addTool(server, tnWorkitemMove, WorkitemMoveHandler, proj)
    addTool(server, tnWorkitemCreate, WorkitemCreateHandler, proj)
    addTool(server, tnWorkitemSearch, WorkitemSearchHandler, proj)
    addTool(server, tnWorkitemAttrsSet, WorkitemAttrsSetHandler, proj)
    addTool(server, tnWorkitemAttrsDel, WorkitemAttrsDelHandler, proj)
    addTool(server, tnWorkitemAttrsSchema, WorkitemAttrSchemaHandler, proj)
    addTool(server, tnWorkitemDependencies, WorkitemDependenciesHandler, proj)
    addTool(server, tnLaneList, LaneListHandler, proj)
    addTool(server, tnProjectInfo, ProjectInfoHandler, proj)

    // Filesystem tools
    addTool(server, tnFsMove, FsMoveHandler, proj)
    addTool(server, tnFsCopy, FsCopyHandler, proj)
    addTool(server, tnFsList, FsListHandler, proj)
    addTool(server, tnFsTree, FsTreeHandler, proj)
    addTool(server, tnFsFind, FsFindHandler, proj)
    addTool(server, tnFsGrep, FsGrepHandler, proj)
    addTool(server, tnFsDiff, FsDiffHandler, proj)

    // Dynamic tools from orqen.yaml
    RegisterDynamicTools(server, proj)

    return mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
        return server
    }, nil)
}
```

Each HTTP request to `/mcp/http` creates an independent MCP session. The handler dispatches `tools/call` requests to the registered tool functions, which operate on the shared `*engine.Project`.

### Agent Invocation

When the main process invokes an ACP agent, it provides an HTTP MCP server configuration:

```go
// single_project_service.go — agent invoker
orqenMcp := acp.McpServer{
    Http: &acp.McpServerHttpInline{
        Name:    "orqen",
        Headers: make([]acp.HttpHeader, 0),
        Type:    "http",
        Url:     fmt.Sprintf("http://127.0.0.1:%d/mcp/http/%s", orqenPort, proj.Id),
    },
}
agent.Exec(..., []acp.McpServer{orqenMcp}, ...)
```

The agent connects directly to Orqen's HTTP endpoint and calls tools without any proxy layer.

## Transport: Why Streamable HTTP (not SSE)

The previous implementation used SSE (Server-Sent Events) transport, which has a fundamental flaw:

**The problem with SSE:**
- SSE requires a **hanging GET** connection that stays open for the session lifetime
- Go's `http.Server` has a `WriteTimeout` (60s) that applies to the **entire response duration**
- After 60s of inactivity on the SSE stream, the server kills the connection
- If a tool call arrives after the SSE stream died, the tool **executes successfully** but the response **cannot be delivered** back to the client

**Why Streamable HTTP fixes it:**
- Each tool call is an **independent** HTTP POST request → POST response pair
- No long-lived streaming connection
- `WriteTimeout` only applies to the actual response body write (milliseconds), not to idle time
- A failed tool call does not affect subsequent calls — no shared connection state

## Tool Registration

Tools are registered via `addTool`:

```go
addTool(server, tnWorkitemMove, WorkitemMoveHandler, proj)
```

The handler receives the real `*engine.Project` reference and operates on it directly.

The `addTool` function is a generic wrapper that adapts project handlers to the MCP SDK's `ToolHandlerFor` interface.


## Module Resolution

Tools use `findTargetModuleBy()` to resolve the target module:

1. **Explicit module parameter** — If `module` is provided, use it directly
2. **Single module fallback** — If there's only one module in the project, use it
3. **Ambiguous** — If none of the above succeed, return `nil` (handler reports error)

This allows agents to call tools without explicitly specifying the module when the context is clear.

## Error Handling

### Transport Errors

When an HTTP request fails (e.g., connection error), the error is returned upstream. The SDK's `toolForErr` wrapper converts it into a `CallToolResult` with `IsError: true`, which the agent sees as a tool failure.

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
- Happy path operations (create, move, search, attribute management, filesystem operations)
- Error scenarios (nil project, missing parameters, invalid values)
- Edge cases (ambiguous module resolution, invalid conditions, kebab-case validation)
- File system verification (directory creation, file existence, YAML content)

---

**Orqen © 2026 — Execution layer for AI workflows**