[← Back to README](README.md)

# Engram — Technical Reference

**Persistent memory for AI coding agents**

This package is a fork of [Engram](https://github.com/Gentleman-Programming/engram), copied into Orqen to provide persistent memory. Many features from the original have been removed (CLI, TUI, HTTP server, cloud sync, git sync, agent plugins). This package will be evolved independently within Orqen.

This is the complete technical reference for the parts that remain. For the original full docs, see the [Engram repository](https://github.com/Gentleman-Programming/engram).

---

## Quick Navigation

| Section | What you'll find |
|---------|-----------------|
| [Database Schema](#database-schema) | Tables, FTS5, SQLite config |
| [MCP Tools](#mcp-tools) | Detailed reference for all memory tools |
| [MCP Project Resolution](#mcp-project-resolution) | Auto-detection algorithm, response envelope, tool categories |
| [Memory Protocol](#memory-protocol) | When/how agents should use the tools |
| [Project Name Normalization](#project-name-normalization) | Auto-detection, normalization, similar-project warnings |
| [Features](#features) | FTS5 search, timeline, privacy |

---

## Database Schema

### Tables

- **sessions** — `id` (TEXT PK), `project`, `directory`, `started_at`, `ended_at`, `summary`, `status`
- **observations** — `id` (INTEGER PK AUTOINCREMENT), `session_id` (FK), `type`, `title`, `content`, `tool_name`, `project`, `scope`, `topic_key`, `normalized_hash`, `revision_count`, `duplicate_count`, `last_seen_at`, `created_at`, `updated_at`, `deleted_at`
- **observations_fts** — FTS5 virtual table synced via triggers (`title`, `content`, `tool_name`, `type`, `project`)
- **user_prompts** — `id` (INTEGER PK AUTOINCREMENT), `session_id` (FK), `content`, `project`, `created_at`
- **prompts_fts** — FTS5 virtual table synced via triggers (`content`, `project`)
- **sync_chunks** — `target_key` (TEXT), `chunk_id` (TEXT), `imported_at`; composite PK (`target_key`, `chunk_id`) for target-scoped chunk tracking
- **memory_relations** — stores conflict-surfacing verdicts from `mem_judge`; columns include `sync_id` (TEXT PK), `source_id`, `target_id`, `relation`, `judgment_status` (`pending` | `judged`), `reason`, `evidence`, `confidence`, `marked_by_actor`, `marked_by_kind`, `marked_by_model`, `session_id`, `project`. Syncs across machines via cloud autosync when the project is enrolled.
- **sync_apply_deferred** — holds pulled mutations that could not be applied locally due to a missing FK dependency (e.g. relation references an observation not yet present); columns: `sync_id` (TEXT PK), `entity`, `payload`, `apply_status` (`deferred` | `applied` | `dead`), `retry_count`, `last_error`, `last_attempted_at`, `first_seen_at`. Rows with `apply_status='dead'` have exceeded the retry cap (5 attempts) and will not be retried automatically.

### SQLite Configuration

- WAL mode for concurrent reads
- Busy timeout 5000ms
- Synchronous NORMAL
- Foreign keys ON

---

## MCP Project Resolution

Engram resolves the project for every MCP tool call from the **server's working directory** (cwd), not from the LLM's arguments. This eliminates project drift caused by agents supplying different names for the same repo.

### Detection algorithm (5 cases)

| Case | Condition | Source | Project |
|------|-----------|--------|---------|
| 1 | cwd is a git root with `origin` remote | `git_remote` | repo name from remote URL |
| 2 | cwd is inside a git repo (subdirectory) | `git_root` | git root's directory basename |
| 3 | cwd has exactly one git-repo child | `git_child` | child repo name (warning included) |
| 4 | cwd has multiple git-repo children | `ambiguous_project` error | — write tools fail fast |
| 5 | no git repo near cwd | `dir_basename` | basename of cwd |

Child scan constraints: depth=1, max 20 entries, 200ms timeout, skips hidden dirs and noise dirs (`node_modules`, `vendor`, `.venv`, `__pycache__`, `target`, `dist`, `build`, `.idea`, `.vscode`).

### Response envelope

Every successful tool response includes these fields:

```json
{
  "project": "engram",
  "project_source": "git_remote",
  "project_path": "/home/user/engram",
  "result": "...(tool output)..."
}
```

Error responses include `available_projects` when the error is `ambiguous_project` or `unknown_project`.

### Write tools (cwd-detected project, limited recovery override)

`mem_session_start`, `mem_session_end`, `mem_session_summary`, `mem_capture_passive`, and `mem_update` auto-detect project from cwd. Any `project` argument the LLM sends is ignored.

`mem_save` and `mem_save_prompt` also auto-detect project from cwd by default, but they accept one narrow recovery override: after a previous `ambiguous_project` error, the agent may retry with `project=<one of available_projects>` and `project_choice_reason=user_selected_after_ambiguous_project`. Without that exact reason, LLM-supplied `project` is ignored so routine writes cannot drift across project names.

### Read tools (optional project override)

`mem_search`, `mem_context`, `mem_timeline`, `mem_get_observation`, `mem_stats` — `project` is an optional argument. If supplied, it is validated against the store via `ProjectExists`. Unknown project names return a structured error with `available_projects`.

### Admin tools (project required)

`mem_delete`, `mem_merge_projects` — `project` remains required. Auto-detection is NOT applied to destructive operations.

### mem_current_project

Use `mem_current_project` as the first call in a session to inspect the detection result:

```json
{
  "project": "engram",
  "project_source": "git_remote",
  "project_path": "/home/user/engram",
  "cwd": "/home/user/engram",
  "available_projects": [],
  "warning": ""
}
```

Returns success even when cwd is ambiguous — empty `project` + non-empty `available_projects` signals the agent to navigate to a specific repo before writing.

---

## MCP Tools (18 tools)

### mem_search

Search persistent memory across all sessions. Supports FTS5 full-text search with type/project/scope/limit filters.

When an observation has judged relations in `memory_relations`, the result entry includes annotation lines immediately after the title/content block:

```
supersedes: #<id> (<title>)       — this memory supersedes another
superseded_by: #<id> (<title>)    — another memory supersedes this one
conflicts: #<id> (<title>)        — judged conflict with another memory
conflict: contested by #<id> (pending)  — pending (not yet judged)
```

Multiple annotation lines appear when multiple relations apply — one per related observation. Titles are retrieved via JOIN (no N+1 queries). When the related observation has been deleted, `(deleted)` replaces the title. Agent parsers should match by prefix — these prefixes are stable across versions (REQ-012).

Pending relations (from `mem_save` conflict surfacing, before `mem_judge` is called) produce the `conflict: contested by #<id> (pending)` form. Judged relations produce the enriched form with title.

### mem_save

Save structured observations. The tool description teaches agents the format:

- **title**: Short, searchable (e.g. "JWT auth middleware")
- **type**: `decision` | `architecture` | `bugfix` | `pattern` | `config` | `discovery` | `learning`
- **scope**: `project` (default) | `personal`
- **topic_key**: optional canonical topic id (e.g. `architecture/auth-model`) used to upsert evolving memories
- **capture_prompt**: optional boolean, default `true`; when current prompt context is available in the same MCP process for the same project/session, Engram best-effort records it alongside the observation. If that process-local context is unavailable or prompt capture fails, `mem_save` still succeeds. Automated pipeline saves such as SDD artifacts should pass `false`.
- **content**: Structured with `**What**`, `**Why**`, `**Where**`, `**Learned**`

Exact duplicate saves are deduplicated in a rolling time window using a normalized content hash + project + scope + type + title.
When `topic_key` is provided, `mem_save` upserts the latest observation in the same `project + scope + topic_key`, incrementing `revision_count`.

### mem_update

Update an observation by ID. Supports partial updates for `title`, `content`, `type`, `project`, `scope`, and `topic_key`.

### mem_suggest_topic_key

Suggest a stable `topic_key` from `type + title` (or content fallback). Uses family heuristics like `architecture/*`, `bug/*`, `decision/*`, etc. Use before `mem_save` when you want evolving topics to upsert into a single observation.

### mem_delete

Delete an observation by ID. Uses soft-delete by default (`deleted_at`); optional hard-delete for permanent removal.

### mem_save_prompt

Save user prompts — records what the user asked so future sessions have context about user goals.
When called in the same MCP process, this also feeds process-local current prompt context used by later `mem_save` calls with `capture_prompt=true`. The same MCP process lifecycle must receive the prompt context before the later save; prompt capture is best-effort and `mem_save` still succeeds when no context is available.

### mem_context

Get recent memory context from previous sessions — shows sessions, prompts, and observations, with optional scope filtering for observations.

### mem_stats

Show memory system statistics — sessions, observations, prompts, projects.

### mem_timeline

Progressive disclosure: after searching, drill into chronological context around a specific observation. Shows N observations before and after within the same session.

### mem_get_observation

Get full untruncated content of a specific observation by ID.

### mem_session_summary

Save comprehensive end-of-session summary:

```
## Goal
## Instructions
## Discoveries
## Accomplished
## Relevant Files
```

### mem_session_start

Register the start of a new coding session.

### mem_session_end

Mark a session as completed with optional summary.

### mem_capture_passive

Extract structured learnings from text output. Looks for `## Key Learnings:` sections and saves each numbered/bulleted item as a separate observation. Duplicates are automatically skipped.

### mem_merge_projects

**Admin tool.** Merge multiple project name variants into a single canonical name. Accepts an array of source project names and a target canonical name. All observations, sessions, and prompts from the source projects are reassigned to the canonical project.

### mem_current_project

Detect the current project from the working directory. Returns `project`, `project_source`, `project_path`, `cwd`, `available_projects`, and `warning`. Never returns an error — even on ambiguous cwd it returns success with an empty `project` and non-empty `available_projects`. Recommended as the first call when starting a session.

### mem_judge

Record a verdict on a pending memory conflict. When `mem_save` returns `candidates[]` and `judgment_required: true`, the agent inspects the candidates and calls `mem_judge` to mark the relation between the saved memory and a candidate.

Parameters:
- **judgment_id** (required): the `judgment_id` returned by `mem_save`
- **relation** (required): `related` | `compatible` | `scoped` | `conflicts_with` | `supersedes` | `not_conflict`
- **reason** (required): short text explaining the verdict
- **evidence** (optional): free-form text or JSON the agent can use to justify the call (e.g., quoted excerpts from both memories)
- **confidence** (optional, default 1.0): 0.0–1.0; if the value is below 0.7 the agent SHOULD ask the user before calling

Re-judging an existing relation overwrites it (deliberate revision). Two agents judging the same pair persist as separate rows — Phase 1 surfaces both; cross-actor reconciliation is Phase 2.

Search results subsequently expose annotation lines like `supersedes: #<id> (<title>)`, `superseded_by: #<id> (<title>)`, and `conflicts: #<id> (<title>)` so the recalling agent sees relevant verdicts at-a-glance. For enrolled projects with autosync enabled, judgments propagate to other machines via the cloud mutation pipeline — the annotation appears in `mem_search` results on any machine that has pulled the relevant mutations.

### mem_compare

Records a verdict on a semantic comparison between two memories. The agent reads both memories, judges the relationship using its LLM reasoning, and calls `mem_compare` to persist the verdict. Unlike `mem_judge` (which resolves a pre-existing `pending` candidate surfaced by `mem_save`), `mem_compare` creates a new relation row directly — useful for proactive semantic analysis that goes beyond FTS5 lexical matching.

Available in the `agent` profile (`engram mcp --tools=agent`).

Parameters:
- **memory_id_a** (required): int — observation ID of the first memory
- **memory_id_b** (required): int — observation ID of the second memory
- **relation** (required): string — one of `conflicts_with` | `supersedes` | `scoped` | `related` | `compatible` | `not_conflict`
- **confidence** (required): float 0.0..1.0
- **reasoning** (required): string — explanation of the verdict (max 200 chars)
- **model** (optional): string — model name for provenance (e.g. `"claude-haiku-4-5"`)

Behavior:
- Persists a relation row via `JudgeBySemantic` with system provenance (`marked_by_kind="system"`, `marked_by_actor="engram"`)
- Idempotent: the same `(source_id, target_id)` pair updates the existing row rather than inserting a duplicate
- `not_conflict` verdicts are no-ops — acknowledged but not persisted, matching the scan flow contract
- Cross-project relations are rejected with an error

---

## Memory Protocol

The Memory Protocol teaches agents **when** and **how** to use Engram's MCP tools. Without it, the agent has the tools but no behavioral guidance. Add this to your agent's prompt file (see [Agent Setup](docs/AGENT-SETUP.md) for per-agent locations).

### WHEN TO SAVE (mandatory)

Call `mem_save` IMMEDIATELY after any of these:
- Bug fix completed
- Architecture or design decision made
- Non-obvious discovery about the codebase
- Configuration change or environment setup
- Pattern established (naming, structure, convention)
- User preference or constraint learned

Format for `mem_save`:
- **title**: Verb + what — short, searchable (e.g. "Fixed N+1 query in UserList", "Chose Zustand over Redux")
- **type**: `bugfix` | `decision` | `architecture` | `discovery` | `pattern` | `config` | `preference`
- **scope**: `project` (default) | `personal`
- **topic_key** (optional, recommended for evolving decisions): stable key like `architecture/auth-model`
- **content**:
  ```
  **What**: One sentence — what was done
  **Why**: What motivated it (user request, bug, performance, etc.)
  **Where**: Files or paths affected
  **Learned**: Gotchas, edge cases, things that surprised you (omit if none)
  ```

### Topic update rules (mandatory)

- Different topics must not overwrite each other (e.g. architecture vs bugfix)
- Reuse the same `topic_key` to update an evolving topic instead of creating new observations
- If unsure about the key, call `mem_suggest_topic_key` first and then reuse it
- Use `mem_update` when you have an exact observation ID to correct

### WHEN TO SEARCH MEMORY

When the user asks to recall something — any variation of "remember", "recall", "what did we do", "how did we solve", "recordar", "acordate", or references to past work:
1. First call `mem_context` — checks recent session history (fast, cheap)
2. If not found, call `mem_search` with relevant keywords (FTS5 full-text search)
3. If you find a match, use `mem_get_observation` for full untruncated content

Also search memory PROACTIVELY when:
- Starting work on something that might have been done before
- The user mentions a topic you have no context on — check if past sessions covered it

### SESSION CLOSE PROTOCOL (mandatory)

Before ending a session or saying "done" / "listo" / "that's it", you MUST call `mem_session_summary` with this structure:

```
## Goal
[What we were working on this session]

## Instructions
[User preferences or constraints discovered — skip if none]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done — for the next session]

## Relevant Files
- path/to/file — [what it does or what changed]
```

This is NOT optional. If you skip this, the next session starts blind.

### PASSIVE CAPTURE

When completing a task, include a `## Key Learnings:` section at the end of your response with numbered items. Engram will automatically extract and save these as observations.

Example:
```
## Key Learnings:

1. bcrypt cost=12 is the right balance for our server performance
2. JWT refresh tokens need atomic rotation to prevent race conditions
```

You can also call `mem_capture_passive(content)` directly with any text that contains a learning section.

### AFTER COMPACTION

If you see a message about compaction or context reset:
1. IMMEDIATELY call `mem_session_summary` with the compacted summary content
2. Then call `mem_context` to recover additional context from previous sessions
3. Only THEN continue working

Do not skip step 1. Without it, everything done before compaction is lost from memory.

---

## Project Name Normalization

Engram automatically prevents project name drift — the same project saved under different names (`"engram"` vs `"Engram"` vs `"engram-memory"`) by different clients or users.

### Automatic normalization

All project names are normalized on write and read: **lowercase**, **trimmed**, **collapsed hyphens/underscores**. If a name is changed during normalization, a warning is included in the response.

### Auto-detection

The MCP server auto-detects the project from the current working directory using a priority chain:
1. Git remote origin URL (extracts repo name)
2. Git repository root directory name
3. Current working directory basename

### Similar-project warnings

When saving to a project that doesn't exist yet, Engram checks for similar existing project names (Levenshtein distance, substring, case-insensitive matching) and warns the agent if a likely variant already exists.

### Retroactive cleanup

Use `mem_merge_projects` for agent-driven consolidation of project name variants.

---

## Features

### Full-Text Search (FTS5)

- Searches across title, content, tool_name, type, and project
- Query sanitization: wraps each word in quotes to avoid FTS5 syntax errors
- Supports type and project filters

### Timeline (Progressive Disclosure)

Three-layer pattern for token-efficient memory retrieval:

1. `mem_search` — Find relevant observations
2. `mem_timeline` — Drill into chronological neighborhood of a result
3. `mem_get_observation` — Get full untruncated content

### Privacy Tags

`<private>...</private>` content is stripped at the store layer via `stripPrivateTags()` inside `AddObservation()` and `AddPrompt()`.

Example: `Set up API with <private>sk-abc123</private>` becomes `Set up API with [REDACTED]`

### User Prompt Storage

Separate table captures what the USER asked (not just tool calls). Gives future sessions the "why" behind the "what". Full FTS5 search support.

---

## Design Decisions

1. **SQLite + FTS5 over vector DB** — FTS5 covers 95% of use cases. No ChromaDB/Pinecone complexity.
2. **Agent-driven compression** — The agent already has an LLM. No separate compression service.
3. **Privacy at the store layer** — `<private>` tags are stripped before any DB write.
4. **Pure Go SQLite (modernc.org/sqlite)** — No CGO means true cross-platform binary distribution.
5. **No raw tool-call auto-capture** — The agent saves curated summaries; `mem_save` may best-effort capture process-local prompt context tied to that save, but Engram does not ingest raw tool-call firehoses.

---

## Dependencies

### Go

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/mark3labs/mcp-go` | v0.44.0 | MCP protocol implementation |
| `modernc.org/sqlite` | v1.45.0 | Pure Go SQLite driver (no CGO) |

---

## Next Steps

- [Architecture](docs/ARCHITECTURE.md) — How it works under the hood
- [Doctor](docs/DOCTOR.md) — Operational diagnostics and safe repair
