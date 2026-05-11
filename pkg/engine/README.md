# Engine Architecture

The `pkg/engine` package is the core orchestrator of Orqen. It manages a hierarchical model of **Projects → Modules → Lanes → WorkItems**, drives autonomous agent execution via a tick-based loop, and maintains real-time awareness of the file system through an event-driven watcher.

---

## Table of Contents

- [High-Level Overview](#high-level-overview)
- [Component Hierarchy](#component-hierarchy)
- [Core Components](#core-components)
  - [Project](#project)
  - [Module](#module)
  - [Lane](#lane)
  - [WorkItem](#workitem)
  - [Attributes](#attributes)
- [Execution System](#execution-system)
  - [Executor](#executor)
  - [InvocationHandle](#invocationhandle)
  - [Slot Management](#slot-management)
- [File System Watcher (Fsys)](#file-system-watcher-fsys)
  - [Event Flow](#event-flow)
  - [Work Item Cache Sync](#work-item-cache-sync)
- [Ignore Rules Pipeline](#ignore-rules-pipeline)
  - [shouldIgnoreTimeAfter](#shouldignoretimeafter)
  - [shouldIgnoreIfExists](#shouldignoreifexists)
  - [shouldIgnoreIfNotExists](#shouldignoreifnotexists)
  - [shouldIgnoreIfDependency](#shouldignoreifdependency)
  - [shouldIgnoreIfAttr](#shouldignoreifattr)
- [Project Loading & Initialization](#project-loading--initialization)
- [Lane References & Resolution](#lane-references--resolution)
- [Condition DSL](#condition-dsl)
- [Prompt System](#prompt-system)
- [Memory Store (WIP)](#memory-store-wip)
- [Concurrency & Thread Safety](#concurrency--thread-safety)
- [Key Design Patterns](#key-design-patterns)
- [External Dependencies](#external-dependencies)

---

## High-Level Overview

Orqen's engine operates as a **file-system-driven autonomous loop**. The configuration lives in `.orqen/orqen.yaml` within a project directory. The engine reads this configuration, builds an in-memory hierarchy, watches the file system for changes, and autonomously invokes AI agents to process work items as they appear.

```
┌─────────────────────────────────────────────────────┐
│                        Project                      │
│  ┌───────────────────────────────────────────────┐  │
│  │                    Executor (tick loop)       │  │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  │  │
│  │  │ Module 1  │  │ Module 2  │  │ Module N  │  │  │
│  │  │ ┌───────┐ │  │ ┌───────┐ │  │ ┌───────┐ │  │  │
│  │  │ │ Lane1 │ │  │ │ Lane1 │ │  │ │ Lane1 │ │  │  │
│  │  │ │  WI   │ │  │ │  WI   │ │  │ │  WI   │ │  │  │
│  │  │ │  WI   │ │  │ │  WI   │ │  │ │  WI   │ │  │  │
│  │  │ └───────┘ │  │ └───────┘ │  │ └───────┘ │  │  │
│  │  │ ┌───────┐ │  │ ┌───────┐ │  │ ┌───────┐ │  │  │
│  │  │ │ Lane2 │ │  │ │ Lane2 │ │  │ │ Lane2 │ │  │  │
│  │  │ └───────┘ │  │ └───────┘ │  │ └───────┘ │  │  │
│  │  └───────────┘  └───────────┘  └───────────┘  │  │
│  └───────────────────────────────────────────────┘  │
│                           ▲                         │
│                           │ events                  │
│                     ┌─────┴─────┐                   │
│                     │   Fsys    │ ← fsnotify        │
│                     └───────────┘   watcher         │
└─────────────────────────────────────────────────────┘
```

---

## Component Hierarchy

```
Project
 └── Modules[]
      └── Lanes[]          (sorted by Module.Order)
           └── WorkItems   (cached in tinylfu LRU)
                └── Attributes (key-value, persisted as YAML)
```

Each level holds a back-reference to its parent:
- `WorkItem.Lane` → `Lane`
- `Lane.Module` → `Module`
- `Module.Project` → `Project`

This enables any component to traverse upward without passing context explicitly.

---

## Core Components

### Project

**File:** `project.go`

The top-level orchestrator. Represents a single Orqen-managed project directory.

```go
type Project struct {
    Id        string      // directory hash (unique cache key)
    DirAbs    string      // absolute path to project root
    Agents    Agent       // agent configuration
    Execution *Execution  // concurrency and timing settings
    Modules   []*Module   // functional groupings

    // Runtime state (not serialized)
    mu       sync.Mutex
    fsys     *Fsys
    memory   *store.Store
    running  bool
    executor *Executor
    invoker  AgentInvoker
}
```

**Key Responsibilities:**
- **Module lookup:** `GetModule(name)` returns a module by name.
- **Cross-module work item lookup:** `GetWorkItemById(id)` scans all modules.
- **Iterators:** `WorkItems()` yields every work item across all modules via `iter.Seq2[*Module, *WorkItem]`.
- **Slot management:** `ActiveAgentCount()` and `HasAvailableSlot()` enforce project-level concurrency limits.
- **Lifecycle:** `Start()` launches the executor goroutine; `Stop()` cancels context and waits for completion. Both are idempotent.

**AgentInvoker:** The `invoker` field is a callback (`func(prompt string, item *WorkItem) error`) responsible for actually calling the AI agent (e.g., spawning an LLM CLI process). It is set via `WithInvoker(invoker)` and invoked by the executor for each eligible work item.

---

### Module

**File:** `module.go`

A functional grouping within a project (e.g., "backend", "frontend", "docs"). Contains multiple lanes that represent workflow stages.

```go
type Module struct {
    Name        string
    Order       []string              // lane names in priority order
    Prefix      string                // work item prefix (e.g., "TASK", "ADR")
    Lanes       []*Lane
    Prompt      string                // generated prompt content
    Dir         string                // relative directory path
    DirAbs      string                // absolute directory path
    Project     *Module               // back-reference
    DirPrompts  string                // prompts directory path
    ExtraPrompt string                // additional context for prompts

    // Runtime caches
    mu               sync.Mutex
    workItemsBySeq   *tinylfu.SyncCacheT[*WorkItem]  // seq → item
    workItemsStashed *tinylfu.SyncCacheT[*WorkItem]  // temporarily removed items
}
```

**Key Responsibilities:**
- **Lane ordering:** `GetLanesInOrder()` returns lanes sorted by `Order`, with any remaining lanes appended.
- **Work item iteration:** `WorkItems()` iterates all items across all lanes. `FilterWorkItems(cond)` filters using the condition DSL.
- **Lookup by sequence:** `GetWorkItemBySeq(seq)` uses an LRU cache (`workItemsBySeq`).
- **Schema introspection:** `Schema()` scans all work items and collects observed front matter fields with their types and values — useful for understanding the shape of work item metadata.
- **Thread-safe creation:** `TxNewWorkItem(fn)` finds the max sequence number, then calls `fn(nextSeq)` within a mutex, preventing ID collisions under concurrent writes.
- **Stash/Unstash:** Internal mechanism to temporarily remove items from the main cache and restore them, used during lane transitions.

---

### Lane

**File:** `lane.go`

A workflow stage within a module (e.g., "inbox", "doing", "review", "done"). Lanes define *where* work items are and *how* agents should behave when processing them.

```go
type Lane struct {
    Dir                string   // relative dir name (e.g., "01_inbox")
    Name               string   // lane identifier
    Agent              string   // override default agent name
    DirAbs             string   // absolute path
    Prompt             string   // generated prompt content
    Purpose            string   // description injected into agent prompts
    MaxAgents          int      // per-lane concurrency cap (0 = unlimited)
    Artifacts          []string // artifact types the agent may create
    UserAction         string   // expected user action label
    ExtraPrompt        string   // additional context
    AgentBehavior      []string // numbered steps the agent must follow
    CriticalRules      []string // absolute rules the agent must obey
    IgnoreIfAttr       string   // condition DSL (e.g., "priority > 3")
    IgnoreIfExists     []string // skip if any items exist in referenced lanes
    IgnoreIfNotExists  []string // skip if referenced lanes/files are empty
    IgnoreIfDependency []string // skip if item's dependencies are in referenced lanes
    Module             *Lane    // back-reference

    workItemsByID *tinylfu.SyncCacheT[*WorkItem]  // hash ID → item
}
```

**Key Responsibilities:**
- **Work item cache:** `workItemsByID` is a tinylfu LRU cache mapping hash-based IDs to `*WorkItem`. Lookups are O(1).
- **File system event handler:** `onFsysUpdate(ev)` is the critical bridge between the file watcher and the work item cache. It processes `Create`, `Write`, `Remove`, and `Rename` events to keep the cache in sync with disk.
- **Work item creation:** `CreateWorkItem(simpleName)` creates a directory named `{PREFIX}-{NNNN}-{name}` and its `.yaml` metadata file, then waits for the cache to sync.
- **Reference parsing:** `laneParseReference(ref)` parses three formats:
  - `"lane_name"` — lane in the same module
  - `"module.lane_name"` — cross-module reference
  - `"file:..."` — file/glob pattern within a lane directory
- **Slot availability:** `HasAvailableSlot()` checks if `CountActive < MaxAgents` (or always true if `MaxAgents == 0`).
- **Filtering:** `FilterWorkItems(cond)` applies the condition DSL within this lane.

**Inbox Special Handling:** The inbox lane additionally accepts standalone `.md` and `.txt` files (not just directories), with `Seq = 0`. This allows raw notes and ideas to enter the system without formal structure.

---

### WorkItem

**File:** `workitem.go`

A unit of work represented as a directory on disk with a `.yaml` metadata file.

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

    attributesModTime time.Time // cache timestamp for attribute reload
}
```

**Directory Structure on Disk:**
```
{module}/{lane}/{PREFIX}-{NNNN}-{name}/
├── {PREFIX}-{NNNN}-{name}.yaml   # metadata (attributes)
├── description.md                 # task description
├── notes.md                       # agent/user notes
└── ...                            # other artifacts
```

**Key Responsibilities:**
- **Attributes lifecycle:** `AttributesLoad()` reads from `{Name}/{Name}.yaml` if the file has been modified (checked via `attributesModTime`). `AttributesSave(other)` merges and writes back.
- **Dependencies:** `Dependencies()` iterates over work items referenced in the `dependencies` attribute. `Dependents()` finds items that depend on this one.
- **Lane transitions:** `MoveTo(laneName)` moves the item's directory from one lane to another via `os.Rename`, effectively progressing it through the workflow.
- **JSON serialization:** `MarshalJSON()` includes lane name, module name, and resolved dependencies for external tooling.

---

### Attributes

**File:** `attributes.go`

A typed key-value store (`map[string]any`) backed by YAML files.

```go
type Attributes map[string]any
```

**Type-Aware Accessors:**
- `String(key)`, `Int(key)`, `Float(key)`, `Bool(key)` — perform Go type conversion (e.g., `Int()` accepts all int/uint/float variants).
- `StringArray(key)` — returns `[]string`.
- `Get(key)` / `Set(key, value)` / `Has(key)` / `Delete(key)` — basic CRUD.
- `Keys()` — returns sorted key names.
- `Merge(other)` — combines two attribute maps.
- `LoadFromYAML(path)` / `SaveToYAML(path)` — file I/O.

Attributes power the ignore rules pipeline, condition DSL evaluation, and agent prompt generation.

---

## Execution System

### Executor

**File:** `executor.go`

The tick-based execution loop that drives autonomous agent invocation.

```go
type Executor struct {
    project *Project
    invoker WorkItemInvoker
    mu      sync.Mutex
    active  map[string]InvocationHandle  // item ID → running invocation
    done    chan struct{}
    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
}
```

**Execution Loop:**

```
Start()
  └── goroutine: Run()
        └── for {
              select {
              case <-ctx.Done(): return
              case <-ticker:     tick()
              }
            }
```

**`tick()` — Single Iteration:**
1. `cleanupCompleted()` — removes finished invocations from `active` map, sets `InProgress = false` on items.
2. `processWorkItems()` — scans all modules (in config order), then lanes (in `Order` priority), then items:
   - Skip lanes with no `AgentBehavior` defined (purely organizational lanes).
   - Check lane has an available slot (`HasAvailableSlot()`).
   - Check project has an available slot (`project.HasAvailableSlot()`).
   - For each item: skip if already `InProgress`, skip if `shouldIgnore()` returns true, otherwise invoke.

**`invokeItem(module, lane, item)`:**
1. Marks `item.InProgress = true`.
2. Calls `invoker(project, module, lane, item)` to get an `InvocationHandle`.
3. Stores handle in `active` map.
4. Spawns a goroutine that waits for completion and cleans up.

---

### InvocationHandle

**File:** `types.go`

Represents a running agent invocation.

```go
type InvocationHandle struct {
    Item *WorkItem
    Done chan struct{}
    err  error
}
```

- `Wait()` — blocks until the invocation completes.
- `IsComplete()` — non-blocking check via select on `Done` channel.

---

### Slot Management

Two-level concurrency control:

| Level | Field | Description |
|-------|-------|-------------|
| **Project** | `Execution.MaxAgents` | Maximum total `InProgress` items across ALL modules. If `<= 0`, unlimited. |
| **Lane** | `Lane.MaxAgents` | Maximum `InProgress` items within this lane. If `== 0`, unlimited. |

Both checks must pass for an item to be invoked. The executor respects both limits during `processWorkItems()`.

---

## File System Watcher (Fsys)

**File:** `fsys.go`

Wraps `fsnotify.Watcher` to provide real-time awareness of file system changes.

```go
type Fsys struct {
    watcher *fsnotify.Watcher
    out     chan FsysEvent  // buffered, capacity 1024
    done    chan struct{}
}

type FsysEvent struct {
    Path     string
    Op       FsysOp  // Create, Write, Remove, Rename
    Time     time.Time
    IsDir    bool
    FileInfo fs.FileInfo
}
```

### Event Flow

```
fsnotify.Watcher
  └── loop() reads fsnotify.Events
        └── handle(e) normalizes events
              └── out ← FsysEvent
                    └── goroutine in initializeFsys() dispatches to lane.onFsysUpdate(ev)
```

**Key Behaviors:**
- **Recursive watching:** `addRecursive(root)` walks all subdirectories and adds each to the watcher.
- **Recursive creation events:** When a directory is created, `handle()` synthesizes `Create` events for all existing files within it, ensuring no items are missed.
- **Operation mapping:** `fsnotify.Op.Create` → `FsysOpCreate`, `fsnotify.Op.Write` → `FsysOpWrite`, etc.
- **Buffered channel:** The `out` channel has capacity 1024, providing backpressure if the consumer falls behind.

### Work Item Cache Sync

When `initializeFsys()` starts, it:
1. Creates the `Fsys` watcher.
2. Recursively watches all lane directories.
3. **Synthesizes `FsysOpCreate` events** for every existing file (cold-start cache population).
4. Starts a goroutine that reads `FsysEvent`s and dispatches them to the matching lane's `onFsysUpdate(ev)`.

`onFsysUpdate(ev)` maintains the `workItemsByID` cache:
- **Create:** Adds a new work item to the cache.
- **Write:** Updates the item's `ModTime` and file list.
- **Remove:** Removes the item from the cache.
- **Rename:** Treats as remove + create (lane transition).

---

## Ignore Rules Pipeline

**File:** `should_ignore.go`

Before invoking an agent, the executor runs a pipeline of ignore checks. The first check that returns `true` causes the item to be skipped for this tick.

**Evaluation Order:**

```
shouldIgnoreIfModtime     → N-second debounce
  ↓ (if false)
shouldIgnoreIfExists      → skip if referenced lanes have items
  ↓ (if false)
shouldIgnoreIfNotExists   → skip if referenced lanes are empty
  ↓ (if false)
shouldIgnoreIfDependency  → skip if dependencies are in referenced lanes
  ↓ (if false)
shouldIgnoreIfAttr        → skip if attributes match condition DSL
  ↓ (if all false)
INVOKE
```

### shouldIgnoreIfModtime

**File:** `should_ignore_if_modtime.go`

Skips items modified within the last N seconds. This debounce prevents the executor from acting on items that are still being written by a human or external process. Defaults to 30 seconds

### shouldIgnoreIfExists

**File:** `should_ignore_if_exists.go`

For each reference in `Lane.IgnoreIfExists`:
- If it's a lane reference: checks if that lane has ANY work items.
- If it's a `file:...` reference: checks if any file matches the glob pattern.

Returns `true` (ignore) if **ANY** reference matches. Supports glob patterns including `**`.

### shouldIgnoreIfNotExists

**File:** `should_ignore_if_not_exists.go`

Inverse of `shouldIgnoreIfExists`. Returns `true` (ignore) if **ANY** referenced lane is empty or **ANY** referenced file is missing.

### shouldIgnoreIfDependency

**File:** `should_ignore_if_dependency.go`

For each reference in `Lane.IgnoreIfDependency`:
- Checks if the work item's `dependencies` attribute references items that exist in the referenced lane.
- Returns `true` (ignore) if any dependency is found in a referenced lane.

This enables lanes that should only run after upstream dependencies have been resolved.

### shouldIgnoreIfAttr

**File:** `should_ignore_if_attr.go`

Parses `Lane.IgnoreIfAttr` as a condition DSL string and evaluates it against the item's `Attributes`. Returns `true` if the condition evaluates to true.

Example: `"priority > 3"` — items with priority > 3 are ignored in this lane.

---

## Project Loading & Initialization

**File:** `load.go`

The bootstrapping process that transforms `.orqen/orqen.yaml` on disk into a running `Project`.

**`Load(projectDir)` — Main Entry Point:**
1. `ValidateDir(projectDir)` — ensures directory exists and contains `.orqen/orqen.yaml`.
2. Reads and unmarshals `orqen.yaml` via `goccy/go-yaml`.
3. `applyDefaults(proj)` — sets defaults:
   - `Execution.MaxAgents = 10`
   - `Execution.SleepIntervalSeconds = 60`
   - Auto-creates `inbox` lane if none defined
   - Formats lane dirs as `NN_name` (e.g., `01_inbox`, `02_doing`)
   - Creates tinylfu caches for modules and lanes
4. `validate(proj)` — checks modules exist, lanes have unique names, agent clients have commands.
5. `initialize(proj)` — creates directories, generates prompt files, initializes memory DB (if `FLAG_USE_MEMORY`), sets up file system watcher.
6. Caches the project in a global `projects` map (keyed by directory hash) for `Get(id)` lookup.

**`initialize(proj)` — Prompt Generation:**
- Creates `prompts/` directories for modules and lanes.
- Generates `HEADER.md` from the embedded template (`prompts/HEADER.md`) or uses an existing file.
- Generates per-lane prompt files by combining `HEADER.md`, lane `Purpose`, `AgentBehavior`, `CriticalRules`, and `ExtraPrompt`.

**`initializeFsys(proj)` — File System Watcher Setup:**
- Creates an `Fsys` watcher.
- Recursively watches all lane directories.
- Synthesizes `FsysOpCreate` events for all existing files (populates initial cache).
- Starts a goroutine to dispatch events to matching lanes.

---

## Lane References & Resolution

Lane references appear in `IgnoreIfExists`, `IgnoreIfNotExists`, and `IgnoreIfDependency`. Three formats are supported:

| Format | Example | Meaning |
|--------|---------|---------|
| Lane name | `"review"` | Lane in the same module |
| Qualified | `"backend.review"` | Lane `review` in module `backend` |
| File pattern | `"file:backend.review/docs/*.md"` | Files matching glob in lane directory |
| Deep glob | `"file:backend.**/*.go"` | Glob across all subdirectories |

**Resolution logic** (`laneResolvePath`):
1. Parse the reference via `laneParseReference`.
2. If it's a lane reference, resolve the module and lane name.
3. If it's a file reference, resolve to the target lane and apply the glob pattern.

---

## Condition DSL

The condition DSL (used in `IgnoreIfAttr` and `FilterWorkItems`) is parsed and evaluated by `pkg/condition`. It supports expressions like:

```
priority > 3
status == "blocked"
type in ["bug", "incident"]
```

The DSL operates against the `Attributes` map of a work item, allowing lane configurations to express conditional ignore rules and item filtering without Go code changes.

---

## Prompt System

**File:** `prompts/HEADER.md` (embedded via `//go:embed`)

The prompt system generates agent prompts by combining:
1. **HEADER.md** — A shared preamble (embedded template or project-specific override).
2. **Lane Purpose** — A description of the lane's role, injected from configuration.
3. **Agent Behavior** — Numbered steps (`1.`, `2.`, `3.` ...) that define the agent's workflow.
4. **Critical Rules** — Absolute rules the agent must follow.
5. **Extra Prompt** — Additional context from module or lane configuration.

Generated prompts are written to `{module}/{lane}/prompts/PROMPT.md` and read by the invoker when calling the agent.

---

## Memory Store (WIP)

An optional memory store (`pkg/memory/store`) is integrated but gated by a `FLAG_USE_MEMORY` build flag. When enabled, it provides persistent knowledge across sessions. The `Project.Memory()` method returns the store instance. This feature is a work in progress and may not be available in all builds.

---

## Concurrency & Thread Safety

The engine is designed for concurrent operation. Key thread-safety mechanisms:

| Component | Mechanism | Protected Data |
|-----------|-----------|----------------|
| `Project` | `sync.Mutex` | `running`, `executor`, `invoker` |
| `Module` | `sync.Mutex` | `workItemsBySeq`, `workItemsStashed` |
| `Lane` | `sync.Mutex` | `workItemsByID` (via tinylfu thread-safe cache) |
| `Executor` | `sync.Mutex` | `active` map |
| `projects` (global) | `sync.Mutex` | Global project cache |
| Caches | `tinylfu.SyncCacheT` | Thread-safe LRU cache |

**Key patterns:**
- **Back-references** avoid passing context down chains, reducing lock contention.
- **Iterators** (`iter.Seq2`) provide lazy evaluation without snapshotting entire collections.
- **TX pattern** (`TxNewWorkItem`) ensures sequence number uniqueness under concurrent writes.
- **Invocation tracking** via `active` map and `Done` channels enables clean shutdown.

---

## Key Design Patterns

| Pattern | Where | Why |
|---------|-------|-----|
| **Hierarchical back-references** | `WorkItem → Lane → Module → Project` | Any component can traverse upward without explicit context passing |
| **Tick-based execution** | `Executor.Run()` | Simple, predictable loop that avoids complex scheduling logic |
| **File-system as source of truth** | `Fsys → onFsysUpdate → workItemsByID` | Work items exist as directories on disk, not in a separate database |
| **LRU caching** | tinylfu caches for `workItemsBySeq`, `workItemsByID` | Fast lookups without scanning all files on every tick |
| **Short-circuit ignore pipeline** | `shouldIgnore*` chain | Composable filtering where the first matching rule skips the item |
| **Two-level concurrency** | Project + Lane `MaxAgents` | Prevents both per-lane overload and system-wide saturation |
| **Directory-as-WorkItem** | `{PREFIX}-{NNNN}-{name}/` | Human-readable, git-friendly, file-system-watchable |
| **Embedded prompts** | `//go:embed prompts/HEADER.md` | Sensible defaults that can be overridden per-project |

---

## External Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/fsnotify/fsnotify` | Real-time file system watching |
| `github.com/goccy/go-yaml` | YAML parsing and marshaling for `.orqen/orqen.yaml` and work item attributes |
| `github.com/nidorx/orqen/pkg/utils/tinylfu` | Thread-safe LRU cache for work item lookups |
| `github.com/nidorx/orqen/pkg/utils` | Hash generation and unique ID utilities |
| `github.com/nidorx/orqen/pkg/utils/glob` | Glob pattern matching for file references |
| `github.com/nidorx/orqen/pkg/condition` | Condition DSL parsing and evaluation |
| `github.com/nidorx/orqen/pkg/memory/store` | Optional persistent memory store (WIP, gated by `FLAG_USE_MEMORY`) |

---

## Adding a New Lane

To add a new lane to a module:

1. Define the lane in `.orqen/orqen.yaml` under the module's `lanes` list.
2. Set `name`, `purpose`, and optionally `max_agents`, `agent_behavior`, `critical_rules`.
3. If the lane should have a specific directory order, add its name to the module's `order` list.
4. Optionally configure ignore rules (`ignore_if_attr`, `ignore_if_exists`, etc.).
5. The engine will auto-create the directory structure and generate prompt files on next `Load()`.

---

## Adding a New Module

1. Add a new entry under `modules` in `.orqen/orqen.yaml`.
2. Define `name`, `dir`, `prefix`, `order` (lane priority), and `lanes`.
3. Run `Load()` — the engine validates, applies defaults, creates directories, and generates prompts.
