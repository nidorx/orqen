[← Back to README](../README.md)

# Architecture

- [How It Works](#how-it-works)
- [Session Lifecycle](#session-lifecycle)
- [MCP Tools](#mcp-tools)
- [Progressive Disclosure](#progressive-disclosure-3-layer-pattern)
- [Memory Hygiene](#memory-hygiene)
- [Topic Key Workflow](#topic-key-workflow-recommended)

> **Note:** This package is a fork of [Engram](https://github.com/Gentleman-Programming/engram). The CLI, TUI, HTTP server, cloud sync, git sync, and agent plugin features from the original have been removed. This document covers the parts that remain.

---

## How It Works

Engram trusts the **agent** to decide what's worth remembering — not a firehose of raw tool calls.

### The Agent Saves, Engram Stores

```
1. Agent completes significant work (bugfix, architecture decision, etc.)
2. Agent calls mem_save with a structured summary:
   - title: "Fixed N+1 query in user list"
   - type: "bugfix"
   - content: What/Why/Where/Learned format
3. Engram persists to SQLite with FTS5 indexing
4. Next session: agent searches memory, gets relevant context
```

---

## Session Lifecycle

```
Session starts → Agent works → Agent saves memories proactively
                                    ↓
Session ends → Agent writes session summary (Goal/Discoveries/Accomplished/Files)
                                    ↓
Next session starts → Previous session context is injected automatically
```

---

## MCP Tools

| Tool | Purpose |
|------|---------|
| `mem_save` | Save a structured observation (decision, bugfix, pattern, etc.); best-effort captures process-local current prompt context when available unless `capture_prompt=false` |
| `mem_update` | Update an existing observation by ID |
| `mem_delete` | Delete an observation (soft-delete by default, hard-delete optional) |
| `mem_suggest_topic_key` | Suggest a stable `topic_key` for evolving topics before saving |
| `mem_search` | Full-text search across all memories |
| `mem_session_summary` | Save end-of-session summary |
| `mem_context` | Get recent context from previous sessions |
| `mem_timeline` | Chronological context around a specific observation |
| `mem_get_observation` | Get full content of a specific memory |
| `mem_save_prompt` | Save a user prompt for future context |
| `mem_stats` | Memory system statistics |
| `mem_session_start` | Register a session start |
| `mem_session_end` | Mark a session as completed |
| `mem_capture_passive` | Extract learnings from text output |
| `mem_merge_projects` | Merge project name variants into canonical name (admin) |
| `mem_current_project` | Detect project from cwd — never errors, recommended first call |
| `mem_judge` | Record a verdict on a pending memory conflict |
| `mem_compare` | Record a verdict on a semantic comparison between two memories |
| `mem_doctor` | Run operational diagnostics against the local SQLite store |

---

## Progressive Disclosure (3-Layer Pattern)

Token-efficient memory retrieval — don't dump everything, drill in:

```
1. mem_search "auth middleware"     → compact results with IDs (~100 tokens each)
2. mem_timeline observation_id=42  → what happened before/after in that session
3. mem_get_observation id=42       → full untruncated content
```

---

## Memory Hygiene

- `mem_save` now supports `scope` (`project` default, `personal` optional)
- `mem_save` also supports `topic_key`; with a topic key, saves become upserts (same project+scope+topic updates the existing memory)
- `mem_save` supports `capture_prompt` (`true` by default). When the same MCP process lifecycle has current prompt context for the same project and session, it best-effort records that prompt alongside the observation. The prompt context must be fed before the later `mem_save` (typically via `mem_save_prompt`); `mem_save` still succeeds if context is unavailable or prompt capture fails. Automated saves such as SDD artifacts should pass `capture_prompt=false`.
- Exact dedupe prevents repeated inserts in a rolling window (hash + project + scope + type + title)
- Duplicates update metadata (`duplicate_count`, `last_seen_at`, `updated_at`) instead of creating new rows
- Topic upserts increment `revision_count` so evolving decisions stay in one memory
- `mem_delete` uses soft-delete by default (`deleted_at`), with optional hard delete
- `mem_search`, `mem_context`, recent lists, and timeline ignore soft-deleted observations

---

## Topic Key Workflow (Recommended)

Use this when a topic evolves over time (architecture, long-running feature decisions, etc.):

```text
1. mem_suggest_topic_key(type="architecture", title="Auth architecture")
2. mem_save(..., topic_key="architecture-auth-architecture")
3. Later change on same topic -> mem_save(..., same topic_key)
   => existing observation is updated (revision_count++)
```

Different topics should use different keys (e.g. `architecture/auth-model` vs `bug/auth-nil-panic`) so they never overwrite each other.

`mem_suggest_topic_key` now applies a family heuristic for consistency across sessions:

- `architecture/*` for architecture/design/ADR-like changes
- `bug/*` for fixes, regressions, errors, panics
- `decision/*`, `pattern/*`, `config/*`, `discovery/*`, `learning/*` when detected

---

## Next Steps

- [Doctor](../docs/DOCTOR.md) — Operational diagnostics and safe repair
- [Full Docs](../DOCS.md) — Complete technical reference
