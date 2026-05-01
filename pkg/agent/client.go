package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/acp-go-sdk"
)

type GenericClient struct {
	autoApprove    bool
	thoughtStarted bool
	thoughtParts   []string
	thoughtLastMsg time.Time
	logger         Logger
}

func (c *GenericClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
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

func (c *GenericClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update

	switch {
	case u.AgentThoughtChunk != nil:
		// A chunk of the agent's internal reasoning being streamed.

		thought := u.AgentThoughtChunk.Content
		if thought.Text != nil {
			if !c.thoughtStarted {
				c.logger.Log("\033[90mThinking ...\033[0m\n")
				c.thoughtStarted = true
				c.thoughtLastMsg = time.Now()
				c.thoughtParts = append(c.thoughtParts, thought.Text.Text)
			} else {
				c.thoughtParts = append(c.thoughtParts, thought.Text.Text)
				c.checkThough()
			}
		}

	case u.UserMessageChunk != nil:
		// A chunk of the user's message being streamed.

		c.thoughtStarted = false
		c.checkThough()

		content := u.UserMessageChunk.Content
		if content.Text != nil {
			c.logger.Log("\n👤 %s\n", content.Text.Text)
		}

	case u.AgentMessageChunk != nil:
		// A chunk of the agent's response being streamed.

		c.thoughtStarted = false
		c.checkThough()

		content := u.AgentMessageChunk.Content
		if content.Text != nil {
			fmt.Printf("%s", content.Text.Text)
		}

	case u.ToolCall != nil:
		// Notification that a new tool call has been initiated.

		c.thoughtStarted = false
		c.checkThough()

		c.logger.Log("%s \033[90m(%s)\033[0m\n", u.ToolCall.Title, u.ToolCall.Status)

	case u.ToolCallUpdate != nil:
		// Update on the status or results of a tool call.

		c.thoughtStarted = false
		c.checkThough()

		title := ""
		if u.ToolCallUpdate.Title != nil {
			title = *u.ToolCallUpdate.Title
		}
		status := ""
		if u.ToolCallUpdate.Status != nil {
			status = string(*u.ToolCallUpdate.Status)
		}
		if title != "" {
			c.logger.Log("%s [%s]\n", title, status)
		} else {
			c.logger.Log("Tool call `%s` [%s]\n", u.ToolCallUpdate.ToolCallId, status)
		}

	case u.Plan != nil:
		// The agent's execution plan for complex tasks.
		// See protocol docs: [Agent Plan](https://agentclientprotocol.com/protocol/agent-plan)

		c.thoughtStarted = false
		c.checkThough()

		c.logger.Log("Plan updated\n")

	}

	// // Available commands are ready or have changed
	// AvailableCommandsUpdate *SessionAvailableCommandsUpdate `json:"-"`
	// // The current mode of the session has changed
	// //
	// // See protocol docs: [Session Modes](https://agentclientprotocol.com/protocol/session-modes)
	// CurrentModeUpdate *SessionCurrentModeUpdate `json:"-"`
	// // Session configuration options have been updated.
	// ConfigOptionUpdate *SessionConfigOptionUpdate `json:"-"`
	// // Session metadata has been updated (title, timestamps, custom metadata)
	// SessionInfoUpdate *SessionSessionInfoUpdate `json:"-"`
	// // **UNSTABLE**
	// //
	// // This capability is not part of the spec yet, and may be removed or changed at any point.
	// //
	// // Context window and cost update for the session.
	// UsageUpdate *SessionUsageUpdate `json:"-"`
	return nil
}

func (c *GenericClient) checkThough() {
	if !c.thoughtStarted {
		if len(c.thoughtParts) > 0 {
			c.logger.Log("Thinking: %s\n", strings.TrimSpace(strings.Join(c.thoughtParts, "")))
			c.thoughtParts = nil
		}
	} else {
		if c.thoughtLastMsg.Before(time.Now().Add(-10 * time.Second)) {
			c.logger.Log("\033[90mThinking ...\033[0m\n")
			c.thoughtLastMsg = time.Now()
		}
	}
}

func (c *GenericClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if !filepath.IsAbs(params.Path) {
		return acp.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	b, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}
	content := string(b)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = min(max(*params.Line-1, 0), len(lines))
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 {
			if start+*params.Limit < end {
				end = start + *params.Limit
			}
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

func (c *GenericClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
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

// Optional/UNSTABLE terminal methods: implement as no-ops for example
func (c *GenericClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{TerminalId: "term-1"}, nil
}

func (c *GenericClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{Output: "", Truncated: false}, nil
}

func (c *GenericClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *GenericClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}

// KillTerminal implements acp.Client.
func (c *GenericClient) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, nil
}
