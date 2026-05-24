package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type SchemaInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
}

func (i *SchemaInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type SchemaOutput struct {
	Module string               `json:"module"`
	Fields []engine.SchemaField `json:"fields"`
	Error  string               `json:"error,omitempty"`
}

const tnWorkitemAttrsSchema = "workitem_attrs_schema"

func init() {
	tools[tnWorkitemAttrsSchema] = &mcp.Tool{
		Description: "Returns all observed workitem attributes and their unique values (domains) across all workitems in a module. Use this to understand what metadata fields exist.",
	}
}

func WorkitemAttrSchemaHandler(ctx context.Context, req *mcp.CallToolRequest, input *SchemaInput, proj *engine.Project) (*mcp.CallToolResult, SchemaOutput, error) {
	out := SchemaOutput{}

	if proj == nil {
		out.Error = "project not available"
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

	out.Module = targetModule.Name
	out.Fields = targetModule.Schema()

	return nil, out, nil
}
