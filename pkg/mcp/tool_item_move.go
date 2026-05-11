package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ItemMoveInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
	ItemSeq    int     `json:"item_seq" jsonschema:"sequential ID of the work item to move"`
	ToLane     string  `json:"to_lane" jsonschema:"destination lane name"`
}

func (i *ItemMoveInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type ItemMoveOutput struct {
	Success bool   `json:"success"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tnItemMove = "orqen_item_move"

func init() {
	tools[tnItemMove] = &mcp.Tool{
		Description: "Moves a work item directory from one lane to another within a module. Updates internal state to reflect the new lane position.",
	}
}

func ItemMoveHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemMoveInput, proj *engine.Project) (*mcp.CallToolResult, ItemMoveOutput, error) {
	out := ItemMoveOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.ToLane == "" {
		out.Error = "to_lane is required"
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

	// Find the work item
	var item *engine.WorkItem
	if input.ItemSeq > 0 {
		item = targetModule.GetWorkItemBySeq(input.ItemSeq)
	}

	// Fallback: try to find by WorkItemID
	if item == nil && input.WorkItemID != nil && *input.WorkItemID != "" {
		item = targetModule.GetWorkItemById(*input.WorkItemID)
	}

	if item == nil {
		out.Error = fmt.Sprintf("work item not found (id=%d)", input.ItemSeq)
		return nil, out, nil
	}

	fromLane := item.Lane

	if err := item.MoveTo(input.ToLane); err != nil {
		out.Error = fmt.Sprintf("failed to move item: %v", err)
		return nil, out, nil
	}

	out.Success = true
	out.From = fromLane.Dir
	out.To = item.Lane.Dir

	return nil, out, nil
}
