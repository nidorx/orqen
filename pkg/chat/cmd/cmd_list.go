package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/nidorx/orqen/pkg/engine"
)

const listLimit = 20

func init() {
	Register(Command{
		Name:        "list",
		Description: "List workitems in a lane (or all lanes)",
		Handler:     listCommandHandler,
	})
}

func listCommandHandler(ctx context.Context, req *Request) (string, error) {
	proj := req.Project
	if proj == nil {
		return "**Error:** No project loaded.", nil
	}

	var sb strings.Builder
	totalCount := 0
	truncated := false

	// Parse lane filter from args
	laneFilter := strings.TrimSpace(req.Content)
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
			return fmt.Sprintf("**Error:** Lane %q not found.", laneFilter), nil
		}
	}

	if truncated {
		sb.WriteString(fmt.Sprintf("\n*... (showing first %d of many items)*\n", listLimit))
	}

	sb.WriteString(fmt.Sprintf("\n**Total:** %d items", totalCount))

	return sb.String(), nil
}

func formatLaneItems(moduleName string, lane *engine.Lane, limit int) (string, int, bool) {
	var sb strings.Builder
	count := 0
	truncated := false

	fmt.Fprintf(&sb, "### Module: %s | Lane: %s\n", moduleName, lane.Name)

	for item := range lane.WorkItems() {
		if count >= limit {
			truncated = true
			break
		}
		status := "pending"
		if item.InProgress {
			status = "in_progress"
		}
		fmt.Fprintf(&sb, "- `%s` (%s)\n", item.Name, status)
		count++
	}

	if count == 0 {
		sb.WriteString("*empty*\n")
	}

	return sb.String(), count, truncated
}
