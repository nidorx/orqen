<p align="center">
  <strong>Persistent memory for AI coding agents</strong><br>
  <em>Forked from <a href="https://github.com/Gentleman-Programming/engram">Engram</a>, evolved independently within Orqen.</em>
</p>

---

> **engram** `/ˈen.ɡræm/` — *neuroscience*: the physical trace of a memory in the brain.

This package is a copy of [Engram](https://github.com/Gentleman-Programming/engram) to allow Orqen to persist memory across agent sessions. Many features from the original dependency have been removed, and this package will be evolved independently within Orqen.

## What was removed

The following features from the original Engram have been stripped out:

- CLI, TUI, and HTTP server — only the MCP server and store remain
- Cloud sync, cloud dashboard, and cloud autosync
- Git sync (compressed chunks)
- Agent setup plugins (OpenCode, Claude Code)
- Docker compose, deployment, and infrastructure tooling

The core that remains is the SQLite-backed memory store with FTS5 full-text search, exposed via an MCP server so any agent can read and write persistent memories.

## How It Works

```
1. Agent completes significant work (bugfix, architecture decision, etc.)
2. Agent calls mem_save → title, type, What/Why/Where/Learned
3. Engram persists to SQLite with FTS5 indexing
4. Next session: agent searches memory, gets relevant context
```

Full details on session lifecycle, topic keys, and memory hygiene → [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

## MCP Tools

| Category | Tools |
|----------|-------|
| **Save & Update** | `mem_save`, `mem_update`, `mem_delete`, `mem_suggest_topic_key` |
| **Search & Retrieve** | `mem_search`, `mem_context`, `mem_timeline`, `mem_get_observation` |
| **Session Lifecycle** | `mem_session_start`, `mem_session_end`, `mem_session_summary` |
| **Conflict Surfacing** | `mem_judge`, `mem_compare` |
| **Utilities** | `mem_save_prompt`, `mem_stats`, `mem_capture_passive`, `mem_merge_projects`, `mem_current_project`, `mem_doctor` |

Full tool reference with parameters → [DOCS.md#mcp-tools](DOCS.md#mcp-tools)

## Documentation

| Doc | Description |
|-----|-------------|
| [Architecture](docs/ARCHITECTURE.md) | How it works, session lifecycle, project structure |
| [Doctor](docs/DOCTOR.md) | Operational diagnostics and safe repair |
| [Full Docs](DOCS.md) | Complete technical reference |

## License

MIT

---

**Originally inspired by [claude-mem](https://github.com/thedotmack/claude-mem)** — forked from [Engram](https://github.com/Gentleman-Programming/engram), stripped down and evolved independently within Orqen.
