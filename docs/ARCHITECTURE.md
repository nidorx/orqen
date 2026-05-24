# Architecture - Orqen

> **For developers and AI agents.** This document describes the current system design.

## Overview

Orqen is a Go-based workflow orchestration engine that manages projects, modules, lanes, and work items through the **Agent Client Protocol (ACP)**. It serves as the execution layer that coordinates AI agents with structured, filesystem-backed workflows.

## Design Principles

1. **Filesystem-First** - All state (tasks, ADRs, learnings) persists as structured files. No database.
2. **State-Driven** - Everything is explicit. No hidden state, no prompt chaos.
3. **Agent-Agnostic** - Works with any ACP-compatible agent via standardized protocol.
4. **Concurrent** - Multi-module, multi-lane execution with configurable concurrency limits.
5. **Deterministic** - Predictable behavior, auditable outcomes.

## High-Level Architecture

```
┌──────────────────────────────────────────────────────┐
│                    Orqen CLI (main.go)                │
│                                                       │
│  ┌─────────────────┐    ┌──────────────────────────┐  │
│  │  Project Loader  │───▶│    Project Runtime       │  │
│  │  (load.go)       │    │  (project.go, executor.go)│  │
│  └─────────────────┘    └──────────┬───────────────┘  │
│                                    │                   │
│                    ┌───────────────┼───────────────┐   │
│                    ▼               ▼               ▼   │
│         ┌─────────────┐  ┌─────────────────┐  ┌─────────┐ │
│         │  MCP Stdio   │  │  MCP Streamable │  │  Agent  │ │
│         │  Server      │  │  HTTP Server    │  │  Exec   │ │
│         │  (tools)     │  │  (HTTP)         │  │  (ACP)  │ │
│         └─────────────┘  └─────────────────┘  └─────────┘ │
│                                                       │
└────────────────────────┬──────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │    Filesystem         │
              │  .orqen/orqen.yaml    │
              │  .orqen/tasks/lanes/  │
              │  docs/adr/, learnings/│
              └──────────────────────┘
```

## Package Structure

```
orqen/
├── main.go                     # CLI entry point, project loading, service startup
├── go.mod
│
├── pkg/
│   ├── engine/                 # Core domain: Project, Module, Lane, WorkItem, Attributes
│   │   ├── project.go          # Project runtime management (start/stop, invoker)
│   │   ├── executor.go         # Execution loop: scan lanes, invoke agents, track state
│   │   ├── module.go           # Module operations (lane ordering, sequence numbering)
│   │   ├── lane.go             # Lane item scanning, work item lifecycle
│   │   ├── workitem.go         # WorkItem struct, Alias, Attributes, Dependencies, MoveTo
│   │   ├── attributes.go       # Attributes map type with YAML load/save, type accessors
│   │   ├── dependency.go       # Dependency parsing and reference resolution
│   │   └── ...
│   │
│   ├── mcp/                    # Model Context Protocol tools and servers
│   │   ├── server.go           # MCP server creation, tool registration, handler adapter
│   │   ├── server_http.go      # Streamable HTTP server (host: registers tools directly)
│   │   ├── server_stdio.go     # Stdio server (proxy: forwards tool calls to host via HTTP)
│   │   ├── tool_workitem.go    # Get work item context
│   │   ├── tool_workitem_create.go  # Create work items in lanes
│   │   ├── tool_workitem_move.go    # Move items between lanes
│   │   ├── tool_workitem_search.go  # Search work items with condition DSL
│   │   ├── tool_workitem_attrs_set.go    # Set work item attributes
│   │   ├── tool_workitem_attrs_del.go    # Delete work item attributes
│   │   ├── tool_workitem_attrs_schema.go # Schema of observed attributes
│   │   ├── tool_workitem_dependencies.go # Check dependency status
│   │   ├── tool_lane_list.go   # List available lanes
│   │   ├── tool_project_info.go# Get project information
│   │   ├── tool_fs_*.go        # Filesystem tools (move, copy, list, tree, find, grep, diff)
│   │   └── utils.go            # Shared utilities (findTargetModuleBy)
│   │
│   ├── agent/                  # ACP agent client
│   │   ├── exec.go             # ACP agent execution: connect, initialize, prompt
│   │   └── client.go           # Generic ACP client implementation
│   │
│   ├── service/                # HTTP and lifecycle services
│   │   ├── service.go          # Service registry (start/stop)
│   │   └── http/
│   │       └── http_service.go # HTTP server with MCP Streamable HTTP endpoints
│   │
│   ├── conf/                   # Configuration utilities
│   └── storage/                # Markdown storage utilities
│
├── cmd/                        # CLI commands (future)
├── .orqen/                     # Orqen configuration
│   ├── orqen.yaml              # Example workflow configuration
│   └── SKILL.md                # Skill for creating custom workflows
│
└── docs/                       # Documentation
    ├── ARCHITECTURE.md         # This file
    ├── BRANDING.md             # Visual identity and design system
    └── ...                     # Module prompt templates
```

## Core Domain Models (`pkg/engine`)

### Project

The top-level configuration loaded from `.orqen/orqen.yaml`.

```go
type Project struct {
    DirAbs    string     // Absolute path to project directory
    Agents    Agent      // Agent configuration (default client, commands)
    Execution *Execution // Max agents, sleep interval
    Modules   []*Module  // Workflow modules (task, adr, learning, etc.)
}
```

### Module

A self-contained workflow unit with its own lanes, prompts, and artifact directory.

```go
type Module struct {
    Name        string   // Module name (unique within project), e.g., "task", "adr", "learning"
    Dir         string   // Relative artifact directory from project root
    Prefix      string   // Prefix for work item names (e.g., TASK, ADR, LKN; default "WI")
    Order       []string // Lane priority order for work scanning
    Lanes       []*Lane  // Lanes belonging to this module
    Prompt      string   // Synthesized HEADER.md prompt (injected into every invocation)
    ExtraPrompt string   // Additional context appended to header
    Project     *Project // Reference to parent project
    Hooks       *HookBindings // Pre/post hook bindings for this module
}
```

### Lane

A stage within a module's workflow.

```go
type Lane struct {
    Name               string   // Lane name (e.g., "inbox", "doing", "review")
    Purpose            string   // Description injected into agent prompts
    Agent              string   // Optional override of default agent for this lane
    MaxAgents          int      // Concurrency limit for this lane (0 = unlimited)
    Artifacts          []string // Artifact types the agent may create (SUMMARY, FAIL, etc.)
    UserAction         string   // Short label for expected user action (e.g., "approve")
    AgentBehavior      []string // Ordered steps the agent should follow
    CriticalRules      []string // Absolute rules that must never be violated
    IgnoreIfAttr       string   // Condition expression: skip items whose attributes match
    IgnoreIfModtime    int      // Skip items modified within this many seconds
    IgnoreIfExists     []string // Skip this lane if referenced lanes have items
    IgnoreIfNotExists  []string // Skip if referenced lanes/files don't exist
    IgnoreIfDependency []string // Skip items with dependencies in referenced lanes
    ExtraPrompt        string   // Lane-specific context appended to prompt
    AllowedNext        []string // Restricts which lanes items can move to from this lane (default: next lane in Module.Order)
    McpServers         []string // MCP server names to inject for this lane
    Module             *Module  // Reference to parent module
    Hooks              *HookBindings // Lane-level hook bindings (can exclude module-level hooks)
    Schedule           *LaneSchedule // Optional schedule configuration for execution windows
}
```

### WorkItem

A unit of work discovered in a lane directory during scanning.

```go
type WorkItem struct {
    ID         string     `json:"id"`         // Unique identifier (hash of Seq+Name)
    Seq        int        `json:"seq"`         // Sequential number extracted from directory/file name
    Name       string     `json:"name"`        // Directory/file name (e.g., WI-001-create-project)
    Files      []string   `json:"files"`       // Files within the work item directory
    Lane       *Lane      `json:"-"`           // Parent lane reference (not serialized)
    InProgress bool       `json:"in_progress"` // Whether an agent is currently processing this item
    Attributes Attributes `json:"attributes"`  // Key-value metadata attached to the work item
    ModTime    time.Time  `json:"mod_time"`    // Most recent modification time of the item

    attributesModTime time.Time `json:"-"` // Internal: tracks when attributes were last loaded from YAML
}
```

#### Attributes

WorkItem metadata is stored in the `Attributes` field, which is a `map[string]any` persisted as YAML.
The attribute file is named `<module_prefix>-<seq>.yaml` (e.g., `task-0001.yaml`) and lives inside
the work item directory.

**Important:** The `Dependencies` field that existed in earlier versions is now stored inside
`Attributes` under the key `"dependencies"` as a `[]string`. This design keeps the `WorkItem`
struct lean and allows flexible querying/filtering by arbitrary attributes.

When serializing a `WorkItem` for external consumption (e.g., MCP tool responses), the `Alias()`
method returns a `WorkItemAlias` that flattens relevant context:

```go
type WorkItemAlias struct {
    *WorkItem                         // Embedded original WorkItem (with Attributes)
    Lane         string   `json:"lane"`         // Lane name (convenience field)
    Module       string   `json:"module"`       // Module name (convenience field)
    Dependencies []string `json:"dependencies"` // Extracted from Attributes["dependencies"]
}
```

The `Dependencies` slice in `WorkItemAlias` is populated by reading
`item.Attributes.StringArray("dependencies")`. This is a **denormalized view** — the actual
dependency data lives only in `Attributes`, not as a separate field on `WorkItem`.

#### WorkItem Methods

| Method | Description |
|--------|-------------|
| `Alias()` | Returns a `*WorkItemAlias` with lane name, module name, and dependencies extracted from Attributes for external serialization. |
| `RelativePath()` | Returns the work item's relative path within the module (e.g., `"04_prioritized/WI-0002-hooks"`). |
| `AttributesLoad()` | Reloads attributes from the YAML file on disk if the file has been modified since the last load. No-op if `Seq <= 0`. |
| `AttributesSave(other)` | Merges the provided attributes into the work item and persists to YAML. No-op if `Seq <= 0` or `other` is empty. The `"dependencies"` key is preserved and never deleted during this operation. |
| `AttributesDel(keys)` | Deletes the specified attribute keys from the work item and persists to YAML. The `"dependencies"` key is **protected** and will not be deleted even if included in `keys`. |
| `Dependencies()` | Returns an `iter.Seq[*WorkItem]` that iterates over all work items this item depends on. Each dependency reference in `Attributes["dependencies"]` is parsed (format: `[module:]SEQ`) and resolved across modules if needed. |
| `Dependents()` | Returns an `iter.Seq[*WorkItem]` that iterates over all work items that depend on **this** item. Scans all work items in the project to find reverse dependencies within the same module. |
| `MoveTo(laneName)` | Moves the work item directory from the current lane to the target lane within the same module. Returns an error if the lane doesn't exist. Includes a brief wait loop to ensure the move is observed. |

## Execution Model

### The Loop

The execution cycle in `executor.go`:

```
1. cleanupCompleted()  - Remove finished invocations from tracking
2. check slots         - Verify project and lane concurrency limits
3. scan lanes          - Iterate modules in order, lanes in priority order
4. filter items        - Apply ignore rules (ignore_if_exists, ignore_if_dependency)
5. invoke agent        - Call agentInvoker with synthesized prompt
6. track invocation    - Register InvocationHandle for async completion
7. sleep               - Wait for sleep_interval_seconds, then repeat
```

### Agent Invocation

When `agentInvoker` is called (in `project.go`), it assembles a prompt from three sources:

```
1. Module Prompt (mod.Prompt)
   - HEADER.md content with artifact naming conventions, extra_prompt context

2. Lane Prompt (lan.Prompt)
   - Workflow definition, agent behavior steps, critical rules, lane-specific extra_prompt

3. Pre-Execution Context
   - Recommended action, related resource files
```

The assembled prompt is passed to `agent.Exec()` along with the agent command and MCP server configuration.

### Concurrency

- **Project-level**: `execution.max_agents` limits total concurrent agents across all modules
- **Lane-level**: Each lane has `max_agents` for fine-grained control
- The executor checks both limits before invoking an agent

## MCP (Model Context Protocol) — `pkg/mcp`

Orqen exposes workflow operations as MCP tools that agents can call during execution. The `mcp` package is the **bridge between AI agents and Orqen's filesystem-backed project state**. It delegates all project operations to `pkg/engine` — it is purely a **protocol layer**.

### Three Roles

Orqen uses MCP in three distinct roles:

| Role | File | Description |
|------|------|-------------|
| **Host** | `server_http.go` | Main process registers tools directly on the `*engine.Project` |
| **Proxy** | `server_stdio.go` | Subprocess proxies tool calls to the Host over HTTP |
| **Agent** | external | ACP agent communicates via stdin/stdout, unaware of the proxy |

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
└───────────────────────────────────────────────────────────────────┘
```

### Stdio Flow (Primary — Agent Subprocess)

```
main.go:
  1. Start HTTP server with MCP Streamable HTTP endpoint (/mcp/http)
  2. Load project, set agent invoker
  3. Start execution loop

executor.go → agentInvoker:
  4. When invoking an agent, spawn agent process with:
     --mcp --port=6180 --workitem=<workitem_id> --project=<project_id>

  5. Agent subprocess enters StartStdio() (server_stdio.go):
     a. Creates mcp.Server (no project reference)
     b. Creates mcp.Client, connects to host via StreamableClientTransport
     c. Registers all tools as PROXIES — each calls cs.CallTool() on the shared session
     d. Runs server over StdioTransport (stdin/stdout)

  6. Agent calls tools via stdin/stdout → proxy forwards to host via HTTP POST
  7. Host executes tool, returns HTTP response → proxy writes result to stdout
```

### Streamable HTTP Flow (Remote Agents)

```
server_http.go:
  1. Creates mcp.Server with all tools registered directly (real handlers)
  2. Returns http.Handler via mcp.NewStreamableHTTPHandler

Remote agent:
  3. Connects to http://<orqen-host>:<port>/mcp/http
  4. Calls tools directly — no proxy layer
```

### Why Streamable HTTP (not SSE)

The previous SSE transport used a hanging GET connection for server-to-client messages.
The Go `http.Server` `WriteTimeout` (60s) applies to the entire response lifetime of the
hanging GET, causing the SSE stream to be killed after 60 seconds of inactivity. This
resulted in tool calls succeeding on the server side but losing their response during
transport back to the proxy.

Streamable HTTP resolves this: each tool call is an independent HTTP request/response pair.
There is no long-lived streaming connection, so `WriteTimeout` only applies to the actual
response write (milliseconds), not to idle waiting time. Tool calls are isolated — a failure
in one does not affect subsequent calls.

### MCP Tools

The package exposes **18 MCP tools** across five categories:

#### Work Item Operations

| Tool Name | Handler | Description |
|-----------|---------|-------------|
| `workitem` | `WorkitemHandler` | Returns the current work item, lane, module, and project context. Requires `workitem_id`. |
| `workitem_move` | `WorkitemMoveHandler` | Moves a work item directory from one lane to another within a module. |
| `workitem_create` | `WorkitemCreateHandler` | Creates a new work item in a specific lane (directory + YAML file). |
| `workitem_search` | `WorkitemSearchHandler` | Searches work items in a module or lane, filtered by a SQL-like condition DSL. |

#### Work Item Attribute Operations

| Tool Name | Handler | Description |
|-----------|---------|-------------|
| `workitem_attrs_set` | `WorkitemAttrsSetHandler` | Merges attributes into a work item and persists to YAML. |
| `workitem_attrs_del` | `WorkitemAttrsDelHandler` | Removes specified attribute keys. The `"dependencies"` key is protected. |
| `workitem_attrs_schema` | `WorkitemAttrSchemaHandler` | Returns all observed attributes and their unique values across a module. |

#### Work Item Dependencies

| Tool Name | Handler | Description |
|-----------|---------|-------------|
| `workitem_dependencies` | `WorkitemDependenciesHandler` | Checks dependency status for the current work item, resolved to actual items. |

#### Project & Lane Operations

| Tool Name | Handler | Description |
|-----------|---------|-------------|
| `lane_list` | `LaneListHandler` | Lists all lanes in a module with configuration, purpose, item counts, and availability. |
| `project_info` | `ProjectInfoHandler` | Returns full project structure: modules, lanes, item counts, and configuration. |

#### Filesystem Operations

| Tool Name | Handler | Description |
|-----------|---------|-------------|
| `fs_move` | `FsMoveHandler` | Move a file or directory within the project filesystem. |
| `fs_copy` | `FsCopyHandler` | Copy a file or directory within the project filesystem. |
| `fs_list` | `FsListHandler` | List contents of a directory. |
| `fs_tree` | `FsTreeHandler` | Display a directory tree structure. |
| `fs_find` | `FsFindHandler` | Find files by name pattern. |
| `fs_grep` | `FsGrepHandler` | Search file contents by regex pattern. |
| `fs_diff` | `FsDiffHandler` | Compare two files and show differences. |

### Tool Registration

Tools are registered via `addTool` (host) and `addToolProxy` (stdio proxy):

```go
// Host — direct registration with project reference
addTool(server, tnWorkitemMove, WorkitemMoveHandler, proj)

// Stdio — proxy registration, auto-injects workitem_id
addToolProxy(server, tnWorkitemMove, WorkitemMoveHandler, session, workItemID)
```

The generic `projectHandler2MCP` adapter converts typed project handlers to the MCP SDK's `ToolHandlerFor` interface:

```go
type ToolProjectHandler[In, Out any] func(
    ctx context.Context, request *mcp.CallToolRequest, input In, proj *engine.Project,
) (result *mcp.CallToolResult, output Out, err error)
```

### WorkItemID Auto-Injection

Tool input structs implement `InputWithWorkItemID` to receive automatic injection:

```go
type InputWithWorkItemID interface {
    SetWorkItemID(workItemID string)
}
```

The proxy wrapper calls `input.SetWorkItemID(workItemID)` before forwarding, so the agent never needs to pass `workitem_id` explicitly.

### Module Resolution

Tools use `findTargetModuleBy()` to resolve the target module:

1. **Explicit `module` parameter** — use directly
2. **`workitem_id` resolution** — scan modules/lanes to find the item's module
3. **Single module fallback** — if the project has only one module
4. **Ambiguous** — return `nil` (handler reports error)

### Files

| File | Responsibility |
|------|----------------|
| `server.go` | Server creation, tool registration map, handler adapter (`projectHandler2MCP`) |
| `server_http.go` | Host HTTP server entry point (`ServerHttp`) — registers all tools directly |
| `server_stdio.go` | Stdio subprocess entry point (`StartStdio`) — registers all tools as proxies |
| `tool_workitem*.go` | Work item tools (get, create, move, search, attrs set/del/schema, dependencies) |
| `tool_lane_list.go` | Lane listing tool |
| `tool_project_info.go` | Project info tool |
| `tool_fs_*.go` | Filesystem tools (move, copy, list, tree, find, grep, diff) |
| `utils.go` | Shared utilities (`findTargetModuleBy`) |

## ACP (Agent Client Protocol)

ACP is the protocol through which Orqen communicates with AI agents.

### Agent Execution (`pkg/agent/exec.go`)

```
1. Spawn agent process: exec.Command(agent, args...)
2. Establish stdio pipes: stdin, stdout
3. Initialize ACP connection with capabilities (filesystem, terminal)
4. Create new session with MCP server configuration
5. Send assembled prompt via Prompt()
6. Wait for completion, stream updates
```

### Agent Capabilities

The ACP initialization declares:
- **FileSystem**: Read and write text files
- **Terminal**: Execute shell commands

### MCP Server Injection

Each agent invocation receives an MCP server configuration pointing to Orqen's Streamable HTTP endpoint:

```go
acp.McpServer{
    Stdio: &acp.McpServerStdio{
        Name:    "orqen",
        Command: orqen_executable,
        Args:    []string{"--mcp", "--port=6180", "--workitem=<workitem_id>", "--project=<project_id>"},
    },
}
```

This gives the executing agent access to all 18 MCP tools (`workitem`, `workitem_create`, `workitem_move`, `fs_*`, etc.) during its invocation. The agent's subprocess acts as a proxy, forwarding tool calls to the host's HTTP endpoint.

## Filesystem Layout

### Work Item Naming

```
LANE_DIR/
  MOD_TYPE-NNNN-slug/          # Work item directory
    MOD_TYPE-NNNN.md           # Primary work item file
    MOD_TYPE-NNNN-SUMMARY.md   # Summary artifact (optional)
    MOD_TYPE-NNNN-FAIL.md      # Failure artifact (optional)
    MOD_TYPE-NNNN.yaml         # Attributes file (key-value metadata, including dependencies)
```

- `MOD_TYPE`: Uppercase module name (TASK, ADR, LEARNING)
- `NNNN`: Zero-padded 4-digit sequence number
- `slug`: Kebab-case description

### Inbox Lane

Inbox lanes use **single files** (not directories) as work items:
- Files with `.md` or `.txt` extensions
- Non-empty files only
- Must not have been modified in the last 60 seconds (debounce)

### Module Prompts

During initialization (`load.go → initialize()`), Orqen creates prompt templates:
- `HEADER.md` - Module-level context (artifact naming, extra_prompt)
- `NN_LANE_NAME.md` - Per-lane workflow definition (agent behavior, critical rules)

These are generated from embedded defaults unless user-customized versions already exist.

## Error Handling

Errors are explicit and surfaced at multiple levels:

| Level | Error Type | Behavior |
|-------|-----------|----------|
| Config loading | `fmt.Errorf` wrapped | Prevents project from starting |
| Lane scanning | Silent (returns nil) | Lane treated as empty |
| Agent invocation | Logged, continues loop | Other items still processed |
| MCP tool calls | Returned to agent as JSON | Agent decides how to handle |

## Testing

- **Unit tests** in `pkg/project/` (project_test.go) validate loading, validation, defaults
- **MCP tool tests** in `pkg/mcp/` (*_test.go) validate each tool's input/output contract
- **Integration tests** would validate end-to-end execution with mock agents

---

**Orqen © 2026 - Execution layer for AI workflows**