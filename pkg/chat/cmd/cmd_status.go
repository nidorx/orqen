package cmd

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(Command{
		Name:        "status",
		Description: "Show project execution status",
		Handler:     statusCommandHandler,
	})
}

func statusCommandHandler(ctx context.Context, req *Request) (string, error) {
	proj := req.Project
	if proj == nil {
		return "**Error:** No project loaded.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Project: %s\n\n", proj.Id))

	// Determine overall status
	activeCount := proj.ActiveAgentCount()
	if activeCount > 0 {
		sb.WriteString(fmt.Sprintf("**Status:** `running` (%d active agents)\n", activeCount))
	} else {
		sb.WriteString("**Status:** `idle`\n")
	}

	sb.WriteString("\n### Active Lanes\n")

	hasActive := false
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			active := lane.CountActiveWorkItems()
			if active > 0 {
				hasActive = true
				for item := range lane.WorkItems() {
					if item.InProgress {
						sb.WriteString(fmt.Sprintf("- **%s:** `%s` (%s)\n", lane.Name, item.Name, "in_progress"))
					}
				}
			}
		}
	}

	if !hasActive {
		sb.WriteString("*none*\n")
	}

	return sb.String(), nil
}
