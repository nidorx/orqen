# Getting Started

How to navigate the Orqen codebase, run the engine, and find what you need.

## Prerequisites

- **Go 1.21+**
- **git** CLI
- An ACP-compatible AI agent (e.g., Qwen Code, Claude Code)

## Quick Start

```bash
# Clone the repository
git clone https://github.com/nidorx/orqen.git
cd orqen

# Build
go build -o orqen ./main.go

# Run
./orqen
```

The CLI starts and looks for a project directory containing `.orqen/orqen.yaml`.

### Running in Development

```bash
# Run directly with Go
go run ./main.go

# Run tests
go test ./...

# Run tests for a specific package
go test ./pkg/engine/...
go test ./pkg/mcp/...
```

## Project Structure

```
orqen/
├── main.go                     # CLI entry point — banner, flags, service lifecycle
├── go.mod / go.sum             # Go module dependencies
│
├── pkg/                        # Core packages
│   ├── engine/                 # Workflow engine: Project → Module → Lane → WorkItem
│   │   ├── project.go          # Project lifecycle (start/stop), slot management
│   │   ├── executor.go         # Tick-based execution loop
│   │   ├── module.go           # Module operations, work item iteration
│   │   ├── lane.go             # Lane scanning, file system event handling
│   │   ├── workitem.go         # Work item model, attributes, dependencies
│   │   ├── fsys.go             # File system watcher (fsnotify integration)
│   │   ├── load.go             # Config loading, validation, initialization
│   │   ├── should_ignore*.go   # Ignore rules pipeline (modtime, exists, dependency, attr)
│   │   └── README.md           # Detailed engine architecture
│   │
│   ├── mcp/                    # Model Context Protocol — tools & servers
│   │   ├── server.go           # Server creation, tool registration
│   │   ├── server_stdio.go     # Stdio MCP server (agent-side proxy)
│   │   ├── server_http.go      # Streamable HTTP MCP server (host-side)
│   │   ├── tool_*.go           # Individual tool implementations
│   │   └── README.md           # Detailed MCP architecture
│   │
│   ├── chat/                   # User interaction layer (Telegram, future CLI/Web)
│   │   ├── agent/              # Chat agent flow
│   │   ├── cmd/                # Chat commands
│   │   ├── conn/               # Connection management
│   │   ├── dialog/             # Dialog/conversation management
│   │   ├── mcp/                # Chat-specific MCP tools
│   │   ├── memory/             # Chat memory
│   │   ├── paths/              # Path resolution
│   │   ├── service.go          # Chat service lifecycle
│   │   └── AGENT_FLOW.md       # Agent conversation flow documentation
│   │
│   ├── agent/                  # ACP agent client
│   │   ├── exec.go             # ACP agent execution (spawn, prompt, wait)
│   │   └── client.go           # Generic ACP client
│   │
│   ├── service/                # HTTP and lifecycle services
│   │   ├── service.go          # Service registry (start/stop)
│   │   └── http/
│   │       └── http_service.go # HTTP server with MCP Streamable HTTP endpoint
│   │
│   ├── conf/                   # Configuration utilities
│   ├── cli/                    # CLI i18n and output utilities
│   ├── condition/              # Condition DSL parser & evaluator
│   └── storage/                # Markdown storage utilities
│
├── .orqen/                     # Orqen configuration examples
│   ├── orqen.yaml              # Example workflow configuration
│   └── SKILL.md                # Skill for creating custom workflows
│
└── docs/                       # Documentation
    ├── ARCHITECTURE.md         # System design for developers and AI agents
    ├── CONFIG.md               # Complete configuration reference
    ├── GETTING-STARTED.md      # This file
    └── ABSTRACTIONS.md         # Key abstractions and domain models
```

## Key Files to Know

### Entry Point

| File | Why it matters |
|------|---------------|
| `main.go` | CLI entry point. Handles `--mcp` flag for stdio mode, starts HTTP server, loads projects, manages graceful shutdown. |

### Core Engine

| File | Why it matters |
|------|---------------|
| `pkg/engine/project.go` | Top-level orchestrator. Manages modules, lanes, work items, concurrency slots, and lifecycle. |
| `pkg/engine/executor.go` | Tick-based execution loop. Scans lanes, applies ignore rules, invokes agents. |
| `pkg/engine/lane.go` | Lane operations: work item cache, file system event handling, reference parsing. |
| `pkg/engine/workitem.go` | Work item model: directory-based work units with YAML attributes and dependencies. |
| `pkg/engine/fsys.go` | File system watcher. Keeps work item cache in sync with disk via fsnotify. |
| `pkg/engine/load.go` | Config loading, validation, defaults, prompt generation, initialization. |
| `pkg/engine/README.md` | Detailed engine architecture with diagrams and component documentation. |

### MCP (Model Context Protocol)

| File | Why it matters |
|------|---------------|
| `pkg/mcp/server.go` | Server creation and tool registration. |
| `pkg/mcp/server_stdio.go` | Stdio subprocess entry — proxies agent tool calls to host HTTP server. |
| `pkg/mcp/server_http.go` | Streamable HTTP server — exposes tools directly for remote agents. |
| `pkg/mcp/tool_*.go` | Individual tool implementations (create, move, search, attributes, dependencies). |
| `pkg/mcp/README.md` | Detailed MCP architecture with invocation flow diagrams. |

### Chat (User Interaction)

| File | Why it matters |
|------|---------------|
| `pkg/chat/service.go` | Chat service lifecycle. Currently supports Telegram; CLI and Web interfaces planned. |
| `pkg/chat/AGENT_FLOW.md` | Documentation of the agent conversation flow. |

### Agent (ACP Client)

| File | Why it matters |
|------|---------------|
| `pkg/agent/exec.go` | Spawns ACP agent processes, establishes stdio pipes, sends prompts. |
| `pkg/agent/client.go` | Generic ACP client implementation. |

### Configuration

| File | Why it matters |
|------|---------------|
| `pkg/conf/` | Configuration utilities (version, HTTP server settings). |
| `docs/CONFIG.md` | Complete reference for `orqen.yaml` — every attribute, type, and example. |
| `.orqen/orqen.yaml` | Example workflow configuration. |

## How Orqen Works (Brief)

1. **Load** — Orqen reads `.orqen/orqen.yaml` from a project directory.
2. **Initialize** — Creates lane directories, generates prompt templates, starts file watcher.
3. **Execute** — The executor loop ticks at a configurable interval:
   - Scans modules in order, lanes by priority
   - Applies ignore rules (modtime debounce, exists, dependency, attribute conditions)
   - Invokes ACP agents with synthesized prompts when work is available
4. **Agent acts** — The agent receives a prompt, calls MCP tools (`orqen_item_create`, `orqen_item_move`, etc.), and modifies work items on disk.
5. **Repeat** — The loop continues, picking up changes made by agents or users.

### MCP Transport Modes

| Mode | Purpose | Direction |
|------|---------|-----------|
| **Stdio** | Agent subprocess proxies tool calls to Orqen's HTTP server | Agent → Orqen |
| **Streamable HTTP** | Orqen exposes tools over HTTP for remote agents | Orqen → Agent |

### Work Item Structure

```
{module}/{lane}/{PREFIX}-NNNN-slug/
├── {PREFIX}-NNNN-slug.yaml    # Attributes (key-value metadata)
├── description.md              # Task description
└── ...                         # Artifacts (SUMMARY, FAIL, etc.)
```

## Adding a New Module or Lane

1. Edit `.orqen/orqen.yaml` — add a module or lane under `modules`.
2. Define `name`, `purpose`, `dir`, and optionally `agent_behavior`, `critical_rules`, ignore rules.
3. The engine creates lane directories automatically on next startup.
4. Place work items (or inbox files) in the appropriate directories.

## Running Tests

```bash
# All tests
go test ./...

# Engine tests
go test ./pkg/engine/...

# MCP tool tests
go test ./pkg/mcp/...
```

## Debugging

- Enable verbose logging by checking `pkg/engine/` and `pkg/mcp/` for log statements.
- MCP stdio mode can be debugged by setting `mcp.DEBUG_STDIO = true` in `main.go`.
- Work item state lives on disk — inspect `{module}/{lane}/` directories directly.
