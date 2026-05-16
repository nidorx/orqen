package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coder/acp-go-sdk"
)

// Client implements the ACP client interface, providing tool execution,
// file operations, and terminal management capabilities.
type Client struct {
	logger    Logger
	terminals *TerminalManager
}

func (c *Client) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// Prefer an allow option if present; otherwise choose the first option.
	for _, o := range params.Options {
		if o.Kind == acp.PermissionOptionKindAllowOnce || o.Kind == acp.PermissionOptionKindAllowAlways {
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{
						OptionId: o.OptionId,
					},
				},
			}, nil
		}
	}
	if len(params.Options) > 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Selected: &acp.RequestPermissionOutcomeSelected{
					OptionId: params.Options[0].OptionId,
				},
			},
		}, nil
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Cancelled: &acp.RequestPermissionOutcomeCancelled{},
		},
	}, nil
}

func (c *Client) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update
	cs := ClientSessionGet(params.SessionId)

	switch {
	case u.AgentThoughtChunk != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.UserMessageChunk != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.AgentMessageChunk != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.ToolCall != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.ToolCallUpdate != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.SessionInfoUpdate != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.Plan != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}
	case u.AvailableCommandsUpdate != nil:
		if cs != nil {
			cs.SessionUpdate(ctx, params)
		}

	case u.CurrentModeUpdate != nil:
		// The current mode of the session has changed
		//
		// See protocol docs: [Session Modes](https://agentclientprotocol.com/protocol/session-modes)

		c.logger.Log("CurrentModeUpdate\n")

	case u.ConfigOptionUpdate != nil:
		// Session configuration options have been updated.

		c.logger.Log("ConfigOptionUpdate\n")

	case u.UsageUpdate != nil:
		// **UNSTABLE**
		//
		// This capability is not part of the spec yet, and may be removed or changed at any point.
		//
		// Context window and cost update for the session.

		c.logger.Log("UsageUpdate\n")
	}

	return nil
}

// ReadTextFile reads text file content with optional line-based pagination.
//
// This implementation uses bufio.Scanner for memory-efficient line-by-line reading,
// avoiding loading the entire file into memory. It supports the ACP protocol's
// line/limit parameters for targeted partial reads:
//   - Line: 1-based starting line number (optional)
//   - Limit: maximum number of lines to return (optional)
//
// Performance characteristics:
//   - Memory: O(L) where L = number of requested lines, not file size
//   - I/O: Sequential read with early exit once limit is reached
//   - Suitable for files of any size, including multi-gigabyte log files
//
// Edge cases:
//   - If Line exceeds total lines: returns empty content
//   - If Limit exceeds remaining lines: returns available lines
//   - If Line/Limit not specified: returns entire file
//
// See protocol docs: [Read Text File](https://agentclientprotocol.com/protocol/file-system#reading-files)
func (c *Client) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}

	file, err := os.Open(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("open %s: %w", params.Path, err)
	}
	defer file.Close()

	// Calculate start line (1-based) and limit
	startLine := 1
	if params.Line != nil && *params.Line > 0 {
		startLine = *params.Line
	}

	limit := 0 // 0 means no limit
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}

	// Configure scanner with larger buffer for better performance
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 64KB initial, 1MB max

	var result strings.Builder
	currentLine := 0
	linesCollected := 0

	for scanner.Scan() {
		currentLine++

		// Skip lines before start
		if currentLine < startLine {
			continue
		}

		// Stop if we've collected enough lines
		if limit > 0 && linesCollected >= limit {
			break
		}

		// Add newline separator between lines
		if linesCollected > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(scanner.Text())
		linesCollected++
	}

	if err := scanner.Err(); err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}

	return acp.ReadTextFileResponse{Content: result.String()}, nil
}

// WriteTextFile Request to write content to a text file.
//
// Only available if the client supports the 'fs.writeTextFile' capability.
func (c *Client) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	dir := filepath.Dir(params.Path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return acp.WriteTextFileResponse{}, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write %s: %w", params.Path, err)
	}
	return acp.WriteTextFileResponse{}, nil
}

// CreateTerminal creates a new terminal session and starts the specified command.
// The command runs in the background with stdout/stderr captured in memory buffers;
// output never leaks to the parent process's terminal.
//
// See protocol docs: [Terminal Create](https://agentclientprotocol.com/protocol/terminals)
func (c *Client) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	// Convert env from ACP format to internal format
	env := make([]struct{ Name, Value string }, 0, len(params.Env))
	for _, e := range params.Env {
		env = append(env, struct{ Name, Value string }{Name: e.Name, Value: e.Value})
	}

	cwd := ""
	if params.Cwd != nil {
		cwd = *params.Cwd
	}

	terminalID, err := c.terminals.CreateSession(
		ctx,
		params.Command,
		params.Args,
		env,
		cwd,
		params.OutputByteLimit,
	)
	if err != nil {
		return acp.CreateTerminalResponse{}, fmt.Errorf("create terminal: %w", err)
	}

	return acp.CreateTerminalResponse{TerminalId: terminalID}, nil
}

// TerminalOutput polls the captured output of a terminal session.
// If the command has finished, ExitStatus is populated with the exit code
// and/or signal. Output is truncated from the beginning if it exceeds the
// configured byte limit.
//
// See protocol docs: [Terminal Output](https://agentclientprotocol.com/protocol/terminals)
func (c *Client) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	output, truncated, exitStatusInternal, err := c.terminals.TerminalOutput(params.TerminalId)
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}

	resp := acp.TerminalOutputResponse{
		Output:    output,
		Truncated: truncated,
	}

	if exitStatusInternal != nil {
		resp.ExitStatus = &acp.TerminalExitStatus{
			ExitCode: exitStatusInternal.ExitCode,
			Signal:   exitStatusInternal.Signal,
		}
	}

	return resp, nil
}

// ReleaseTerminal kills the running process (if active), frees all resources,
// and permanently invalidates the terminal ID. If the terminal is embedded in
// a tool call, the client should continue displaying its output visually.
//
// See protocol docs: [Terminal Release](https://agentclientprotocol.com/protocol/terminals)
func (c *Client) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	if err := c.terminals.ReleaseTerminal(params.TerminalId); err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	return acp.ReleaseTerminalResponse{}, nil
}

// WaitForTerminalExit blocks until the terminal command completes or the
// context is cancelled. It returns the exit code and signal that terminated
// the process. This method does not block the parent execution — it uses a
// context-aware select to respect cancellation.
//
// See protocol docs: [Terminal Wait For Exit](https://agentclientprotocol.com/protocol/terminals)
func (c *Client) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	exitCode, signal, err := c.terminals.WaitForExit(ctx, params.TerminalId)
	if err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}

	return acp.WaitForTerminalExitResponse{
		ExitCode: exitCode,
		Signal:   signal,
	}, nil
}

// KillTerminal sends a termination signal to the running process. On Unix-like
// systems it sends SIGINT with a grace period before SIGKILL; on Windows it
// calls Process.Kill() directly. The terminal ID remains valid after kill —
// call ReleaseTerminal afterward to free resources.
//
// See protocol docs: [Terminal Kill](https://agentclientprotocol.com/protocol/terminals)
func (c *Client) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	if err := c.terminals.KillTerminal(params.TerminalId); err != nil {
		return acp.KillTerminalResponse{}, err
	}
	return acp.KillTerminalResponse{}, nil
}
