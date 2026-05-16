package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/nidorx/orqen/pkg/chat/memory"
)

const historyLimit = 10

func init() {
	Register(Command{
		Name:        "history",
		Description: "Show recent conversation history",
		Handler:     commandHistoryHandler,
	})
}

func commandHistoryHandler(ctx context.Context, req *Request) (string, error) {
	if req.ExtId == "" {
		return "**Info:** Use `/start` to initialize a session first.", nil
	}

	sessionMgr := req.SessionManager
	if sessionMgr == nil {
		return "**Error:** Session manager not available.", nil
	}

	// Get active session
	session, err := sessionMgr.GetOrCreateSession(req.ExtId)
	if err != nil {
		return fmt.Sprintf("**Error:** Error getting session: %v", err), nil
	}

	messages, err := sessionMgr.GetSessionHistory(session.ID, historyLimit)
	if err != nil {
		return fmt.Sprintf("**Error:** Error loading history: %v", err), nil
	}

	if len(messages) == 0 {
		return "No messages in this session yet. Send a message to start chatting!", nil
	}

	var sb strings.Builder
	sb.WriteString("## Recent Messages\n\n")

	for _, m := range messages {
		prefix := "**You**"
		if m.Role == memory.RoleAssistant {
			prefix = "**Orqen**"
		} else if m.Role == memory.RoleSystem {
			continue // Skip system messages
		}

		content := m.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", prefix, content))
	}

	return strings.TrimSuffix(sb.String(), "\n"), nil
}
