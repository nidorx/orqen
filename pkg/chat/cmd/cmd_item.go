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
		Description: "Show workitem details by ID or sequential number",
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
	var item *engine.WorkItem
	// Try parsing as sequence number
	if seq, err := strconv.Atoi(workItemID); err == nil {
		for _, mod := range proj.Modules {
			item = mod.GetWorkItemBySeq(seq)
			if item != nil {
				break
			}
		}
	}

	if item == nil {
		return fmt.Sprintf("**Error:** Workitem %q not found.", workItemID), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s\n\n", item.Name)
	fmt.Fprintf(&sb, "- **Seq:** `%d`\n", item.Seq)
	fmt.Fprintf(&sb, "- **Lane:** %s\n", item.Lane.Name)
	fmt.Fprintf(&sb, "- **Status:** `%s`\n", itemStatus(item))

	if len(item.Files) > 0 {
		sb.WriteString("\n### Files\n")
		for _, f := range item.Files {
			fmt.Fprintf(&sb, "- `%s`\n", f)
		}
	}

	// Show attributes if any
	if len(item.Attributes) > 0 {
		sb.WriteString("\n### Attributes\n")
		for k, v := range item.Attributes {
			fmt.Fprintf(&sb, "- **%s:** `%v`\n", k, v)
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
