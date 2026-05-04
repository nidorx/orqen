# Architecture — Orqen

> **For developers and AI agents.** This document describes the current system design.

## Overview

Orqen is a Go-based workflow orchestration engine that manages projects, modules, lanes, and work items through the **Agent Client Protocol (ACP)**. It serves as the execution layer that coordinates AI agents with structured, filesystem-backed workflows.

## Design Principles

1. **Filesystem-First** — All state (tasks, ADRs, learnings) persists as structured files. No database.
2. **State-Driven** — Everything is explicit. No hidden state, no prompt chaos.
3. **Agent-Agnostic** — Works with any ACP-compatible agent via standardized protocol.
4. **Concurrent** — Multi-module, multi-lane execution with configurable concurrency limits.
5. **Deterministic** — Predictable behavior, auditable outcomes.

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
│   ├── project/                # Core domain: Project, Module, Lane, WorkItem
│   │   ├── load.go             # Config loading, validation, defaults, initialization
│   │   ├── project.go          # Project runtime management (start/stop, invoker)
│   │   ├── executor.go         # Execution loop: scan lanes, invoke agents, track state
│   │   ├── module.go           # Module operations (lane ordering, sequence numbering)
│   │   ├── lane.go             # Lane item scanning, work item lifecycle
│   │   └── types.go            # Core structs (Agent, Execution, WorkItem, InvocationHandle)
│   │
│   ├── mcp/                    # Model Context Protocol tools and servers
│   │   ├── server.go           # MCP server creation, tool registration, streamable HTTP proxy
│   │   ├── server_stdio.go     # Stdio MCP server (agent-side: connects to Streamable HTTP server)
│   │   ├── server_http.go      # Streamable HTTP MCP server (host-side: exposes tools over HTTP)
│   │   ├── tool_create_item.go # Create work items in lanes
│   │   ├── tool_move_item.go   # Move items between lanes
│   │   ├── tool_list_items.go  # List items in a lane
│   │   ├── tool_status.go      # Get current work item context by workitem_id
│   │   ├── tool_dependencies.go# Manage item dependencies (DEP_XXX files)
│   │   ├── tool_scan_module.go # Scan module lanes for items
│   │   ├── tool_schema.go      # Get project schema
│   │   ├── tool_next_sequence.go # Get next sequence number
│   │   ├── tool_list_lanes.go  # List available lanes
│   │   ├── tool_project_info.go# Get project information
│   │   └── utils.go            # Shared utilities (findModuleByJobID)
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

## Core Domain Models

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
    Name        string   // e.g., "task", "adr", "learning"
    Dir         string   // Relative artifact directory
    Order       []string // Lane priority order for work scanning
    Lanes       []*Lane
    Prompt      string   // Synthesized HEADER.md prompt (injected into every invocation)
    ExtraPrompt string   // Additional context appended to header
}
```

### Lane

A stage within a module's workflow.

```go
type Lane struct {
    Name               string   // e.g., "inbox", "doing", "review"
    Purpose            string   // Description injected into agent prompts
    AgentBehavior      []string // Ordered steps the agent should follow
    Artifacts          []string // Types of files the agent may create (SUMMARY, FAIL, etc.)
    CriticalRules      []string // Absolute rules that must never be violated
    IgnoreIfExists     []string // Skip this lane if referenced lanes have items
    IgnoreIfDependency []string // Skip items with dependencies in referenced lanes
    ExtraPrompt        string   // Lane-specific context appended to prompt
    MaxAgents          int      // Concurrency limit for this lane
}
```

### WorkItem

A unit of work discovered in a lane directory during scanning.

```go
type WorkItem struct {
    ID           int         // Numeric ID extracted from directory name
    Name         string      // Directory name (e.g., TASK-0001-refactor-auth)
    Files        []string    // Files within the work item directory
    Lane         *Lane       // Parent lane
    JobID        string      // Current agent invocation ID (if being processed)
    InProgress   bool        // Whether an agent is currently working on this
    Dependencies []*WorkItem // Items this work item depends on (via DEP_XXX files)
}
```

## Execution Model

### The Loop

The execution cycle in `executor.go`:

```
1. cleanupCompleted()  — Remove finished invocations from tracking
2. check slots         — Verify project and lane concurrency limits
3. scan lanes          — Iterate modules in order, lanes in priority order
4. filter items        — Apply ignore rules (ignore_if_exists, ignore_if_dependency)
5. invoke agent        — Call agentInvoker with synthesized prompt
6. track invocation    — Register InvocationHandle for async completion
7. sleep               — Wait for sleep_interval_seconds, then repeat
```

### Agent Invocation

When `agentInvoker` is called (in `project.go`), it assembles a prompt from three sources:

```
1. Module Prompt (mod.Prompt)
   — HEADER.md content with artifact naming conventions, extra_prompt context

2. Lane Prompt (lan.Prompt)
   — Workflow definition, agent behavior steps, critical rules, lane-specific extra_prompt

3. Pre-Execution Context
   — Recommended action, related resource files
```

The assembled prompt is passed to `agent.Exec()` along with the agent command and MCP server configuration.

### Concurrency

- **Project-level**: `execution.max_agents` limits total concurrent agents across all modules
- **Lane-level**: Each lane has `max_agents` for fine-grained control
- The executor checks both limits before invoking an agent

## MCP (Model Context Protocol)

Orqen exposes workflow operations as MCP tools that agents can call during execution.

### Two Transport Modes

| Mode | Purpose | Direction |
|------|---------|-----------|
| **Stdio** | Agent-side: agent connects to Orqen's Streamable HTTP server | Agent → Orqen |
| **Streamable HTTP** | Host-side: Orqen exposes tools over HTTP for remote agents | Orqen → Agent |

### Stdio Flow (Primary)

```
main.go:
  1. Start HTTP server with MCP Streamable HTTP endpoint (/mcp/http)
  2. Load project, set agent invoker
  3. Start execution loop

executor.go → agentInvoker:
  4. When invoking an agent, spawn agent process with:
     --mcp --port=6180 --workitem=<workitem_id> --project=<project_id>
  5. Agent process starts MCP Stdio server (server_stdio.go)
  6. Stdio server connects to Orqen's Streamable HTTP endpoint
  7. Agent calls tools (orqen_status, orqen_create_item, etc.) via HTTP proxy

server_stdio.go:
  8. Receives tool calls from agent over stdin/stdout
  9. Proxies calls to Streamable HTTP server at http://127.0.0.1:<port>/mcp/http
  10. Returns results back to agent
```

### Streamable HTTP Flow (Remote Agents)

```
http_service.go:
  1. Exposes /mcp/http endpoint via chain router
  2. Registers MCP tools directly (no proxy) — tools operate on the loaded Project

Remote agent:
  3. Connects to http://<orqen-host>:<port>/mcp/http
  4. Calls tools directly (orqen_list_items, orqen_move_item, etc.)
```

### Why Streamable HTTP (not SSE)

The previous SSE transport used a hanging GET connection for server-to-client messages,
with a separate POST endpoint for client-to-server messages. The Go `http.Server`
`WriteTimeout` (60s) applies to the entire response lifetime of the hanging GET, causing
the SSE stream to be killed after 60 seconds of inactivity. This resulted in tool calls
succeeding on the server side but losing their response during transport back to the proxy.

Streamable HTTP resolves this: each tool call is an independent HTTP request/response pair.
There is no long-lived streaming connection, so `WriteTimeout` only applies to the actual
response write (milliseconds), not to idle waiting time. Tool calls are isolated — a failure
in one does not affect subsequent calls.

### MCP Tools

| Tool | Purpose |
|------|---------|
| `orqen_status` | Get current work item context by workitem_id |
| `orqen_create_item` | Create a new work item in a lane (directory + .md file) |
| `orqen_move_item` | Move a work item directory from one lane to another |
| `orqen_list_items` | List all work items in a lane |
| `orqen_list_lanes` | List available lanes in a module |
| `orqen_scan_module` | Scan all lanes in a module for items |
| `orqen_dependencies` | Manage dependency files (DEP_XXX) |
| `orqen_project_info` | Get project metadata |
| `orqen_schema` | Get the project configuration schema |

### JobId Resolution

Most tools accept an optional `workitem_id` parameter. When provided, Orqen uses it to:
- Resolve the calling agent's current module (via `findModuleByJobID`)
- Scope operations to the correct module without requiring explicit `module` parameter
- Track which work item is being processed

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

Each agent invocation receives an MCP server configuration pointing to Orqen's own SSE endpoint:

```go
acp.McpServer{
    Stdio: &acp.McpServerStdio{
        Name:    "orqen",
        Command: orqen_executable,
        Args:    ["--mcp", "--port=6180", "--workitem=<workitem_id>", "--project=<project_id>"],
    },
}
```

This gives the executing agent access to Orqen's MCP tools (`orqen_create_item`, `orqen_move_item`, etc.) during its invocation.

## Filesystem Layout

### Work Item Naming

```
LANE_DIR/
  MOD_TYPE-NNNN-slug/          # Work item directory
    MOD_TYPE-NNNN.md           # Primary work item file
    MOD_TYPE-NNNN-SUMMARY.md   # Summary artifact (optional)
    MOD_TYPE-NNNN-FAIL.md      # Failure artifact (optional)
    DEP_NNNN                   # Dependency file (references another item)
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
- `HEADER.md` — Module-level context (artifact naming, extra_prompt)
- `NN_LANE_NAME.md` — Per-lane workflow definition (agent behavior, critical rules)

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

**Orqen © 2026 — Execution layer for AI workflows**
