package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_status ───────────────────────────────────────────────────
// Returns the current work item, lane, module, and project context
// for the running agent job. Requires jobId.

type StatusInput struct {
	JobId *string `json:"job_id,omitempty" jsonschema:"job id (auto-injected)"`
}

func (i *StatusInput) SetJobId(jobId string) {
	i.JobId = &jobId
}

type StatusLaneInfo struct {
	Name      string   `json:"name"`
	Dir       string   `json:"dir"`
	Purpose   string   `json:"purpose"`
	Module    string   `json:"module"`
	Artifacts []string `json:"artifacts,omitempty"`
}

type StatusOutput struct {
	Found       bool           `json:"found"`
	ItemID      int            `json:"item_id,omitempty"`
	ItemName    string         `json:"item_name,omitempty"`
	ItemFiles   []string       `json:"item_files,omitempty"`
	CurrentLane StatusLaneInfo `json:"current_lane,omitempty"`
	ProjectDir  string         `json:"project_dir,omitempty"`
	Error       string         `json:"error,omitempty"`
}

const tnStatus = "orqen_status"

func init() {
	tools[tnStatus] = &mcp.Tool{
		Description: "Returns the current work item, lane, module, and project context for the running agent job. Use this to understand what you are working on.",
	}
}

func StatusHandler(ctx context.Context, req *mcp.CallToolRequest, input *StatusInput, proj *project.Project) (*mcp.CallToolResult, StatusOutput, error) {
	out := StatusOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.JobId == nil || *input.JobId == "" {
		out.Error = "no job id provided"
		return nil, out, nil
	}

	jobID := *input.JobId

	// Scan all modules and lanes to find the work item with this JobID
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			for _, item := range lane.ListItems() {
				if item.JobID == jobID {
					out.Found = true
					out.ItemID = item.ID
					out.ItemName = item.Name
					out.ItemFiles = item.Files
					out.ProjectDir = proj.DirAbs
					out.CurrentLane = StatusLaneInfo{
						Name:      lane.Name,
						Dir:       lane.Dir,
						Purpose:   lane.Purpose,
						Module:    mod.Name,
						Artifacts: lane.Artifacts,
					}
					return nil, out, nil
				}
			}
		}
	}

	out.Error = fmt.Sprintf("work item with job id %q not found", jobID)
	return nil, out, nil
}
