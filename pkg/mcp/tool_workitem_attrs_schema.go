package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type SchemaInput struct {
	Module *string `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
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

	targetModule, err := proj.FindModule(input.Module)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}
	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter"
		return nil, out, nil
	}

	out.Module = targetModule.Name
	out.Fields = targetModule.Schema()

	return nil, out, nil
}
