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
   - Manages `WorkItem` instances via `ListItems()`
   - Enforces per-lane `max_agents` limits
   - Supports execution precondition rules:
     - `ignore_if_exists`: Skip if any items exist in referenced lanes
     - `ignore_if_dependency`: Skip if item's dependencies exist in referenced lanes

4. **Executor** (`executor.go`)
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

## Testing

Comprehensive unit tests cover:

- Lane item discovery and tracking
- Slot management (per-lane and project-level)
- Module and project active agent counting
- Ignore rules (both `ignore_if_exists` and `ignore_if_dependency`)
- Lane reference parsing
- Executor tick behavior
- Max agents enforcement
- Cleanup of completed invocations
- Full integration test of the execution loop

Run tests with:
```bash
go test ./pkg/project -v
```

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
