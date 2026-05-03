package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
	"github.com/nidorx/orqen/pkg/storage"
)

// ── orqen_schema ───────────────────────────────────────────────────
// Returns all observed front matter attributes and their unique values
// (domains) across all markdown files in a module.

type SchemaInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
}

func (i *SchemaInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type SchemaFieldInfo struct {
	Field  string   `json:"field"`
	Types  []string `json:"types"`
	Values []any    `json:"values"`
}

type SchemaOutput struct {
	Module string            `json:"module"`
	Dir    string            `json:"dir"`
	Fields []SchemaFieldInfo `json:"fields"`
	Error  string            `json:"error,omitempty"`
}

const tnSchema = "orqen_schema"

func init() {
	tools[tnSchema] = &mcp.Tool{
		Description: "Returns all observed front matter attributes and their unique values (domains) across all markdown files in a module. Use this to understand what metadata fields exist.",
	}
}

func SchemaHandler(ctx context.Context, req *mcp.CallToolRequest, input *SchemaInput, proj *project.Project) (*mcp.CallToolResult, SchemaOutput, error) {
	out := SchemaOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	var targetModule *project.Module
	if input.Module != nil && *input.Module != "" {
		targetModule = proj.GetModule(*input.Module)
		if targetModule == nil {
			out.Error = fmt.Sprintf("module not found: %s", *input.Module)
			return nil, out, nil
		}
	} else {
		// Try to resolve current module from WorkItemID
		if input.WorkItemID != nil && *input.WorkItemID != "" {
			targetModule = findModuleByWorkItemID(proj, *input.WorkItemID)
		}
		if targetModule == nil && len(proj.Modules) == 1 {
			targetModule = proj.Modules[0]
		}
	}

	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter or ensure workitem_id is set"
		return nil, out, nil
	}

	out.Module = targetModule.Name
	out.Dir = targetModule.DirAbs

	fields, err := storage.Schema(targetModule.DirAbs)
	if err != nil {
		out.Error = fmt.Sprintf("schema error: %v", err)
		return nil, out, nil
	}

	for _, f := range fields {
		out.Fields = append(out.Fields, SchemaFieldInfo{
			Field:  f.Field,
			Types:  f.Types,
			Values: f.Values,
		})
	}

	return nil, out, nil
}
