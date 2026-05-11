package chat

import (
	"context"
)

func init() {
	RegisterCommand(CommandDef{
		Name:        "help",
		Description: "List available commands",
		Handler:     handleHelp,
	})
}

func handleHelp(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	return ListCommands(), nil
}
