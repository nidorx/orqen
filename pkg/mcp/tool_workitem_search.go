package mcp

import (
	"context"
	"iter"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/utils"
)

// Searches for work items in a module or lane, optionally filtered by
// a condition DSL string. Returns full WorkItem objects.

type ItemSearchInput struct {
	Module    *string `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
	Lane      string  `json:"lane,omitempty" jsonschema:"optional lane name to filter within the module"`
	Condition string  `json:"condition,omitempty" jsonschema:"optional condition SQL-like DSL string for filtering (e.g., \"priority > 3\")"`
}

type ItemSearchOutput struct {
	Items []*engine.WorkItemAlias `json:"items"`
	Error string                  `json:"error,omitempty"`
}

const tnWorkitemSearch = "workitem_search"

func init() {
	tools[tnWorkitemSearch] = &mcp.Tool{
		Description: "Searches for work items in a module or lane, optionally filtered by a condition SQL-like DSL string. Returns full WorkItem objects.",
	}
}

func WorkitemSearchHandler(ctx context.Context, req *mcp.CallToolRequest, input *ItemSearchInput, proj *engine.Project) (*mcp.CallToolResult, ItemSearchOutput, error) {
	out := ItemSearchOutput{}

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

	var iterator iter.Seq[*engine.WorkItem]

	if input.Lane != "" {
		// Search within a specific lane
		lane := targetModule.GetLane(input.Lane)
		if lane == nil {
			out.Error = "lane not found: " + input.Lane
			return nil, out, nil
		}

		if input.Condition != "" {
			// Use FilterWorkItems with condition
			it, err := lane.FilterWorkItems(input.Condition)
			if err != nil {
				out.Error = "failed to parse condition: " + err.Error()
				return nil, out, nil
			}
			iterator = it
		} else {
			// No condition: use WorkItems()
			iterator = lane.WorkItems()
		}
	} else {
		// Search across all lanes in the module
		if input.Condition != "" {
			// Use FilterWorkItems with condition
			it, err := targetModule.FilterWorkItems(input.Condition)
			if err != nil {
				out.Error = "failed to parse condition: " + err.Error()
				return nil, out, nil
			}
			iterator = it
		} else {
			// No condition: use WorkItems()
			iterator = targetModule.WorkItems()
		}
	}

	out.Items = utils.Map(slices.Collect(iterator), workItem2Alias)

	return nil, out, nil
}
