package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_next_sequence ────────────────────────────────────────────
// Returns the next available sequence number for a module.
// Based on the highest existing sequence number across all lanes.

type NextSequenceInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string `json:"module,omitempty" jsonschema:"module name (e.g., task, adr, learning)"`
}

func (i *NextSequenceInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type NextSequenceOutput struct {
	Next    int    `json:"next"`
	Current int    `json:"current_max"`
	Module  string `json:"module"`
	Error   string `json:"error,omitempty"`
}

const tnNextSequence = "orqen_next_sequence"

func init() {
	tools[tnNextSequence] = &mcp.Tool{
		Description: "Returns the next available sequence number for a module based on the highest existing sequence across all lanes.",
	}
}

func NextSequenceHandler(ctx context.Context, req *mcp.CallToolRequest, input *NextSequenceInput, proj *project.Project) (*mcp.CallToolResult, NextSequenceOutput, error) {
	out := NextSequenceOutput{}

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

	// Get next sequence number
	nextSeq := targetModule.NextSequence()

	out.Module = targetModule.Name
	out.Current = nextSeq - 1
	out.Next = nextSeq

	return nil, out, nil
}
