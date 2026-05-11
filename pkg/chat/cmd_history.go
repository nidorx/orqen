package chat

import (
	"context"
	"fmt"
	"strings"
)

const historyLimit = 10

func init() {
	RegisterCommand(CommandDef{
		Name:        "history",
		Description: "Show recent conversation history",
		Handler:     handleHistory,
	})
}

func handleHistory(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	if userID == "" {
		return "Use /start to initialize a session first.", nil
	}

	sessionMgr := bot.SessionManager
	if sessionMgr == nil {
		return "Session manager not available.", nil
	}

	// Get active session
	session, err := sessionMgr.GetOrCreateSession(userID)
	if err != nil {
		return fmt.Sprintf("Error getting session: %v", err), nil
	}

	messages, err := sessionMgr.GetSessionHistory(session.ID, historyLimit)
	if err != nil {
		return fmt.Sprintf("Error loading history: %v", err), nil
	}

	if len(messages) == 0 {
		return "No messages in this session yet. Send a message to start chatting!", nil
	}

	var sb strings.Builder
	sb.WriteString("Recent messages:\n\n")

	for _, m := range messages {
		prefix := "You"
		if m.Role == RoleAssistant {
			prefix = "Orqen"
		} else if m.Role == RoleSystem {
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
