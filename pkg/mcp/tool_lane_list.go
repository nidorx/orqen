package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type LaneListInput struct {
	Module *string `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
}

type LaneDetail struct {
	Name      string   `json:"name"`
	Purpose   string   `json:"purpose"`
	MaxAgents int      `json:"max_agents"`
	Artifacts []string `json:"artifacts,omitempty"`
}

type LaneListOutput struct {
	Module string       `json:"module"`
	Lanes  []LaneDetail `json:"lanes"`
	Error  string       `json:"error,omitempty"`
}

const tnLaneList = "lane_list"

func init() {
	tools[tnLaneList] = &mcp.Tool{
		Description: "Lists all lanes in a module with their configuration, purpose, item counts, and availability. Use this to understand lane structure before creating or moving items.",
	}
}

func LaneListHandler(ctx context.Context, req *mcp.CallToolRequest, input *LaneListInput, proj *engine.Project) (*mcp.CallToolResult, LaneListOutput, error) {
	out := LaneListOutput{}

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

	for _, lane := range targetModule.Lanes {
		out.Lanes = append(out.Lanes, LaneDetail{
			Name:      lane.Name,
			Purpose:   lane.Purpose,
			MaxAgents: lane.MaxAgents,
			Artifacts: lane.Artifacts,
		})
	}

	return nil, out, nil
}
