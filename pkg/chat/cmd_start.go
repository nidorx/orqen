package chat

import (
	"context"
	"fmt"
)

func init() {
	RegisterCommand(CommandDef{
		Name:        "start",
		Description: "Initialize chat session and welcome message",
		Handler:     handleStart,
	})
}

func handleStart(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	return fmt.Sprintf(
		"Welcome to Orqen Chat! 🤖\n\n"+
			"I'm your remote assistant for the Orqen project.\n"+
			"You can ask me to create workitems, check status, explore files, and more.\n\n"+
			"Type /help to see available commands.",
	), nil
}
