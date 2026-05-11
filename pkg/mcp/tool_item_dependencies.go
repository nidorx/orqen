package mcp

import (
	"context"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type DependenciesInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
}

func (i *DependenciesInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type DependenciesOutput struct {
	Item         *engine.WorkItem   `json:"item"`
	Dependents   []*engine.WorkItem `json:"dependents"`
	Dependencies []*engine.WorkItem `json:"dependencies"`
	Error        string             `json:"error,omitempty"`
}

const tnItemDependencies = "orqen_item_dependencies"

func init() {
	tools[tnItemDependencies] = &mcp.Tool{
		Description: "Checks dependency status for the current work item. Resolves them to actual work items with their status.",
	}
}

func ItemDependenciesHandler(ctx context.Context, req *mcp.CallToolRequest, input *DependenciesInput, proj *engine.Project) (*mcp.CallToolResult, DependenciesOutput, error) {
	out := DependenciesOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.WorkItemID == nil || *input.WorkItemID == "" {
		out.Error = "no Work Item ID provided"
		return nil, out, nil
	}

	workItemID := *input.WorkItemID

	// Find the current work item by WorkItemID
	var currentItem *engine.WorkItem

	for _, mod := range proj.Modules {
		if item := mod.GetWorkItemById(workItemID); item != nil {
			currentItem = item
			break
		}
	}

	if currentItem == nil {
		out.Error = fmt.Sprintf("work item with ID %q not found", workItemID)
		return nil, out, nil
	}

	out.Item = currentItem
	out.Dependents = slices.Collect(currentItem.Dependents())
	out.Dependencies = slices.Collect(currentItem.Dependencies())

	return nil, out, nil
}
