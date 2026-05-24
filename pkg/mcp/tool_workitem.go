package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// status
// Returns the current work item, lane, module, and project context
// for the running agent job. Requires workItemID.

type ItemStatusInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
}

func (i *ItemStatusInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type ItemStatusOutput struct {
	Found bool                  `json:"found"`
	Item  *engine.WorkItemAlias `json:"item,omitempty"`
	Error string                `json:"error,omitempty"`
}

const tnWorkitem = "item"

func init() {
	tools[tnWorkitem] = &mcp.Tool{
		Description: "Returns the current work item, lane, module, and project context for the running agent job. Use this to understand what you are working on.",
	}
}

func WorkitemHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemStatusInput, proj *engine.Project) (*mcp.CallToolResult, ItemStatusOutput, error) {
	out := ItemStatusOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.WorkItemID == nil || *input.WorkItemID == "" {
		out.Error = "no id provided"
		return nil, out, nil
	}

	workItemID := *input.WorkItemID

	item := proj.GetWorkItemById(*input.WorkItemID)

	if item == nil {
		out.Error = fmt.Sprintf("work item with id %q not found", workItemID)
		return nil, out, nil
	}

	out.Found = true
	out.Item = item.Alias()
	return nil, out, nil
}
