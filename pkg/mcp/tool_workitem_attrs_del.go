package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// Removes specified attribute keys from a work item and persists
// the changes to disk. The "dependencies" key cannot be removed.

type ItemAttrsDelInput struct {
	Module      *string  `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
	WorkItemSeq int      `json:"workitem_seq" jsonschema:"work item sequence number"`
	Keys        []string `json:"keys" jsonschema:"attribute keys to remove from the work item"`
}

type ItemAttrsDelOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const tnWorkitemAttrsDel = "workitem_attrs_del"

func init() {
	tools[tnWorkitemAttrsDel] = &mcp.Tool{
		Description: "Removes specified attribute keys from a work item and persists the changes to disk. The \"dependencies\" key cannot be removed.",
	}
}

func WorkitemAttrsDelHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemAttrsDelInput, proj *engine.Project) (*mcp.CallToolResult, ItemAttrsDelOutput, error) {
	out := ItemAttrsDelOutput{Success: false}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.WorkItemSeq <= 0 {
		out.Error = "workitem_seq is required and must be greater than 0"
		return nil, out, nil
	}

	if len(input.Keys) == 0 {
		out.Error = "keys must not be empty"
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

	item := targetModule.GetWorkItemBySeq(input.WorkItemSeq)
	if item == nil {
		out.Error = "work item not found with seq: " + string(rune(input.WorkItemSeq))
		return nil, out, nil
	}

	if err := item.AttributesDel(input.Keys); err != nil {
		out.Error = "failed to delete attributes: " + err.Error()
		return nil, out, nil
	}

	out.Success = true
	return nil, out, nil
}
