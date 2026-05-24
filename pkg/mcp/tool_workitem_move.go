package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ItemMoveInput struct {
	Module      *string `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
	WorkitemSeq int     `json:"workitem_seq" jsonschema:"sequential ID of the work item to move"`
	ToLane      string  `json:"to_lane" jsonschema:"destination lane name"`
}

type ItemMoveOutput struct {
	Success bool   `json:"success"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tnWorkitemMove = "workitem_move"

func init() {
	tools[tnWorkitemMove] = &mcp.Tool{
		Description: "Moves a work item directory from one lane to another within a module. Updates internal state to reflect the new lane position.",
	}
}

func WorkitemMoveHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemMoveInput, proj *engine.Project) (*mcp.CallToolResult, ItemMoveOutput, error) {
	out := ItemMoveOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.ToLane == "" {
		out.Error = "to_lane is required"
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
