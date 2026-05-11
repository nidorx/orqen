package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/nidorx/orqen/pkg/engine"
)

const listLimit = 20

func init() {
	RegisterCommand(CommandDef{
		Name:        "list",
		Description: "List workitems in a lane (or all lanes)",
		Handler:     handleList,
	})
}

func handleList(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	proj := bot.Project
	if proj == nil {
		return "No project loaded.", nil
	}

	var sb strings.Builder
	totalCount := 0
	truncated := false

	// Parse lane filter from args
	laneFilter := strings.TrimSpace(args)
	if laneFilter == "" {
		// List all lanes
		for _, mod := range proj.Modules {
			for _, lane := range mod.Lanes {
				items, count, truncated := formatLaneItems(mod.Name, lane, listLimit-totalCount)
				sb.WriteString(items)
				totalCount += count
				if truncated {
					truncated = true
					break
				}
				if totalCount >= listLimit {
					truncated = true
					break
				}
			}
			if truncated {
				break
			}
		}
		if sb.Len() == 0 {
			sb.WriteString("No workitems found.\n")
		}
	} else {
		// Filter to specific lane
		found := false
		for _, mod := range proj.Modules {
			for _, lane := range mod.Lanes {
				if lane.Name == laneFilter || lane.Dir == laneFilter {
					found = true
					items, count, _ := formatLaneItems(mod.Name, lane, listLimit)
					sb.WriteString(items)
					totalCount = count
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return fmt.Sprintf("Lane %q not found.", laneFilter), nil
		}
	}

	if truncated {
		sb.WriteString(fmt.Sprintf("\n... (showing first %d of many items)\n", listLimit))
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d items", totalCount))

	return sb.String(), nil
}

func formatLaneItems(moduleName string, lane *engine.Lane, limit int) (string, int, bool) {
	var sb strings.Builder
	count := 0
	truncated := false

	sb.WriteString(fmt.Sprintf("Module: %s | Lane: %s\n", moduleName, lane.Name))

	for item := range lane.WorkItems() {
		if count >= limit {
			truncated = true
			break
		}
		status := "pending"
		if item.InProgress {
			status = "in_progress"
		}
		sb.WriteString(fmt.Sprintf("  - %s (%s)\n", item.Name, status))
		count++
	}

	if count == 0 {
		sb.WriteString("  (empty)\n")
	}

	return sb.String(), count, truncated
}
