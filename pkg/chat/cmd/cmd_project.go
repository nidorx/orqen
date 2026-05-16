package cmd

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(Command{
		Name:        "project",
		Description: "Show project overview",
		Handler:     projectCommandHandler,
	})
}

func projectCommandHandler(ctx context.Context, req *Request) (string, error) {
	proj := req.Project
	if proj == nil {
		return "**Error:** No project loaded.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Project: %s\n\n", proj.Id))
	sb.WriteString(fmt.Sprintf("- **Directory:** `%s`\n", proj.DirAbs))
	sb.WriteString(fmt.Sprintf("- **Modules:** `%d`\n", len(proj.Modules)))

	// Count lanes and workitems
	totalLanes := 0
	totalItems := 0
	for _, mod := range proj.Modules {
		totalLanes += len(mod.Lanes)
		for item := range mod.WorkItems() {
			_ = item
			totalItems++
		}
	}

	sb.WriteString(fmt.Sprintf("- **Lanes:** `%d`\n", totalLanes))
	sb.WriteString(fmt.Sprintf("- **Workitems:** `%d`\n", totalItems))

	active := proj.ActiveAgentCount()
	sb.WriteString(fmt.Sprintf("- **Active agents:** `%d`\n", active))

	return sb.String(), nil
}
