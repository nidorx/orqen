# Scripts

This project does not use Bash, Batch, or PowerShell for maintenance tasks. All operational scripts are written in Go and live in this directory.

## Why Go instead of Bash

A single Go program replaces platform-specific shell scripts, giving us:

- **One codebase, every platform** — no duplicate `.sh` / `.bat` / `.ps1` files.
- **Familiar conventions** — anyone who reads the application code can read and modify the scripts.
- **Full access to the Go ecosystem** — no glue code to call out to external tools for JSON parsing, HTTP calls, or file manipulation.

This approach is inspired by [Krzysztof Kowalczyk's article on using Go instead of Bash for project scripts](https://blog.kowalczyk.info/article/4b1f9201181340099b698246857ea98d/using-go-instead-of-bash-for-scripts.html).

## Conventions

- Each script is invoked from the project root via `go run ./do [flags]`.
- On startup, scripts `chdir` to the project root so paths are always relative to it.
- Errors are handled with a `must()` helper that panics on failure — acceptable for short-lived tooling and prints a clear stack trace.
- Command dispatch uses Go's `flag` package: each flag maps to a dedicated function.

## Utilities

Scripts in this directory make use of [`kjk/common/u`](https://github.com/kjk/common/tree/main/u) — a lightweight utility package by the same author of the article above. It provides helpers for:

| Category | File | Purpose |
|----------|------|---------|
| Command execution | [`exec.go`](./exec.go) | Run external commands, capture/stream output |
| File operations | [`file.go`](./file.go) | File I/O helpers |
| In-memory filesystem | [`memfs.go`](https://github.com/kjk/common/blob/main/u/memfs.go) | Simulated in-memory filesystem for testing |
| Compression | [`compress.go`](https://github.com/kjk/common/blob/main/u/compress.go) | Compress / decompress data |
| String helpers | [`strings.go`](https://github.com/kjk/common/blob/main/u/strings.go) | Text manipulation utilities |
| Word count | [`wc.go`](https://github.com/kjk/common/blob/main/u/wc.go) | Text-stream counting |
| Debouncer | [`debouncer.go`](https://github.com/kjk/common/blob/main/u/debouncer.go) | Rate-limit or delay function calls |
| File diffing | [`winmerge_diff.go`](https://github.com/kjk/common/blob/main/u/winmerge_diff.go) | WinMerge integration for file diffs |
| Core / misc | [`base.go`](./base.go), [`misc.go`](https://github.com/kjk/common/blob/main/u/misc.go) | Foundational helpers |

## Quick Start

```bash
# From the project root, run any script:
go run ./do -flag

# Build binaries (replaces build-podman.ps1):
go run ./do -build                        # all platforms (default)
go run ./do -build -mac                   # macOS only
go run ./do -build -windows -linux        # Windows + Linux
go run ./do -build -mac -version "v1.0.0" # custom version tag

# Or use a thin wrapper (if available):
./do.sh -build -flags        # Linux / macOS
./do.bat -build -flags       # Windows
```

### Build flags

| Flag | Description |
|------|-------------|
| `-build` | Trigger the build pipeline |
| `-version <string>` | Version embedded in binaries via ldflags (default: `v1.0.0`) |
| `-windows` | Build Windows amd64 (`.exe`) |
| `-linux` | Build Linux amd64 (ELF binary) |
| `-mac` | Build macOS amd64 + arm64 (`.pkg` installers) |

Outputs are written to `.dist/`.

Add a new flag and its handler function to extend what the scripts can do.
