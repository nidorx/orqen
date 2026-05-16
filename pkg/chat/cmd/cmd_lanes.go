package cmd

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(Command{
		Name:        "lanes",
		Description: "List available lanes",
		Handler:     lanesCommandHandler,
	})
}

func lanesCommandHandler(ctx context.Context, req *Request) (string, error) {
	proj := req.Project
	if proj == nil {
		return "**Error:** No project loaded.", nil
	}

	var sb strings.Builder
	sb.WriteString("# Available Lanes\n\n")

	for _, mod := range proj.Modules {
		fmt.Fprintf(&sb, "## Module: **%s**\n", mod.Name)
		for _, lane := range mod.GetLanesInOrder() {
			count := lane.CountWorkItems()
			active := lane.CountActiveWorkItems()
			fmt.Fprintf(&sb, "**%s**", lane.Name)
			if lane.Purpose != "" {
				fmt.Fprintf(&sb, " - %s", lane.Purpose)
			}
			fmt.Fprintf(&sb, "\n - *(%d items, %d active)*\n\n", count, active)
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n"), nil
}
