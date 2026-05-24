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
	Module      *string `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
	WorkitemSeq int     `json:"workitem_seq" jsonschema:"sequential ID of the work item to move"`
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

	targetModule, err := proj.FindModule(input.Module)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}
	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter"
		return nil, out, nil
	}

	// Find the work item
	var item *engine.WorkItem
	if input.WorkitemSeq > 0 {
		item = targetModule.GetWorkItemBySeq(input.WorkitemSeq)
	}

	if item == nil {
		out.Error = fmt.Sprintf("work item not found (workitem_seq=%d)", input.WorkitemSeq)
		return nil, out, nil
	}

	out.Item = item.Alias()
	out.Dependents = utils.Map(slices.Collect(item.Dependents()), workItem2Alias)
	out.Dependencies = utils.Map(slices.Collect(item.Dependencies()), workItem2Alias)
	return nil, out, nil
}

func workItem2Alias(i *engine.WorkItem) *engine.WorkItemAlias {
	return i.Alias()
}
