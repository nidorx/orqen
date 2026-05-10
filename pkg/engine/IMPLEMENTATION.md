# Project Execution Loop Implementation

## Overview

This document describes the implementation of the `Start` method in `pkg/project/project.go`, which implements an execution loop similar to `autopilot/run-afk.go`.

## Architecture

### Key Components

1. **Project** (`project.go`)
   - Top-level configuration and execution manager
   - Manages multiple modules, each with multiple lanes
   - Controls global `max_agents` limit across all modules
   - Provides `Start()` and `Stop()` methods for lifecycle management

2. **Module** (`module.go`)
   - Represents a functional module (e.g., "task", "adr")
   - Contains multiple lanes in a defined order
   - Tracks active agents across all its lanes

3. **Lane** (`lane.go`)
   - Represents a workflow stage within a module
   - Manages `WorkItem` instances via `WorkItems()` iterator (Go range function pattern)
   - Work item caches are maintained by `onFsysUpdate()` which processes file system events
   - Two cache maps: `workItemsByID` (hash-based ID) and `workItemsBySeq` (sequential ID)
   - Enforces per-lane `max_agents` limits
   - Supports execution precondition rules:
     - `ignore_if_exists`: Skip if any items exist in referenced lanes
     - `ignore_if_dependency`: Skip if item's dependencies exist in referenced lanes
   - `onFsysUpdate()` handles file system events (create, remove, rename, write) to keep caches synchronized

4. **Fsys** (`fsys.go`)
   - File system watcher wrapper around `fsnotify`
   - Recursively watches all lane directories
   - Emits `FsysEvent` for file/directory changes (create, write, remove, rename)
   - Events are consumed by lane's `onFsysUpdate()` to maintain work item caches

5. **Executor** (`executor.go`)
   - Manages the execution loop
   - Scans lanes in order, respects slot availability
   - Invokes agents via the `AgentInvoker` interface
   - Tracks and cleans up completed invocations

5. **AgentInvoker** (`types.go`)
   - Interface for invoking agent executions
   - Returns `InvocationHandle` for tracking async completion
   - Allows custom implementations for different agent types

## Execution Flow

```
Project.Start()
    ↓
Executor.Run() - Main loop (runs on sleep interval)
    ↓
Executor.tick() - Single iteration
    ├── cleanupCompleted() - Remove finished invocations
    └── processWorkItems() - Find and execute work
            ↓
        For each module (in config order)
            For each lane (in module.Order)
                ├── Check lane slot availability
                ├── Check project slot availability
                └── For each work item
                        ├── Load dependencies (DEP_* files)
                        ├── Check ignore_if_exists rules
                        ├── Check ignore_if_dependency rules
                        └── Invoke agent if eligible
```

## Lane References

Lane references support both same-module and cross-module references:

- `"doing"` - Lane "doing" in the current module
- `"adr.draft"` - Lane "draft" in the "adr" module
- `"task.ready"` - Lane "ready" in the "task" module

## Ignore Rules

### ignore_if_exists

Skips execution if **any** work item exists in the referenced lanes:

```yaml
lanes:
  - name: "ready"
    ignore_if_exists: ["doing", "adr.draft"]
```

This means: "Don't start new work from 'ready' lane if there are items in 'doing' or 'adr.draft' lanes."

### ignore_if_dependency

Skips execution if the work item's **specific dependencies** (from DEP_* files) exist in the referenced lanes:

```yaml
lanes:
  - name: "ready"
    ignore_if_dependency: ["prioritized", "backlog", "doing"]
```

This means: "Don't start this 'ready' item if any of its dependencies (DEP_001, DEP_002, etc.) are found in 'prioritized', 'backlog', or 'doing' lanes."

## Extensibility

The validation system is designed to be extensible:

1. **New validation types** can be added by:
   - Adding new fields to the `Lane` struct
   - Implementing validation functions in `lane.go`
   - Calling them from `Executor.shouldIgnoreItem()`

2. **Custom AgentInvoker** implementations can be provided:
   ```go
   type MyInvoker struct{}

   func (m *MyInvoker) Invoke(project *Project, module *Module, lane *Lane, item *WorkItem) (InvocationHandle, error) {
       // Custom invocation logic
       return handle, nil
   }

   project.WithInvoker(&MyInvoker{})
   project.Start()
   ```

## File System Update Mechanism

### Overview

The project now uses a file system watcher-based approach to keep work item caches synchronized with the actual file system state. This replaces the previous on-demand scanning approach.

### Initialization Flow

1. **Project Load** (`load.go`)
   - `Load()` loads and validates project configuration
   - `applyDefaults()` initializes lane caches (`workItemsByID`, `workItemsBySeq`)
   - `initialize()` creates directories and prompts
   - `initializeFsys()` sets up file system watcher

2. **Fsys Initialization** (`load.go:442`)
   - Creates `Fsys` watcher using `fsnotify`
   - Recursively adds all lane directories to watcher
   - Synthesizes `FsysEvent{Op: FsysOpCreate}` for existing files/directories
   - Calls `lane.onFsysUpdate()` for each existing entry to populate initial caches
   - Starts goroutine to consume events from `fsys.Events()` and dispatch to lanes

3. **Event Dispatch**
   - Main goroutine reads from `fsys.Events()`
   - For each event, finds the matching lane by path prefix
   - Calls `lane.onFsysUpdate(ev)` with the event

### onFsysUpdate Processing (`lane.go:54`)

The `onFsysUpdate` method processes file system events to maintain work item caches:

1. **Path Resolution**
   - Converts absolute event path to relative path from lane directory
   - Normalizes to forward slashes for consistency
   - Extracts `itemName` (top-level directory/file) and `fileExtraPath` (nested path)

2. **Work Item Directory Detection**
   - Uses `isWorkItemDir()` to check if `itemName` matches pattern `TYPE-NNN-*` (e.g., `TASK-001`, `ADR-002`)
   - Extracts sequence number with `extractItemSeq()`
   - Computes work item ID as hash of `Seq+Name`

3. **Event Handling by Operation**
   - **`FsysOpRemove`/`FsysOpRename` (top-level dir)**: Removes item from both caches
   - **`FsysOpCreate` (top-level dir)**: Walks directory, collects files, computes latest `ModTime`, populates `Files` slice
   - **`FsysOpCreate` (internal file)**: Adds file to item's `Files` list
   - **`FsysOpCreate` (internal subdir)**: Walks new subdir and adds files
   - **`FsysOpRemove`/`FsysOpRename` (internal file/subdir)**: Filters out files with removed prefix

4. **Inbox Special Rules**
   - Inbox lane allows plain `.md`/`.txt` files (not just directories)
   - These items have `Seq = 0` and a single file
   - Other file extensions are ignored

5. **Cache Upsert**
   - Stores item in `workItemsByID` (keyed by hash ID)
   - Stores item in `workItemsBySeq` (keyed by stringified Seq) if `Seq > 0`
   - Reuses existing `WorkItem` objects to preserve `InProgress` state

### Benefits

- **Real-time synchronization**: Work item caches are updated immediately when file system changes
- **State preservation**: `InProgress` state is preserved across file system events
- **No polling needed**: Event-driven approach eliminates need for periodic directory scanning
- **Efficient**: Only processes changed files/directories, not entire directory trees

### Testing

Comprehensive tests cover `onFsysUpdate` scenarios:
- Creating work item directories
- Adding files to existing work items
- Removing work item directories
- Renaming work item directories
- Inbox file handling (accepts `.md`/`.txt`, ignores other extensions)
- Preserving `InProgress` state across updates
- Ignoring non-work-item directories

## Testing

Comprehensive unit tests cover:

- Lane item discovery and tracking via `onFsysUpdate()`
- File system event processing (create, remove, rename)
- Work item cache management (`workItemsByID`, `workItemsBySeq`)
- Slot management (per-lane and project-level)
- Module and project active agent counting
- Ignore rules (both `ignore_if_exists` and `ignore_if_dependency`)
- Lane reference parsing
- Executor tick behavior
- Max agents enforcement
- Cleanup of completed invocations
- Full integration test of the execution loop
- Inbox file handling

Run tests with:
```bash
go test ./pkg/project -v
```

Key test files:
- `lane_test.go`: Lane-specific tests including work item iteration and reference parsing
- `module_test.go`: Module-level tests including lane ordering and sequencing
- `project_test.go`: Project-level tests including load, executor, and ignore rules
- `fsys_update_test.go`: Comprehensive tests for `onFsysUpdate()` function covering all event types and edge cases

## Thread Safety

The implementation uses mutexes to ensure thread-safe access to:
- Work item `InProgress` state (via `Lane.itemsMutex`)
- Active invocation tracking (via `Executor.mu`)
- Mock invoker invocations list (via `mockInvoker.mu`)

## Usage Example

```go
// Load project from directory
project, err := project.Load("/path/to/project")
if err != nil {
    log.Fatal(err)
}

// Set custom invoker (optional)
project.WithInvoker(myCustomInvoker)

// Start execution (non-blocking)
project.Start()

// ... later ...

// Stop execution (blocking until complete)
project.Stop()
```
