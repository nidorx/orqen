package mcp

import (
	"context"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/utils"
)

type DependenciesInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
}

func (i *DependenciesInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type DependenciesOutput struct {
	Item         *engine.WorkItemAlias   `json:"item"`
	Dependents   []*engine.WorkItemAlias `json:"dependents"`
	Dependencies []*engine.WorkItemAlias `json:"dependencies"`
	Error        string                  `json:"error,omitempty"`
}

const tnWorkitemDependencies = "workitem_dependencies"

func init() {
	tools[tnWorkitemDependencies] = &mcp.Tool{
		Description: "Checks dependency status for the current work item. Resolves them to actual work items with their status.",
	}
}

func WorkitemDependenciesHandler(ctx context.Context, req *mcp.CallToolRequest, input *DependenciesInput, proj *engine.Project) (*mcp.CallToolResult, DependenciesOutput, error) {
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

	out.Item = currentItem.Alias()
	out.Dependents = utils.Map(slices.Collect(currentItem.Dependents()), workItem2Alias)
	out.Dependencies = utils.Map(slices.Collect(currentItem.Dependencies()), workItem2Alias)
	return nil, out, nil
}

func workItem2Alias(i *engine.WorkItem) *engine.WorkItemAlias {
	return i.Alias()
}
