package chat

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	RegisterCommand(CommandDef{
		Name:        "lanes",
		Description: "List available lanes",
		Handler:     handleLanes,
	})
}

func handleLanes(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	proj := bot.Project
	if proj == nil {
		return "No project loaded.", nil
	}

	var sb strings.Builder
	sb.WriteString("Available Lanes:\n\n")

	for _, mod := range proj.Modules {
		sb.WriteString(fmt.Sprintf("Module: %s\n", mod.Name))
		for _, lane := range mod.GetLanesInOrder() {
			count := lane.CountWorkItems()
			active := lane.CountActiveWorkItems()
			sb.WriteString(fmt.Sprintf("  - %s", lane.Name))
			if lane.Purpose != "" {
				sb.WriteString(fmt.Sprintf(" — %s", lane.Purpose))
			}
			sb.WriteString(fmt.Sprintf(" (%d items, %d active)\n", count, active))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n"), nil
}
