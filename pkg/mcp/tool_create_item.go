package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type CreateItemInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"job id (auto-injected)"`
	Module     *string `json:"module" jsonschema:"module name (e.g., task, adr, learning)"`
	Lane       string  `json:"lane" jsonschema:"destination lane name"`
	SimpleName string  `json:"simple_name" jsonschema:"kebab-case descriptive name for the item"`
}

func (i *CreateItemInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type CreateItemOutput struct {
	Success  bool             `json:"success"`
	WorkItem *engine.WorkItem `json:"workitem"`
	Error    string           `json:"error,omitempty"`
}

const tnCreateItem = "orqen_create_item"

func init() {
	tools[tnCreateItem] = &mcp.Tool{
		Description: "Creates a new work item in a specific lane of a module. Creates the directory following naming conventions (MOD_TYPE-NNNN-name) and an empty .yaml file.",
	}
}

func CreateItemHandler(ctx context.Context, req *mcp.CallToolRequest, input *CreateItemInput, proj *engine.Project) (*mcp.CallToolResult, CreateItemOutput, error) {
	out := CreateItemOutput{}

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

	targetModule, err := findTargetModuleBy(proj, input.Module, input.WorkItemID)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}
	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter or ensure workitem_id is set"
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
	out.WorkItem = wi

	return nil, out, nil
}

func laneNames(mod *engine.Module) string {
	var names []string
	for _, l := range mod.Lanes {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}
