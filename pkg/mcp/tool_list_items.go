package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_list_items ───────────────────────────────────────────────
// Lists work items in a specific lane (same module or cross-module).

type ListItemsInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
	Lane       string  `json:"lane" jsonschema:"lane name within the module"`
}

func (i *ListItemsInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type ListItemSummary struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
}

type ListItemsOutput struct {
	Items []ListItemSummary `json:"items"`
	Error string            `json:"error,omitempty"`
}

const tnListItems = "orqen_list_items"

func init() {
	tools[tnListItems] = &mcp.Tool{
		Description: "Lists work items in a specific lane within a module. Use this to see what tasks exist in a given lane.",
	}
}

func ListItemsHandler(ctx context.Context, req *mcp.CallToolRequest, input *ListItemsInput, proj *project.Project) (*mcp.CallToolResult, ListItemsOutput, error) {
	out := ListItemsOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Lane == "" {
		out.Error = "lane name is required"
		return nil, out, nil
	}

	var targetModule *project.Module
	if input.Module != nil && *input.Module != "" {
		targetModule = proj.GetModule(*input.Module)
		if targetModule == nil {
			out.Error = fmt.Sprintf("module not found: %s", *input.Module)
			return nil, out, nil
		}
	} else {
		// Try to resolve current module from WorkItemID
		if input.WorkItemID != nil && *input.WorkItemID != "" {
			targetModule = findModuleByWorkItemID(proj, *input.WorkItemID)
		}
		if targetModule == nil && len(proj.Modules) == 1 {
			targetModule = proj.Modules[0]
		}
	}

	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter or ensure workitem_id is set"
		return nil, out, nil
	}

	lane := targetModule.GetLane(input.Lane)
	if lane == nil {
		out.Error = "lane not found: " + input.Lane
		return nil, out, nil
	}

	for item := range lane.WorkItems() {
		dirPath := filepath.Join(lane.DirAbs, item.Name)
		rel, err := filepath.Rel(proj.DirAbs, filepath.Clean(dirPath))
		dirStr := filepath.ToSlash(rel)
		if err != nil {
			dirStr = dirPath
		}
		out.Items = append(out.Items, ListItemSummary{
			ID:    item.Seq,
			Name:  item.Name,
			Dir:   dirStr,
			Files: item.Files,
		})
	}

	return nil, out, nil
}
