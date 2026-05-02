package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/acp-go-sdk"
)

var (
	sessions   = map[acp.SessionId]*ClientSession{}
	sessionsMu sync.Mutex
)

func sessionDelClient(sid acp.SessionId) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	delete(sessions, sid)
}

func sessionSetClient(sid acp.SessionId, session *ClientSession) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	sessions[sid] = session
}

func sessionGetClient(sid acp.SessionId) *ClientSession {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	return sessions[sid]
}

// Client implements the ACP client interface, providing tool execution,
// file operations, and terminal management capabilities.
type ClientSession struct {
	logger       Logger
	agentChunk   *Chunk
	userChunk    *Chunk
	thoughtChunk *Chunk
	toolCallById map[acp.ToolCallId]*acp.SessionUpdateToolCall
}

func (c *ClientSession) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	u := params.Update

	switch {
	case u.AgentThoughtChunk != nil:
		// A chunk of the agent's internal reasoning being streamed.

		c.userChunk.stop()
		c.agentChunk.stop()

		content := u.AgentThoughtChunk.Content
		if content.Text != nil {
			c.thoughtChunk.add(content.Text.Text)
		}

	case u.UserMessageChunk != nil:
		// A chunk of the user's message being streamed.

		c.agentChunk.stop()
		c.thoughtChunk.stop()

		content := u.UserMessageChunk.Content
		if content.Text != nil {
			c.userChunk.add(content.Text.Text)
		}

	case u.AgentMessageChunk != nil:
		// A chunk of the agent's response being streamed.

		c.userChunk.stop()
		c.thoughtChunk.stop()

		content := u.AgentMessageChunk.Content
		if content.Text != nil {
			c.agentChunk.add(content.Text.Text)
		}

	case u.ToolCall != nil:
		// Notification that a new tool call has been initiated.

		c.userChunk.stop()
		c.agentChunk.stop()
		c.thoughtChunk.stop()

		c.logger.Log("\033[90m(Tool call %s)\033[0m %s\n", u.ToolCall.Status, u.ToolCall.Title)

		c.toolCallById[u.ToolCall.ToolCallId] = u.ToolCall

	case u.ToolCallUpdate != nil:
		// Update on the status or results of a tool call.

		c.userChunk.stop()
		c.agentChunk.stop()
		c.thoughtChunk.stop()

		uToolCall := u.ToolCallUpdate
		sToolCall, exists := c.toolCallById[uToolCall.ToolCallId]

		title := ""
		if uToolCall.Title != nil {
			title = *uToolCall.Title
		}
		status := ""
		if uToolCall.Status != nil {
			status = string(*uToolCall.Status)
		}

		if exists && (status == string(acp.ToolCallStatusCompleted) || status == string(acp.ToolCallStatusFailed)) {
			delete(c.toolCallById, uToolCall.ToolCallId)
		}

		statusP := fmt.Sprintf("%-11s", status)

		if title != "" {
			c.logger.Log("\033[90m(Tool call %s)\033[0m %s\n", statusP, title)
		} else {

			switch v := uToolCall.RawOutput.(type) {
			case string:
				if v == "" {
					if exists {
						c.logger.Log("\033[90m(Tool call %s)\033[0m %s\n", statusP, sToolCall.Title)
					}
				} else {
					c.logger.Log("\033[90m(Tool call %s)\033[0m %s\n", statusP, v)
				}
			default:
				if exists {
					c.logger.Log("\033[90m(Tool call %s)\033[0m %s\n", statusP, sToolCall.Title)
				} else {
					c.logger.Log("\033[90m(Tool call %s)\033[0m %s\n", statusP, v)
				}
			}
		}

	case u.Plan != nil:
		// The agent's execution plan for complex tasks.
		// See protocol docs: [Agent Plan](https://agentclientprotocol.com/protocol/agent-plan)

		c.userChunk.stop()
		c.agentChunk.stop()
		c.thoughtChunk.stop()

		c.logger.Log("Plan updated\n")

	case u.AvailableCommandsUpdate != nil:
		// Available commands are ready or have changed

		c.logger.Log("AvailableCommandsUpdate\n")

	case u.CurrentModeUpdate != nil:
		// The current mode of the session has changed
		//
		// See protocol docs: [Session Modes](https://agentclientprotocol.com/protocol/session-modes)

		c.logger.Log("CurrentModeUpdate\n")

	case u.ConfigOptionUpdate != nil:
		// Session configuration options have been updated.

		c.logger.Log("ConfigOptionUpdate\n")

	case u.SessionInfoUpdate != nil:
		// Session metadata has been updated (title, timestamps, custom metadata)
		c.logger.Log("SessionInfoUpdate\n")

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
