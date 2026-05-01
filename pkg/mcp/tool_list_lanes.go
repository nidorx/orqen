package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_list_lanes ───────────────────────────────────────────────
// Lists all lanes in a module with their configuration and item counts.

type ListLanesInput struct {
	JobId  *string `json:"job_id,omitempty" jsonschema:"job id (auto-injected)"`
	Module *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
}

func (i *ListLanesInput) SetJobId(jobId string) {
	i.JobId = &jobId
}

type LaneDetail struct {
	Name             string   `json:"name"`
	Dir              string   `json:"dir"`
	Purpose          string   `json:"purpose"`
	MaxAgents        int      `json:"max_agents"`
	Artifacts        []string `json:"artifacts,omitempty"`
	UserAction       string   `json:"user_action,omitempty"`
	IgnoreIfExists   []string `json:"ignore_if_exists,omitempty"`
	ItemCount        int      `json:"item_count"`
	ActiveCount      int      `json:"active_count"`
	HasAvailableSlot bool     `json:"has_available_slot"`
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

func ListLanesHandler(ctx context.Context, req *mcp.CallToolRequest, input *ListLanesInput, proj *project.Project) (*mcp.CallToolResult, ListLanesOutput, error) {
	out := ListLanesOutput{}

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
		// Try to resolve current module from JobId
		if input.JobId != nil && *input.JobId != "" {
			targetModule = findModuleByJobID(proj, *input.JobId)
		}
		if targetModule == nil && len(proj.Modules) == 1 {
			targetModule = proj.Modules[0]
		}
	}

	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter or ensure job_id is set"
		return nil, out, nil
	}

	out.Module = targetModule.Name

	for _, lane := range targetModule.Lanes {
		items := lane.ListItems()
		activeCount := lane.ActiveItemCount()

		out.Lanes = append(out.Lanes, LaneDetail{
			Name:             lane.Name,
			Dir:              lane.Dir,
			Purpose:          lane.Purpose,
			MaxAgents:        lane.MaxAgents,
			Artifacts:        lane.Artifacts,
			UserAction:       lane.UserAction,
			IgnoreIfExists:   lane.IgnoreIfExists,
			ItemCount:        len(items),
			ActiveCount:      activeCount,
			HasAvailableSlot: lane.HasAvailableSlot(),
		})
	}

	return nil, out, nil
}
