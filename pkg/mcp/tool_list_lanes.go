package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// ── orqen_list_lanes ───────────────────────────────────────────────
// Lists all lanes in a module with their configuration and item counts.

type ListLanesInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
	Module     *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
}

func (i *ListLanesInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type LaneDetail struct {
	Name              string   `json:"name"`
	Dir               string   `json:"dir"`
	Purpose           string   `json:"purpose"`
	MaxAgents         int      `json:"max_agents"`
	Artifacts         []string `json:"artifacts,omitempty"`
	UserAction        string   `json:"user_action,omitempty"`
	IgnoreIfExists    []string `json:"ignore_if_exists,omitempty"`
	IgnoreIfNotExists []string `json:"ignore_if_not_exists,omitempty"`
	ItemCount         int      `json:"item_count"`
	ActiveCount       int      `json:"active_count"`
	HasAvailableSlot  bool     `json:"has_available_slot"`
}

type ListLanesOutput struct {
	Module string       `json:"module"`
	Lanes  []LaneDetail `json:"lanes"`
	Error  string       `json:"error,omitempty"`
}

const tnListLanes = "orqen_list_lanes"

func init() {
	tools[tnListLanes] = &mcp.Tool{
		Description: "Lists all lanes in a module with their configuration, purpose, item counts, and availability. Use this to understand lane structure before creating or moving items.",
	}
}

func ListLanesHandler(ctx context.Context, req *mcp.CallToolRequest, input *ListLanesInput, proj *engine.Project) (*mcp.CallToolResult, ListLanesOutput, error) {
	out := ListLanesOutput{}

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

	for _, lane := range targetModule.Lanes {
		out.Lanes = append(out.Lanes, LaneDetail{
			Dir:               lane.Dir,
			Name:              lane.Name,
			Purpose:           lane.Purpose,
			MaxAgents:         lane.MaxAgents,
			Artifacts:         lane.Artifacts,
			UserAction:        lane.UserAction,
			IgnoreIfExists:    lane.IgnoreIfExists,
			IgnoreIfNotExists: lane.IgnoreIfNotExists,
			ItemCount:         lane.CountWorkItems(),
			ActiveCount:       lane.CountActiveWorkItems(),
			HasAvailableSlot:  lane.HasAvailableSlot(),
		})
	}

	return nil, out, nil
}
