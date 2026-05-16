package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// TerminalManager manages the lifecycle of terminal sessions created via the
// ACP terminal protocol. It is safe for concurrent use.
type TerminalManager struct {
	mu       sync.RWMutex
	sessions map[string]*TerminalSession
	nextID   atomic.Uint64
}

// NewTerminalManager creates a ready-to-use terminal session manager.
func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*TerminalSession),
	}
}

// TerminalSession holds a single terminal command's process, output buffers,
// and lifecycle signals.
type TerminalSession struct {
	mu sync.RWMutex

	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer

	exited     chan struct{}
	exitCode   *int
	exitSignal *string
	released   bool

	outputByteLimit int
	truncated       bool
}

// CreateSession spawns a new terminal session, starts the process, and
// registers it in the manager. The command runs in the background; stdout
// and stderr are captured in memory buffers and never leak to the parent
// process's terminal.
//
// The returned terminal ID is unique and stable for the lifetime of the
// session.
func (tm *TerminalManager) CreateSession(
	ctx context.Context,
	command string,
	args []string,
	env []struct{ Name, Value string },
	cwd string,
	outputByteLimit *int,
) (string, error) {
	// Build the command
	cmd := exec.CommandContext(ctx, command, args...)

	// Environment variables
	if len(env) > 0 {
		envPairs := make([]string, 0, len(env))
		for _, e := range env {
			envPairs = append(envPairs, e.Name+"="+e.Value)
		}
		cmd.Env = envPairs
	}

	// Working directory
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Create session record
	terminalID := fmt.Sprintf("term-%d", tm.nextID.Add(1))
	session := &TerminalSession{
		cmd:    cmd,
		exited: make(chan struct{}),
	}
	if outputByteLimit != nil && *outputByteLimit > 0 {
		session.outputByteLimit = *outputByteLimit
	}

	// Capture stdout/stderr in buffers (never inherited)
	cmd.Stdout = &session.stdout
	cmd.Stderr = &session.stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start terminal command %q: %w", command, err)
	}

	// Register before launching monitor goroutine so that
	// WaitForTerminalExit / TerminalOutput can find it immediately
	tm.mu.Lock()
	tm.sessions[terminalID] = session
	tm.mu.Unlock()

	// Background goroutine: wait for process exit, capture exit status
	go func() {
		err := cmd.Wait()
		session.mu.Lock()
		defer session.mu.Unlock()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				session.exitCode = &code
				if runtime.GOOS != "windows" {
					if ws, ok := exitErr.ProcessState.Sys().(interface{ Signal() os.Signal }); ok {
						sig := ws.Signal().String()
						session.exitSignal = &sig
					}
				}
			} else {
				// Process killed or context cancelled — set signal
				sig := "SIGKILL"
				session.exitSignal = &sig
			}
		} else {
			code := cmd.ProcessState.ExitCode()
			session.exitCode = &code
		}

		close(session.exited)
	}()

	return terminalID, nil
}

// GetTerminal retrieves a session by ID. Returns an error if the terminal
// does not exist or has been released.
func (tm *TerminalManager) GetTerminal(terminalID string) (*TerminalSession, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	s, ok := tm.sessions[terminalID]
	if !ok {
		return nil, fmt.Errorf("terminal %q not found", terminalID)
	}

	s.mu.RLock()
	released := s.released
	s.mu.RUnlock()

	if released {
		return nil, fmt.Errorf("terminal %q has been released", terminalID)
	}

	return s, nil
}

// KillTerminal sends a termination signal to the process. On Unix-like systems
// it sends SIGINT first, then SIGKILL after a short grace period. On Windows
// it calls Process.Kill() directly. The terminal ID remains valid after kill;
// call releaseTerminal to free resources.
func (tm *TerminalManager) KillTerminal(terminalID string) error {
	s, err := tm.GetTerminal(terminalID)
	if err != nil {
		return err
	}

	s.mu.RLock()
	cmd := s.cmd
	exited := s.exited
	s.mu.RUnlock()

	// Check if already exited by trying a non-blocking read on exited channel
	select {
	case <-exited:
		// Process already exited, nothing to kill
		return nil
	default:
	}

	// Kill the process
	if runtime.GOOS == "windows" {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	} else {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
			// Give a short grace period, then force kill
			go func() {
				timer := time.NewTimer(2 * time.Second)
				defer timer.Stop()
				select {
				case <-exited:
					// Process exited gracefully
				case <-timer.C:
					_ = cmd.Process.Kill()
				}
			}()
		}
	}

	return nil
}

// ReleaseTerminal kills the running process (if active), marks the session
// as released, and removes it from the manager. The terminal ID is
// permanently invalidated after this call.
func (tm *TerminalManager) ReleaseTerminal(terminalID string) error {
	tm.mu.Lock()
	s, ok := tm.sessions[terminalID]
	if !ok {
		tm.mu.Unlock()
		return fmt.Errorf("terminal %q not found", terminalID)
	}
	delete(tm.sessions, terminalID)
	tm.mu.Unlock()

	s.mu.Lock()
	if s.released {
		s.mu.Unlock()
		return nil // already released, idempotent
	}
	s.released = true
	cmd := s.cmd
	s.mu.Unlock()

	// Kill process if still running
	if cmd.Process != nil {
		select {
		case <-s.exited:
			// Already exited
		default:
			_ = cmd.Process.Kill()
			// Wait so the goroutine can clean up properly (prevents zombie)
			go func() {
				<-s.exited
			}()
		}
	}

	return nil
}

// TerminalOutput reads the captured output buffer for a terminal session.
// If the process has exited, exitStatus is populated. The output is
// truncated from the beginning if it exceeds the configured byte limit.
func (tm *TerminalManager) TerminalOutput(terminalID string) (output string, truncated bool, exitStatus *struct {
	ExitCode *int
	Signal   *string
}, err error) {
	s, err := tm.GetTerminal(terminalID)
	if err != nil {
		return "", false, nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	raw := s.stdout.String()
	limit := s.outputByteLimit
	truncated = s.truncated

	// Apply byte limit truncation from the beginning
	if limit > 0 && len(raw) > limit {
		raw = raw[len(raw)-limit:]
		truncated = true
		s.truncated = true
	}

	output = raw

	// Build exit status if process has exited
	select {
	case <-s.exited:
		exitStatus = &struct {
			ExitCode *int
			Signal   *string
		}{
			ExitCode: s.exitCode,
			Signal:   s.exitSignal,
		}
	default:
		// Still running
	}

	return output, truncated, exitStatus, nil
}

// WaitForExit blocks until the terminal process exits or the context is
// cancelled. Returns the exit code and signal that terminated the process.
func (tm *TerminalManager) WaitForExit(ctx context.Context, terminalID string) (exitCode *int, signal *string, err error) {
	s, err := tm.GetTerminal(terminalID)
	if err != nil {
		return nil, nil, err
	}

	s.mu.RLock()
	exited := s.exited
	s.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-exited:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exitCode, s.exitSignal, nil
}
