package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatHistoryGetInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID (omit for current session)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum number of messages to return (default: 20)"`
}

type ChatHistoryGetOutput struct {
	Messages []MessageView `json:"messages"`
	Error    string        `json:"error,omitempty"`
}

const tnChatHistoryGet = "chat_history_get"

func init() {
	chatTools[tnChatHistoryGet] = &mcp.Tool{
		Description: "Get recent conversation messages from the current chat session.",
	}
}

func ChatHistoryGetHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatHistoryGetInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatHistoryGetOutput, error) {
	out := ChatHistoryGetOutput{}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	sessionID := input.SessionID
	if sessionID == "" {
		out.Error = "session_id is required"
		return nil, out, nil
	}

	msgs, err := sessionMgr.GetSessionHistory(sessionID, limit)
	if err != nil {
		out.Error = fmt.Sprintf("failed to get history: %v", err)
		return nil, out, nil
	}

	for _, m := range msgs {
		out.Messages = append(out.Messages, MessageView{
			Role:    string(m.Role),
			Content: m.Content,
			Time:    m.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, out, nil
}
