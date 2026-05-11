package chat

import (
	"context"
	"fmt"
)

func init() {
	RegisterCommand(CommandDef{
		Name:        "new",
		Description: "Start a new conversation session",
		Handler:     handleNew,
	})
}

func handleNew(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	if userID == "" {
		return "Use /start to initialize a session first.", nil
	}

	sessionMgr := bot.SessionManager
	if sessionMgr == nil {
		return "Session manager not available.", nil
	}

	newSession, err := sessionMgr.NewSession(userID)
	if err != nil {
		return fmt.Sprintf("Error creating new session: %v", err), nil
	}

	return fmt.Sprintf("New session started! (ID: %s)\nType /help to see available commands.", newSession.ID[:8]), nil
}
