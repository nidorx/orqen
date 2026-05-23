# Orqen Configuration Reference

This document describes every attribute available in `orqen.yaml`, the project configuration file located at `.orqen/orqen.yaml`.

## File Location

```
<project-root>/
├── .orqen/
│   └── orqen.yaml          # Project configuration
└── ...
```

## Top-Level Structure

```yaml
agents:      # AI agent client definitions
execution:   # Runtime execution settings
hooks:       # Named hook definitions (reusable command arrays)
modules:     # Workflow modules (task, adr, learning, etc.)
```

---

## `agents` - Agent Client Configuration

Defines which AI agents are available and how to invoke them.

```yaml
agents:
  default: "qwen"                    # Default agent client name
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]   # CLI command + arguments
    claude:
      command: ["claude", "--dangerously-skip-permissions"]
```

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `default` | string | Yes | - | Name of the default agent client. Must match a key under `clients`. |
| `clients` | map | Yes | - | Map of agent client definitions. Each key is the agent name. |
| `clients.<name>.command` | array of string | Yes | - | Shell command (executable + arguments) used to invoke the agent. Must include flags that enable autonomous mode (e.g., `--yolo`) and ACP protocol support (e.g., `--acp`). |

### Notes

- The agent command is invoked as a subprocess. The agent receives synthesized prompts and interacts with the project via MCP tools (`orqen_status`, `orqen_item_create`, `orqen_item_move`, etc.).
- You can define multiple agent clients and override the default per-lane using the `agent` attribute on a lane.

---

## `execution` - Runtime Settings

Controls how the engine runs across all modules.

```yaml
execution:
  max_agents: 10                   # Max concurrent agents (0 = unlimited)
  sleep_interval_seconds: 60       # Seconds between work cycles
```

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `max_agents` | int | No | 0 (unlimited) | Maximum number of agents running concurrently across all modules. If set to `0` or negative, there is no limit. |
| `sleep_interval_seconds` | int | No | 60 | Number of seconds the engine sleeps between scanning for available work. Controls the polling frequency of the execution loop. |

---

## `hooks` - Named Hook Definitions

Defines reusable command arrays that can be bound to modules and lanes for pre/post task execution. Hooks support OS-specific variants and lane-level exclusion.

```yaml
hooks:
  notify: ["notify.sh", "$WI"]
  notify.windows: ["notify.cmd", "%WI%"]   # OS-specific variant for Windows
  validate: ["validate.sh", "--strict"]
  cleanup: ["cleanup.sh"]
```

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `<hook_name>` | array of string | Yes | - | Command array for the hook. The hook name must be a valid identifier (alphanumeric, underscores, hyphens). |
| `<hook_name>.windows` | array of string | No | - | Windows-specific command array. Used when `runtime.GOOS == "windows"`. |
| `<hook_name>.darwin` | array of string | No | - | macOS-specific command array. Used when `runtime.GOOS == "darwin"`. |
| `<hook_name>.linux` | array of string | No | - | Linux-specific command array. Used when `runtime.GOOS == "linux"`. |

### OS Resolution

At execution time, the system resolves the command array based on the current OS:

1. If an OS-specific variant exists for the current platform, use it.
2. Otherwise, fall back to the base command array.

### Wildcards

Hook commands support the following wildcards:

| Wildcard | Description | Example |
|----------|-------------|---------|
| `$WI` | Work item name (e.g., `01_vision/my-task.md`, `03_backlog/WI-0001-my-task`) | `["notify.sh", "$WI"]` |

### Module and Lane Bindings

Named hooks are referenced by modules and lanes using `pre` and `post` bindings:

```yaml
modules:
  - name: task
    hooks:                          # Applied to ALL lanes in this module
      pre:
        - validate
        - notify
      post:
        - cleanup
    lanes:
      - name: "inbox"
        hooks:                      # Lane-specific overrides
          post:
            - "!cleanup"            # Exclude module-level 'cleanup' hook
            - notify                # Add this hook for this lane
```

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `hooks` | object | No | - | Hook bindings for the module or lane. |
| `hooks.pre` | array of string | No | - | Hooks executed **before** task execution. Each item references a named hook. |
| `hooks.post` | array of string | No | - | Hooks executed **after** task execution. Each item references a named hook. |

### Exclusion Syntax

At lane level, use the `!` prefix to **exclude** a module-level hook:

```yaml
hooks:
  post:
    - "!cleanup"    # Do NOT run the 'cleanup' hook in this lane
    - notify        # DO run the 'notify' hook in this lane
```

**Important:** The `!` prefix must be quoted in YAML (e.g., `"!cleanup"` or `'!cleanup'`) because `!` is a YAML tag indicator. Unquoted `!cleanup` will cause a YAML parse error.

### Validation Rules

- Every named hook definition must have a non-empty base command array.
- All hook bindings must reference an existing named hook.
- Hook names must contain only alphanumeric characters, underscores, and hyphens.
- Exclusion bindings (`!hook_name`) are only meaningful at lane level (they exclude module-level hooks).

---

## `modules` - Workflow Modules

Modules group related work items into lanes within a dedicated directory. Each module defines its own workflow pipeline.

```yaml
modules:
  - name: task                     # Module name (unique within project)
    dir: "tasks"                   # Directory relative to project root
    order: ["doing", "inbox"]      # Lane priority order for work selection
    prefix: "TASK"                 # Prefix for work item names (default: "WI")
    extra_prompt: |                # Context injected into module's HEADER.md
      **Consult ADRs:** Before refining...
    lanes:                         # List of lanes in this module
      - name: "inbox"
        purpose: "..."
```

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | - | Unique module name. Used in cross-module lane references (e.g., `task.doing`). |
| `dir` | string | Yes | - | Directory path **relative to the project root** where the module's lanes and work items are stored. For example, `dir: "tasks"` with project root at `/home/user/project` creates lanes at `/home/user/project/tasks/01_inbox/`, `/home/user/project/tasks/02_backlog/`, etc. |
| `prefix` | string | No | `WI` | Prefix used in work item directory and file names. Normalized to uppercase with spaces and `-` removed. For example, `prefix: "task"` produces `TASK-0001-implement-auth` directories and `TASK-0001.yaml` attribute files. Custom prefixes like `LKN` produce `LKN-0001-automacao-diaria`. |
| `order` | array of string | No | Lane declaration order | List of lane names that defines the priority order for work selection. The engine checks lanes in this order when looking for available work. Lanes not listed are appended at the end in declaration order. |
| `extra_prompt` | string | No | - | Additional context injected into the module's `HEADER.md` file. This is **not** step-by-step instructions - use it for domain knowledge, constraints, or references (e.g., "Consult ADRs before refining", "All content must be in pt-BR"). |
| `lanes` | array | Yes | - | List of lane definitions (see [Lane Attributes](#lanes--lane-configuration) below). |

### How `dir` and `prefix` Work

The `dir` attribute determines **where** the module is written on disk. The `prefix` attribute determines **how** work items are named.

```
Project root:     /home/user/my-project/
Module dir:       "docs/learnings"
Module prefix:    "LRN"
Result:           /home/user/my-project/docs/learnings/
                                                └── 01_knowledge/
                                                    └── LRN-0001-some-pattern/
                                                        ├── LRN-0001.yaml
                                                        └── LRN-0001-LEARNING.md
```

All lanes within the module are created as subdirectories (`NN_name` format) inside this directory. All work items are created inside their respective lane directories.

---

## `lanes` - Lane Configuration

A lane represents a stage in a workflow pipeline (e.g., `inbox`, `doing`, `review`, `done`). Lanes define **what** the agent should do, **when** it should skip work, and **how** to behave.

```yaml
lanes:
  - name: "doing"
    purpose: "Task being implemented by the agent"
    agent: "qwen"
    max_agents: 3
    artifacts: ["SUMMARY", "FAIL"]
    user_action: "review"
    agent_behavior:
      - "Read the provided task document"
      - "Implement according to specifications"
    critical_rules:
      - "Create the SUMMARY artifact upon completion"
    ignore_if_exists: ["draft"]
    ignore_if_not_exists: ["metrics.md"]
    ignore_if_dependency: ["doing"]
    ignore_if_attr: "priority > 3"
    extra_prompt: |
      Upon successful completion, create the SUMMARY artifact...
```

### Lane Attributes

| Attribute | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | - | Lane name. The engine creates directories as `NN_name` (e.g., `05_doing`). Must be kebab-case: lowercase letters, numbers, and hyphens. |
| `purpose` | string | Yes | - | Description of the lane's purpose. **Injected into the agent prompt** so the agent understands what this lane is for. |
| `agent` | string | No | Project default | Overrides the default agent client for this lane. Must match a key under `agents.clients`. |
| `max_agents` | int | No | 0 (unlimited) | Maximum number of agents working simultaneously in this lane. When the limit is reached, the lane is skipped for work selection. |
| `artifacts` | array of string | No | - | Artifact types the agent **may** create when executing this lane (e.g., `SUMMARY`, `FAIL`, `REFINEMENT`). The engine uses this to render naming convention instructions in `HEADER.md`. |
| `user_action` | string | No | - | Short label describing the expected user action when a work item reaches this lane (e.g., `"approve"`, `"archive"`, `"review and accept"`). Displayed in UI/CLI to guide the user. |
| `agent_behavior` | array of string | No | - | **Sequential steps** the agent follows. Each item becomes a numbered step (`1.`, `2.`, `3.`) in the final prompt. Describe **what the agent does** with the work item it already received - not how to find or scan the lane. |
| `critical_rules` | array of string | No | - | **Absolute, non-negotiable rules**. Rendered in a highlighted, separate section in the final prompt. Use for rules that must **never** be ignored (e.g., "ALL tasks must be created from this single inbox file"). |
| `extra_prompt` | string | No | - | Additional context injected **after** `agent_behavior` in the lane prompt. Use for knowledge that is **not** step-by-step: how to consult ADRs, confidence levels, validation criteria, trade-offs, domain constraints. |
| `ignore_if_exists` | array of string | No | - | Skip this lane if **any items exist** in the referenced lanes. Supports lane names (`"draft"`), cross-module references (`"adr.draft"`), and file patterns (`"file:draft.*.md"`, glob supported). |
| `ignore_if_not_exists` | array of string | No | - | Skip this lane if **no items or files exist** in the referenced lanes. Same reference format as `ignore_if_exists`. Useful for lanes that should only activate when a prerequisite file exists (e.g., `"metricas.md"`). |
| `ignore_if_dependency` | array of string | No | - | Skip a work item if it has **dependencies** attribute pointing to items in the referenced lanes. Prevents the agent from working on items whose prerequisites are still in progress. |
| `ignore_if_attr` | string | No | - | Skip work items whose **attributes** match the condition. Uses a SQL-like DSL (see [Condition Language](#condition-language-for-ignore_if_attr)). Evaluated against each work item's YAML attribute file. |

### Reference Format for Ignore Rules

The `ignore_if_exists` and `ignore_if_not_exists` attributes support three reference formats:

| Format | Example | Meaning |
|--------|---------|---------|
| Lane name (same module) | `"draft"` | Check if items exist in lane `draft` within the same module |
| Cross-module | `"adr.draft"` | Check if items exist in lane `draft` of module `adr` |
| File pattern | `"file:draft.*.md"` | Check if any work item contains a file matching the glob pattern |
| Cross-module file | `"file:adr.draft.ADR-*.md"` | Check items in `adr.draft` for files matching the pattern |

### How Ignore Rules Work Together

- **`ignore_if_exists`**: "Don't work here if there's already something in X." - Prevents parallel work on conflicting lanes.
- **`ignore_if_not_exists`**: "Don't work here unless X exists." - Activates lanes only when prerequisites are present.
- **`ignore_if_dependency`**: "Don't work on this item if its dependencies are still in X." - Ensures prerequisites are resolved.
- **`ignore_if_attr`**: "Don't work on this item if its attributes match this condition." - Enables dynamic, attribute-based filtering.

All ignore rules are evaluated per work item. If any rule triggers, the item is skipped for selection.

---

## Condition Language for `ignore_if_attr`

The `ignore_if_attr` attribute uses a SQL-like DSL that evaluates against a work item's YAML attributes. The condition is parsed and executed by the `pkg/condition` package.

### Supported Operators

#### Comparison

| Operator | Example | Description |
|----------|---------|-------------|
| `=` | `status = 'active'` | Equality comparison |
| `!=` | `status != 'done'` | Inequality comparison |
| `>` | `priority > 3` | Greater than (numeric) |
| `>=` | `priority >= 5` | Greater than or equal (numeric) |
| `<` | `priority < 3` | Less than (numeric) |
| `<=` | `priority <= 5` | Less than or equal (numeric) |

#### Pattern Matching

| Operator | Example | Description |
|----------|---------|-------------|
| `LIKE` | `name LIKE "^task-"` | Regex pattern match (Go regex syntax) |
| `CONTAINS` | `tags CONTAINS 'urgent'` | String/array contains check |
| `PREFIX` | `name PREFIX 'TASK-'` | String prefix check |
| `SUFFIX` | `name SUFFIX '.md'` | String suffix check |

#### Set Operations

| Operator | Example | Description |
|----------|---------|-------------|
| `IN` | `type IN ('bug', 'feature')` | Membership in a set |
| `NOT IN` | `status NOT IN ('done', 'archived')` | Not a member of a set |
| `ANY_OF` | `tags ANY_OF ('urgent', 'critical')` | Attribute array has ANY of the specified values |
| `ALL_OF` | `tags ALL_OF ('security', 'backend')` | Attribute array has ALL of the specified values |

#### Existence & Null Checks

| Operator | Example | Description |
|----------|---------|-------------|
| `EXISTS` | `deadline EXISTS` | Attribute exists (any value) |
| `IS NULL` | `reviewer IS NULL` | Attribute does not exist or is nil |
| `IS NOT NULL` | `reviewer IS NOT NULL` | Attribute exists and is not nil |

#### Range & Length

| Operator | Example | Description |
|----------|---------|-------------|
| `BETWEEN` | `priority BETWEEN 1 AND 5` | Numeric range check (inclusive) |
| `HAS_LENGTH` | `tags HAS_LENGTH 3` | Array has exactly N elements |

#### Logical Operators

| Operator | Example | Description |
|----------|---------|-------------|
| `AND` | `priority > 3 AND status = 'active'` | Both conditions must be true |
| `OR` | `type = 'bug' OR type = 'feature'` | At least one condition must be true |
| `NOT` | `NOT (status = 'done')` | Negates the following condition |
| `( )` | `(priority > 3 OR type = 'bug') AND status != 'archived'` | Grouping for precedence |

### Examples

```yaml
# Skip items with high priority
ignore_if_attr: "priority > 3"

# Skip items tagged as urgent or critical
ignore_if_attr: "tags ANY_OF ('urgent', 'critical')"

# Skip items with a specific status pattern
ignore_if_attr: "status LIKE '^blocked'"

# Skip items that are done or archived
ignore_if_attr: "status IN ('done', 'archived')"

# Skip items without a reviewer assigned
ignore_if_attr: "reviewer IS NULL"

# Complex condition with grouping
ignore_if_attr: "(priority > 3 AND status = 'active') OR tags ANY_OF ('blocked')"

# Skip items where a deadline attribute exists
ignore_if_attr: "deadline EXISTS"

# Skip items with more than 2 dependencies
ignore_if_attr: "dependencies HAS_LENGTH > 2"   # Note: uses array length comparison
```

### Attribute Source

Conditions are evaluated against the work item's YAML attribute file (e.g., `TASK-0001-my-task/TASK-0001.yaml`). Attributes defined in this file become the variables available in the condition language.

---

## Complete Example

```yaml
# ── Agent Configuration ──────────────────────────────────────────
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

# ── Execution Settings ───────────────────────────────────────────
execution:
  max_agents: 10
  sleep_interval_seconds: 60

# ── Named Hooks ──────────────────────────────────────────────────
hooks:
  notify: ["notify.sh", "$WI"]
  notify.windows: ["notify.cmd", "%WI%"]
  validate: ["validate.sh", "--strict"]
  cleanup: ["cleanup.sh"]

# ── Modules ──────────────────────────────────────────────────────
modules:
  - name: task
    dir: "tasks"
    order: ["doing", "learning", "review", "inbox", "prioritized", "ready"]
    hooks:
      pre:
        - validate
        - notify
      post:
        - cleanup
    extra_prompt: |
      **Consult ADRs:** Before refining, scan the ADR module for accepted ADRs.

    lanes:
      - name: "inbox"
        purpose: "User ideas ready to be transformed into tasks"
        artifacts: ["TASK"]
        user_action: "create ideas"
        agent_behavior:
          - "Read the provided inbox file to understand the idea"
          - "Analyze and decompose into executable tasks"
          - "Create each task in the backlog lane using the TASK template"
          - "Move the inbox file to the directory of the first task created"
        critical_rules:
          - "Create ALL tasks from the inbox file in this single invocation"

      - name: "doing"
        purpose: "Task currently being implemented"
        artifacts: ["SUMMARY", "FAIL"]
        ignore_if_exists: ["draft"]
        ignore_if_dependency: ["doing"]
        hooks:
          pre:
            - "!validate"   # skip validation in doing lane
            - notify
        agent_behavior:
          - "Read the provided task document and refinement if available"
          - "Check dependency files to ensure prerequisites are met"
          - "Implement the task according to specifications"
        extra_prompt: |
          Upon successful completion, create the SUMMARY artifact.
          If implementation fails with a total blocker, create the FAIL artifact.

      - name: "review"
        purpose: "Completed implementations awaiting quality review"
        artifacts: ["FAIL"]
        agent_behavior:
          - "Read the provided task document and its SUMMARY artifact"
          - "Validate ALL acceptance criteria are met"
          - "Validate ALL DoD items are satisfied"
          - "Review code quality, security, and performance"
        extra_prompt: |
          If validation PASSES, approve the task - no artifact is created.
          If validation FAILS, create the FAIL artifact with rejection reasons.

      - name: "done"
        purpose: "Archive of successfully completed tasks"
        user_action: "archive"

  - name: adr
    dir: "docs/adr"
    lanes:
      - name: "draft"
        purpose: "ADRs written by the agent, awaiting user review"
        user_action: "review and accept/reject"
        critical_rules:
          - "When any ADR exists in draft, the agent MUST respond with COMPLETE and perform NO task work"

      - name: "accepted"
        purpose: "User accepted ADRs - active decisions that constrain work"

      - name: "rejected"
        purpose: "ADRs rejected by the user"

      - name: "superseded"
        purpose: "ADRs replaced by newer ADRs"

      - name: "deprecated"
        purpose: "ADRs no longer recommended for new work"

  - name: learning
    dir: "docs/learnings"
    lanes:
      - name: "knowledge"
        purpose: "Pattern-level knowledge from task implementations"
        agent_behavior:
          - "Read the learning proposal from the task SUMMARY artifact"
          - "Validate each proposed learning against the current codebase"
          - "Assign confidence level: low, medium, or high"
          - "Write accepted learnings using the LEARNING template"
        extra_prompt: |
          Learnings are organized in category subdirectories:
          architecture, conventions, implementations, gotchas, workflow.
```

---

## Attribute Types Summary

### At a Glance

| Section | Attribute | Type | Required |
|---------|-----------|------|----------|
| **agents** | `default` | string | Yes |
| | `clients.<name>.command` | []string | Yes |
| **execution** | `max_agents` | int | No |
| | `sleep_interval_seconds` | int | No |
| **hooks** | `<hook_name>` | []string | Yes (if used) |
| | `<hook_name>.windows` | []string | No |
| | `<hook_name>.darwin` | []string | No |
| | `<hook_name>.linux` | []string | No |
| **modules** | `name` | string | Yes |
| | `dir` | string | Yes |
| | `prefix` | string | No |
| | `order` | []string | No |
| | `hooks` | object | No |
| | `hooks.pre` | []string | No |
| | `hooks.post` | []string | No |
| | `extra_prompt` | string | No |
| | `lanes` | []Lane | Yes |
| **lanes** | `name` | string | Yes |
| | `purpose` | string | Yes |
| | `agent` | string | No |
| | `max_agents` | int | No |
| | `artifacts` | []string | No |
| | `user_action` | string | No |
| | `agent_behavior` | []string | No |
| | `critical_rules` | []string | No |
| | `hooks` | object | No |
| | `hooks.pre` | []string | No |
| | `hooks.post` | []string | No |
| | `ignore_if_exists` | []string | No |
| | `ignore_if_not_exists` | []string | No |
| | `ignore_if_dependency` | []string | No |
| | `ignore_if_attr` | string | No |
| | `extra_prompt` | string | No |

---

## Implementation Notes

- Lane directories are automatically prefixed with a sequence number (`01_inbox`, `02_backlog`, etc.) based on declaration order.
- Work items are created as `PREFIX-NNNN-name` directories (e.g., `TASK-0001-implement-auth`, `LKN-0001-automacao-diaria`) containing a YAML attribute file (`TASK-0001.yaml`) and any artifact files.
- The `prefix` value is normalized at load time: converted to uppercase and all spaces and `-` removed. If not specified, defaults to `WI` (WorkItem).
- The prefix is used in artifact naming conventions shown in each module's `HEADER.md` (e.g., `TASK-${SEQUENCE}-${ARTIFACT}.md`).