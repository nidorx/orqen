package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ItemCreateInput struct {
	Module     *string `json:"module" jsonschema:"module name (e.g., task, adr, learning)"`
	Lane       string  `json:"lane" jsonschema:"destination lane name"`
	SimpleName string  `json:"simple_name" jsonschema:"kebab-case descriptive name for the item"`
}

type ItemCreateOutput struct {
	Success  bool                  `json:"success"`
	WorkItem *engine.WorkItemAlias `json:"workitem"`
	Error    string                `json:"error,omitempty"`
}

const tnWorkitemCreate = "workitem_create"

func init() {
	tools[tnWorkitemCreate] = &mcp.Tool{
		Description: "Creates a new work item in a specific lane of a module. Creates the directory following naming conventions (MOD_TYPE-NNNN-name) and an empty .yaml file.",
	}
}

func WorkitemCreateHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemCreateInput, proj *engine.Project) (*mcp.CallToolResult, ItemCreateOutput, error) {
	out := ItemCreateOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Lane == "" {
		out.Error = "lane name is required"
		return nil, out, nil
	}

	if input.SimpleName == "" {
		out.Error = "simple_name is required"
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

	lane := targetModule.GetLane(input.Lane)
	if lane == nil {
		out.Error = fmt.Sprintf(
			"lane %q not found in module %s (available: %s)",
			input.Lane, *input.Module, laneNames(targetModule),
		)
		return nil, out, nil
	}

	wi, err := lane.CreateWorkItem(input.SimpleName)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}

	out.Success = true
	out.WorkItem = wi.Alias()

	return nil, out, nil
}

func laneNames(mod *engine.Module) string {
	var names []string
	for _, l := range mod.Lanes {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}
