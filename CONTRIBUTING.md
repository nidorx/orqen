# Contributing to Orqen

Thank you for your interest in contributing to Orqen! This document provides guidelines for contributing to the project.

---

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Follow the project's design principles: **deterministic, legible, minimal**

---

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 20+
- Git

### Setup

```bash
# Clone the repository
git clone https://github.com/orqen/orqen.git
cd orqen

# Install Go dependencies
go mod download

# Install frontend dependencies
cd frontend
npm install
cd ..
```

### Run Locally

```bash
# Start the backend
go build -o orqen ./main.go
./orqen

# In another terminal, start the frontend
cd frontend
npm run dev
```

Access the UI at `http://localhost:3000`.

---

## Project Structure

```
orqen/
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── README.md               # Project overview
├── AGENTS.md               # AI agent instructions
├── CONTRIBUTING.md         # This file
├── docs/                   # Documentation
│   ├── PRD.md              # Product Requirements
│   ├── ARCHITECTURE-BACKEND.md  # Go backend architecture
│   ├── ARCHITECTURE-FRONTEND.md # Vue.js frontend architecture
│   ├── UI-SPEC.md          # UI component definitions
│   ├── ADR-SYSTEM.md       # ADR system documentation
│   └── LEARNING-SYSTEM.md  # Learning system documentation
├── cmd/                    # CLI commands
├── pkg/                    # Shared packages
│   ├── api/                # HTTP handlers
│   ├── service/            # Business logic
│   ├── acp/                # Agent Client Protocol
│   ├── model/              # Domain models
│   ├── store/              # Filesystem storage
│   └── event/              # Event system
├── internal/               # Internal packages
└── frontend/               # Vue.js application
    ├── src/
    │   ├── components/     # Vue components
    │   ├── stores/         # Pinia stores
    │   ├── api/            # API clients
    │   ├── views/          # Page views
    │   ├── router/         # Vue Router config
    │   ├── composables/    # Vue composables
    │   └── styles/         # Design tokens and styles
```

---

## Development Workflow

### 1. Pick an Issue

- Browse [GitHub Issues](https://github.com/orqen/orqen/issues)
- Look for `good first issue` or `help wanted` labels
- Comment on the issue to claim it

### 2. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### 3. Make Changes

- Follow existing code style and conventions
- Write tests for new functionality
- Update documentation if needed

### 4. Test

```bash
# Run Go tests
go test ./...

# Run frontend tests
cd frontend
npm run test

# Run linter
golangci-lint run

# Run frontend linter
cd frontend
npm run lint
```

### 5. Commit

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>

[optional body]
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

**Examples:**
```
feat: add ADR management UI
fix: correct lane transition validation
docs: update architecture diagram
test: add tests for task service
```

### 6. Submit a Pull Request

- Push your branch to your fork
- Open a PR against `main`
- Fill in the PR template
- Link related issues

---

## Coding Standards

### Go

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Write tests for all public functions
- Document exported functions with comments

### Vue.js

- Use Composition API (`<script setup>`)
- Follow [Vue Style Guide](https://vuejs.org/style-guide/)
- Run `npm run lint` before committing
- Write tests for components with logic
- Use design tokens from `styles/tokens.scss`

### CSS

- Use CSS variables from design tokens
- No hardcoded colors
- Follow BEM-like naming (component-scoped)
- Dark-first — light mode is optional

---

## Design Principles

When making UI/UX decisions:

1. **Deterministic over decorative** — State must be explicit
2. **Legible over creative** — Users need clarity, not flair
3. **Minimal decoration** — Function over form
4. **Dark-first** — Native dark mode
5. **Low cognitive load** — Clear hierarchy

Refer to [UI-SPEC.md](docs/UI-SPEC.md) for detailed design guidelines.

---

## Documentation

### When to Update Documentation

- New feature → Update relevant docs
- API change → Update architecture docs
- New component → Update UI-SPEC
- Decision affecting architecture → Create ADR

### Documentation Style

- Use clear, concise language
- Include code examples where applicable
- Follow existing doc structure
- Update the table of contents if adding sections

---

## Reporting Issues

### Bug Reports

Include:

- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment (OS, Go version, Node version)
- Screenshots if UI-related

### Feature Requests

Include:

- Problem statement (what's the need?)
- Proposed solution (how should it work?)
- Alternatives considered
- Why this matters for Orqen's positioning

---

## Review Process

- PRs require at least 1 approval
- All CI checks must pass (tests, linting)
- Reviewers focus on:
  - Correctness
  - Code quality
  - Adherence to design principles
  - Test coverage

---

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

---

**Orqen © 2026 — Execution layer for AI workflows**
