package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// orqen_item_attrs_set
// Updates attributes on a work item. Merges the provided attributes
// into the work item's existing attributes and persists them to disk.

type ItemAttrsSetInput struct {
	WorkItemID *string           `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string           `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
	Seq        int               `json:"seq" jsonschema:"work item sequence number"`
	Attributes engine.Attributes `json:"attributes" jsonschema:"key-value pairs of attributes to set or update"`
}

func (i *ItemAttrsSetInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type ItemAttrsSetOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const tnItemAttrsSet = "orqen_item_attrs_set"

func init() {
	tools[tnItemAttrsSet] = &mcp.Tool{
		Description: "Updates attributes on a work item. Merges the provided attributes into the work item's existing attributes and persists them to disk.",
	}
}

func ItemAttrsSetHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemAttrsSetInput, proj *engine.Project) (*mcp.CallToolResult, ItemAttrsSetOutput, error) {
	out := ItemAttrsSetOutput{Success: false}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Seq <= 0 {
		out.Error = "seq is required and must be greater than 0"
		return nil, out, nil
	}

	if len(input.Attributes) == 0 {
		out.Error = "attributes must not be empty"
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

	item := targetModule.GetWorkItemBySeq(input.Seq)
	if item == nil {
		out.Error = "work item not found with seq: " + string(rune(input.Seq))
		return nil, out, nil
	}

	if err := item.AttributesSave(input.Attributes); err != nil {
		out.Error = "failed to save attributes: " + err.Error()
		return nil, out, nil
	}

	out.Success = true
	return nil, out, nil
}
