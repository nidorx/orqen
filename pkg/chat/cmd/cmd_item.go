package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nidorx/orqen/pkg/engine"
)

func init() {
	Register(Command{
		Name:        "item",
		Description: "Show workitem details by ID or seq number",
		Handler:     itemCommandHandler,
	})
}

func itemCommandHandler(ctx context.Context, req *Request) (string, error) {
	proj := req.Project
	if proj == nil {
		return "**Error:** No project loaded.", nil
	}

	workItemID := strings.TrimSpace(req.Content)
	if workItemID == "" {
		return "**Usage:** `/item <id>`", nil
	}

	// Try to find by workitem ID (name)
	item := proj.GetWorkItemById(workItemID)
	if item == nil {
		// Try parsing as sequence number
		if seq, err := strconv.Atoi(workItemID); err == nil {
			for _, mod := range proj.Modules {
				item = mod.GetWorkItemBySeq(seq)
				if item != nil {
					break
				}
			}
		}
	}

	if item == nil {
		return fmt.Sprintf("**Error:** Workitem %q not found.", workItemID), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", item.Name))
	sb.WriteString(fmt.Sprintf("- **ID:** `%s`\n", item.ID))
	sb.WriteString(fmt.Sprintf("- **Seq:** `%d`\n", item.Seq))
	sb.WriteString(fmt.Sprintf("- **Lane:** %s\n", item.Lane.Name))
	sb.WriteString(fmt.Sprintf("- **Status:** `%s`\n", itemStatus(item)))

	if len(item.Files) > 0 {
		sb.WriteString("\n### Files\n")
		for _, f := range item.Files {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	// Show attributes if any
	if len(item.Attributes) > 0 {
		sb.WriteString("\n### Attributes\n")
		for k, v := range item.Attributes {
			sb.WriteString(fmt.Sprintf("- **%s:** `%v`\n", k, v))
		}
	}

	return sb.String(), nil
}

func itemStatus(item *engine.WorkItem) string {
	if item.InProgress {
		return "in_progress"
	}
	return "pending"
}
