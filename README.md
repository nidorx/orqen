# Orqen

> **Execution layer for AI workflows.**

Orqen orchestrates tasks, agents, and decisions into deterministic systems. No prompts chaos. Just structured execution.

## What is Orqen?

Orqen is an open-source workflow orchestration engine that turns repetitive processes into structured, agent-driven pipelines. Inspired by Kanban, it serves not only software development but **any workflow** you want to automate — content creation, marketing, operations, and beyond.

**Key differentiator:** Orqen works with **any agent that uses the Agent Client Protocol (ACP)**, giving you freedom of choice over AI providers.

### Use Cases

- **Software Development** — Scrum-like workflows (backlog, implementation, review, retrospective)
- **Content Creation** — Ideation → script writing → review → publication
- **Marketing Pipelines** — Campaign planning, approval flows, A/B test orchestration
- **Any Repeatable Process** — If your team does it repeatedly, Orqen can structure it

## The Problem

AI-powered development tools today operate as **stateless prompt machines**. Users must manually iterate, track context in their heads, and manage decisions across conversations. There is no structured system for deterministic execution, persistent memory, or multi-project orchestration.

## The Solution

Orqen provides a **structured execution layer** that:

1. **Orchestrates AI agents** through defined workflows (lanes)
2. **Persists state** via filesystem-first design (tasks, decisions, learnings)
3. **Supports multiple projects** with independent configurations
4. **Works with any ACP-compatible agent** (Qwen, Claude, custom)

> **Not another AI tool. Infrastructure for AI execution.**

| Typical AI Tools | Orqen |
|------------------|-------|
| Prompt-based | State-driven |
| Stateless | Persistent memory |
| Manual iteration | Autonomous execution loops |
| Single project | Multi-project orchestration |

## How It Works

Orqen is a **terminal-based orchestrator** driven by a single configuration file (`.orqen/orqen.yaml`).

### Configuration

```yaml
modules:
  - name: task
    dir: ".orqen/tasks"
    lanes:
      - name: "inbox"
        purpose: "User ideas ready for task creation"
        agent_behavior:
          - "Read the inbox file"
          - "Decompose into executable tasks"
          - "Create tasks in the backlog lane"

      - name: "doing"
        purpose: "Task being implemented"
        agent_behavior:
          - "Read task and refinement documents"
          - "Implement according to specifications"
          - "Create SUMMARY artifact on completion"

      - name: "review"
        purpose: "Implementation awaiting quality review"
        agent_behavior:
          - "Validate acceptance criteria"
          - "Review code quality and security"
```

### Execution Flow

```
.orqen/
├── orqen.yaml              # Workflow definition
└── tasks/                  # Module artifacts
    ├── prompts/            # Agent prompt templates
    ├── 01_inbox/           # Lane directories
    │   └── idea.md
    ├── 02_backlog/
    │   └── TASK-0001-...
    ├── 03_ready/
    └── ...
```

1. **Load** — Orqen reads `.orqen/orqen.yaml` to understand the workflow
2. **Scan** — The engine checks lanes in priority order for available work
3. **Invoke** — An ACP agent receives a synthesized prompt (context + lane definition + work item)
4. **Execute** — The agent acts on the work item, creates artifacts, moves it to the next lane
5. **Loop** — The cycle repeats at a configurable interval

Each agent invocation is **stateless** — all context comes from the filesystem. The agent uses MCP tools (`orqen_status`, `orqen_create_item`, `orqen_move_item`, etc.) to interact with the workflow.

## Quick Start

### Prerequisites

- Go 1.21+
- An ACP-compatible agent (e.g., Qwen Code, Claude Code)

### Run Locally

```bash
git clone https://github.com/orqen/orqen.git
cd orqen
go build -o orqen ./main.go
./orqen
```

The CLI will prompt for a project directory containing `.orqen/orqen.yaml`.

### Create a Custom Workflow

Use the built-in skill at `.orqen/SKILL.md` — an agent will interview you about your workflow needs and generate a complete `orqen.yaml` configuration.

## Features

- **Customizable Lanes** — Define your own workflow stages (Kanban, Scrum, or anything else)
- **ACP Agent Support** — Works with any ACP-compatible agent (Qwen, Claude, etc.)
- **MCP Tool Server** — Agents interact via standardized tools (create, move, list, scan items)
- **Architecture Decision Records** — Structured decision tracking that governs future work
- **Learning System** — Auto-capture and apply pattern-level knowledge across tasks
- **Deterministic Execution** — No hidden state. Everything is explicit and auditable
- **Multi-Project** — Run multiple projects simultaneously, each with its own configuration
- **Open Source** — MIT License

## Documentation

| Document | Purpose |
|----------|---------|
| [Architecture](docs/ARCHITECTURE.md) | System design for developers and AI agents |
| [Branding](docs/BRANDING.md) | Visual identity, colors, typography, tone |
| [Contributing](CONTRIBUTING.md) | How to contribute to Orqen |

## Roadmap

- [x] Core concept and design system
- [x] Autopilot (shell script version) — proof of concept
- [x] Go backend with ACP protocol
- [x] Terminal-first CLI with MCP tool server
- [ ] Custom workflow creation via interactive skill
- [ ] ADR and Learning system integration
- [ ] Agent marketplace
- [ ] Template library for common workflows
- [ ] REST APIs for external integrations
- [ ] Web interface

## License

Orqen is open source under the [MIT License](LICENSE).

## Community

- **GitHub:** [github.com/orqen/orqen](https://github.com/orqen/orqen)
- **Issues:** Report bugs and request features via GitHub Issues
- **Contributing:** See [CONTRIBUTING.md](CONTRIBUTING.md)

---

**Orqen © 2026 — Execution layer for AI workflows**
