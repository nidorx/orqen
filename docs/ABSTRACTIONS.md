# Abstractions

Key abstractions and domain models in Orqen.

## Design Philosophy

Orqen follows **filesystem-first, state-driven** design principles:

- **Filesystem as source of truth** — All state (tasks, decisions, learnings) persists as structured files. No database.
- **Explicit state** — No hidden state, no prompt chaos. Everything is auditable.
- **Agent-agnostic** — Works with any ACP-compatible agent via standardized protocol.
- **Deterministic** — Predictable behavior, reproducible outcomes.

The full system architecture is documented in [ARCHITECTURE.md](./ARCHITECTURE.md).

## Domain Model Hierarchy

```
Project
 └── Modules[]
      └── Lanes[]          (sorted by Module.Order)
           └── WorkItems   (cached, directory-based)
                └── Attributes (key-value metadata, YAML)
```

Each level holds a back-reference to its parent, enabling any component to traverse upward without passing context explicitly.

---

## Project

The top-level orchestrator. Represents a single Orqen-managed project directory.

**Configuration file:** `.orqen/orqen.yaml`

```go
type Project struct {
    Id        string      // directory hash (unique cache key)
    DirAbs    string      // absolute path to project root
    Agents    Agent       // agent configuration
    Execution *Execution  // concurrency and timing settings
    Modules   []*Module   // functional groupings
}
```

**Runtime responsibilities:**
- Module lookup: `GetModule(name)` returns a module by name.
- Cross-module work item lookup: `GetWorkItemById(id)` scans all modules.
- Slot management: `ActiveAgentCount()` and `HasAvailableSlot()` enforce concurrency limits.
- Lifecycle: `Start()` launches the executor; `Stop()` cancels context and waits for completion.

### Agent Configuration

Defines which AI agents are available and how to invoke them.

```yaml
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]
```

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `default` | string | Yes | Name of the default agent client. Must match a key under `clients`. |
| `clients.<name>.command` | []string | Yes | Shell command (executable + arguments) to invoke the agent. Must include flags for autonomous mode and ACP support. |

### Execution Configuration

Controls runtime settings across all modules.

```yaml
execution:
  max_agents: 10                   # Max concurrent agents (0 = unlimited)
  sleep_interval_seconds: 60       # Seconds between work cycles
```

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_agents` | int | 0 (unlimited) | Maximum concurrent agents across all modules. |
| `sleep_interval_seconds` | int | 60 | Polling frequency of the execution loop. |

---

## Module

A self-contained workflow unit grouping related lanes within a dedicated directory.

```go
type Module struct {
    Name        string   // e.g., "task", "adr", "learning"
    Dir         string   // relative artifact directory
    Order       []string // lane priority order for work scanning
    Prefix      string   // work item name prefix (e.g., "TASK", "ADR")
    Lanes       []*Lane
    Prompt      string   // generated module-level prompt (HEADER.md)
    ExtraPrompt string   // additional context appended to header
    Project     *Project // back-reference
}
```

**Key behaviors:**
- **Lane ordering:** `GetLanesInOrder()` returns lanes sorted by `Order`, with remaining lanes appended.
- **Work item iteration:** `WorkItems()` iterates all items across all lanes via `iter.Seq2`.
- **Lookup by sequence:** `GetWorkItemBySeq(seq)` uses an LRU cache.
- **Thread-safe creation:** `TxNewWorkItem(fn)` finds the max sequence number, then calls `fn(nextSeq)` within a mutex.

### How `dir` and `prefix` Work

The `dir` attribute determines **where** the module is written on disk. The `prefix` attribute determines **how** work items are named.

```
Project root:     /home/user/my-project/
Module dir:       "tasks"
Module prefix:    "TASK"
Result:           /home/user/my-project/tasks/
                                    └── 01_inbox/
                                    └── 02_doing/
                                        └── TASK-0001-implement-auth/
                                            ├── TASK-0001.yaml
                                            └── description.md
```

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | — | Unique module name. Used in cross-module lane references (e.g., `task.doing`). |
| `dir` | string | — | Directory path **relative to project root** where lanes are stored. |
| `prefix` | string | `WI` | Prefix for work item names. Normalized to uppercase. |
| `order` | []string | declaration order | Lane priority order for work selection. |
| `extra_prompt` | string | — | Additional context injected into the module's `HEADER.md`. |

---

## Lane

A stage within a module's workflow (e.g., `inbox`, `doing`, `review`, `done`).

```go
type Lane struct {
    Dir                string   // relative dir name (e.g., "01_inbox")
    Name               string   // lane identifier
    Agent              string   // override default agent name
    Purpose            string   // description injected into agent prompts
    MaxAgents          int      // per-lane concurrency cap (0 = unlimited)
    Artifacts          []string // artifact types the agent may create
    UserAction         string   // expected user action label
    AgentBehavior      []string // numbered steps the agent must follow
    CriticalRules      []string // absolute rules the agent must obey
    IgnoreIfAttr       string   // condition DSL (e.g., "priority > 3")
    IgnoreIfExists     []string // skip if any items exist in referenced lanes
    IgnoreIfNotExists  []string // skip if referenced lanes/files are empty
    IgnoreIfDependency []string // skip if item's dependencies are in referenced lanes
    ExtraPrompt        string   // additional context
    AllowedNext        []string // restricts which lanes items can move to from this lane (default: next lane in order)
    Module             *Module  // back-reference
}
```

### Lane Attributes

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | — | Lane name. Directories are created as `NN_name` (e.g., `01_inbox`). Must be kebab-case. |
| `purpose` | string | Yes | — | Description of the lane's purpose. **Injected into the agent prompt**. |
| `agent` | string | No | Project default | Overrides the default agent client for this lane. |
| `max_agents` | int | No | 0 (unlimited) | Maximum concurrent agents in this lane. |
| `artifacts` | []string | No | — | Artifact types the agent **may** create (e.g., `SUMMARY`, `FAIL`). |
| `user_action` | string | No | — | Label describing expected user action (e.g., `"approve"`, `"review"`). |
| `agent_behavior` | []string | No | — | **Sequential steps** the agent follows. Each item becomes a numbered step. |
| `critical_rules` | []string | No | — | **Absolute, non-negotiable rules**. Rendered in a separate, highlighted section. |
| `extra_prompt` | string | No | — | Additional context injected **after** `agent_behavior`. Not step-by-step instructions. |
| `allowed_next` | []string | No | Next lane in order | Restricts which lanes work items can be moved to from this lane. Use `"*"` to allow any lane. |

### Ignore Rules

| Attribute | Type | Description |
|-----------|------|-------------|
| `ignore_if_exists` | []string | Skip this lane if **any items exist** in referenced lanes. |
| `ignore_if_not_exists` | []string | Skip this lane if **no items or files exist** in referenced lanes. |
| `ignore_if_dependency` | []string | Skip a work item if it has **dependencies** in referenced lanes. |
| `ignore_if_attr` | string | Skip work items whose **attributes** match the condition DSL. |

#### Reference Format for Ignore Rules

| Format | Example | Meaning |
|--------|---------|---------|
| Lane name (same module) | `"draft"` | Check lane `draft` within the same module |
| Cross-module | `"adr.draft"` | Check lane `draft` of module `adr` |
| File pattern | `"file:draft.*.md"` | Check if any file matches the glob pattern |
| Cross-module file | `"file:adr.draft.ADR-*.md"` | Check items in `adr.draft` for files matching the pattern |

---

## WorkItem

A unit of work represented as a directory on disk with a YAML metadata file.

```go
type WorkItem struct {
    ID         string     // hash(Seq + Name)
    Seq        int        // sequential number within the lane
    Name       string     // directory name (e.g., "TASK-001-create-project")
    Files      []string   // relative file paths within the directory
    Lane       *Lane      // back-reference
    InProgress bool       // agent currently executing
    Attributes Attributes // key-value metadata
    ModTime    time.Time  // most recent file modification time
}
```

### Directory Structure on Disk

```
{module}/{lane}/{PREFIX}-NNNN-slug/
├── {PREFIX}-NNNN.yaml                  # metadata (attributes)
└── {PREFIX}-NNNN-{ARCTIFACT}.md        # artifacts
```

### Naming Convention

- `PREFIX`: Uppercase module prefix (e.g., `TASK`, `ADR`, `LEARNING`)
- `NNNN`: Zero-padded 4-digit sequence number
- `slug`: Kebab-case description

### Inbox Special Handling

Inbox lanes additionally accept standalone `.md` and `.txt` files (not just directories), with `Seq = 0`. This allows raw notes and ideas to enter the system without formal structure.

---

## Attributes

A typed key-value store (`map[string]any`) backed by YAML files.

```go
type Attributes map[string]any
```

**Type-Aware Accessors:**
- `String(key)`, `Int(key)`, `Float(key)`, `Bool(key)` — perform Go type conversion.
- `StringArray(key)` — returns `[]string`.
- `Get(key)` / `Set(key, value)` / `Has(key)` / `Delete(key)` — basic CRUD.
- `LoadFromYAML(path)` / `SaveToYAML(path)` — file I/O.

Attributes power the ignore rules pipeline, condition DSL evaluation, and agent prompt generation.

---

## Condition DSL

The condition DSL (used in `ignore_if_attr` and `FilterWorkItems`) is a SQL-like expression language evaluated against a work item's attributes.

### Supported Operators

| Category | Operators |
|----------|-----------|
| Comparison | `=`, `!=`, `>`, `>=`, `<`, `<=` |
| Pattern | `LIKE`, `CONTAINS`, `PREFIX`, `SUFFIX` |
| Set | `IN`, `NOT IN`, `ANY_OF`, `ALL_OF` |
| Existence | `EXISTS`, `IS NULL`, `IS NOT NULL` |
| Range | `BETWEEN ... AND ...`, `HAS_LENGTH` |
| Logical | `AND`, `OR`, `NOT`, `( )` |

### Examples

```yaml
# Skip items with high priority
ignore_if_attr: "priority > 3"

# Skip items tagged as urgent or critical
ignore_if_attr: "tags ANY_OF ('urgent', 'critical')"

# Skip items with a specific status pattern
ignore_if_attr: "status LIKE '^blocked'"

# Complex condition with grouping
ignore_if_attr: "(priority > 3 AND status = 'active') OR tags ANY_OF ('blocked')"

# Skip items without a reviewer assigned
ignore_if_attr: "reviewer IS NULL"
```

---

## Execution Model

### The Tick Loop

The executor runs a continuous loop:

```
1. cleanupCompleted()  - Remove finished invocations
2. check slots         - Verify project and lane concurrency limits
3. scan lanes          - Iterate modules in order, lanes by priority
4. filter items        - Apply ignore rules pipeline
5. invoke agent        - Call agentInvoker with synthesized prompt
6. track invocation    - Register InvocationHandle for async completion
7. sleep               - Wait for sleep_interval_seconds, then repeat
```

### Agent Invocation Prompt

The assembled prompt combines three sources:

```
1. Module Prompt (mod.Prompt)
   - HEADER.md content with artifact naming conventions, extra_prompt context

2. Lane Prompt (lan.Purpose + AgentBehavior + CriticalRules + ExtraPrompt)
   - Workflow definition, numbered steps, absolute rules, lane-specific context

3. Pre-Execution Context
   - Recommended action, related resource files
```

### Concurrency

Two-level control:

| Level | Field | Description |
|-------|-------|-------------|
| **Project** | `execution.max_agents` | Maximum total `InProgress` items across ALL modules. |
| **Lane** | `Lane.MaxAgents` | Maximum `InProgress` items within this lane. |

Both checks must pass for an item to be invoked.

---

## MCP (Model Context Protocol)

Orqen exposes workflow operations as MCP tools that agents can call during execution.

### Tools

| Tool | Description |
|------|-------------|
| `item` | Get current work item context by workitem_id |
| `item_create` | Create a new work item in a lane |
| `item_move` | Move a work item between lanes |
| `item_search` | Search work items with condition DSL filter |
| `item_attrs_set` | Update attributes on a work item |
| `item_attrs_del` | Remove attribute keys from a work item |
| `item_attrs_schema` | Get observed attribute schema across a module |
| `item_dependencies` | Check dependency status for a work item |
| `lane_list` | List all lanes in a module |
| `project_info` | Get full project structure |

### Transport Modes

| Mode | Purpose | Direction |
|------|---------|-----------|
| **Stdio** | Agent subprocess proxies tool calls to Orqen's HTTP server | Agent → Orqen |
| **Streamable HTTP** | Orqen exposes tools over HTTP for remote agents | Orqen → Agent |

Streamable HTTP was chosen over SSE because each tool call is an independent request/response pair — no long-lived connection that can be killed by `WriteTimeout`, and a failed tool call does not affect subsequent calls.

---

## Prompt System

The prompt system generates agent prompts by combining:

1. **HEADER.md** — A shared preamble (embedded template or project-specific override).
2. **Lane Purpose** — A description of the lane's role.
3. **Agent Behavior** — Numbered steps (`1.`, `2.`, `3.` ...) that define the agent's workflow.
4. **Critical Rules** — Absolute rules the agent must follow.
5. **Extra Prompt** — Additional context from module or lane configuration.

Generated prompts are written to `{module}/{lane}/prompts/PROMPT.md` and read by the invoker when calling the agent.

---

## Chat System (User Interaction)

Orqen provides a chat-based interface for user interaction, currently supporting **Telegram** with **CLI** and **Web** interfaces planned for the future.

### Architecture

```
pkg/chat/
├── service.go        # Chat service lifecycle
├── agent/            # Chat agent flow (conversation management)
├── cmd/              # Chat commands (user-facing commands)
├── conn/             # Connection management (Telegram, future: CLI, Web)
├── dialog/           # Dialog/conversation state management
├── mcp/              # Chat-specific MCP tools
├── memory/           # Chat memory (persistent conversation context)
└── paths/            # Path resolution for chat resources
```

### Agent Flow

The chat agent flow is documented in `pkg/chat/AGENT_FLOW.md`. It describes how user messages are processed, how conversations are maintained, and how the chat system integrates with the Orqen engine.

---

## File System Watcher (Fsys)

Wraps `fsnotify.Watcher` to provide real-time awareness of file system changes.

**Key behaviors:**
- **Recursive watching:** All lane directories and subdirectories are watched.
- **Event synthesis:** When a directory is created, `Create` events are synthesized for all existing files within it.
- **Cache sync:** Events are dispatched to the matching lane's `onFsysUpdate(ev)`, which maintains the `workItemsByID` cache.
- **Cold-start:** On initialization, `FsysOpCreate` events are synthesized for all existing files to populate the cache.

---

## Key Design Patterns

| Pattern | Where | Why |
|---------|-------|-----|
| **Hierarchical back-references** | `WorkItem → Lane → Module → Project` | Any component can traverse upward without explicit context |
| **Tick-based execution** | `Executor.Run()` | Simple, predictable loop — no complex scheduling |
| **Filesystem as source of truth** | `Fsys → onFsysUpdate → workItemsByID` | Work items are directories, not database rows |
| **LRU caching** | tinylfu caches for work items | Fast lookups without scanning files on every tick |
| **Short-circuit ignore pipeline** | `shouldIgnore*` chain | Composable filtering — first matching rule skips the item |
| **Two-level concurrency** | Project + Lane `MaxAgents` | Prevents per-lane overload and system-wide saturation |
| **Directory-as-WorkItem** | `{PREFIX}-NNNN-{name}/` | Human-readable, git-friendly, watchable |
| **Embedded prompts** | `//go:embed prompts/HEADER.md` | Sensible defaults, overrideable per-project |