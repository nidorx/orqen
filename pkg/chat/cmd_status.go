package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/nidorx/orqen/pkg/engine"
)

func init() {
	RegisterCommand(CommandDef{
		Name:        "status",
		Description: "Show project execution status",
		Handler:     handleStatus,
	})
}

func handleStatus(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	proj := bot.Project
	if proj == nil {
		return "No project loaded.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\n", proj.Id))

	// Determine overall status
	activeCount := proj.ActiveAgentCount()
	if activeCount > 0 {
		sb.WriteString(fmt.Sprintf("Status: running (%d active agents)\n", activeCount))
	} else {
		sb.WriteString("Status: idle\n")
	}

	sb.WriteString("\nActive Lanes:\n")

	hasActive := false
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			active := lane.CountActiveWorkItems()
			if active > 0 {
				hasActive = true
				for item := range lane.WorkItems() {
					if item.InProgress {
						sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", lane.Name, item.Name, "in_progress"))
					}
				}
			}
		}
	}

	if !hasActive {
		sb.WriteString("  (none)\n")
	}

	return sb.String(), nil
}

// botProjectKey is a context key for injecting the project into the bot.
type botProjectKey struct{}

// WithProject returns a copy of the bot with the project set.
func (b *TelegramBot) WithProject(proj *engine.Project) *TelegramBot {
	clone := *b
	clone.Project = proj
	return &clone
}
