package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_project_info ─────────────────────────────────────────────
// Returns the full project structure: modules, lanes, prompts, agent
// behaviors, critical rules, and configuration.

type ProjectInfoInput struct {
	JobId   *string `json:"job_id,omitempty" jsonschema:"job id (auto-injected)"`
	Verbose bool    `json:"verbose,omitempty" jsonschema:"include full prompts and agent behavior details"`
}

func (i *ProjectInfoInput) SetJobId(jobId string) {
	i.JobId = &jobId
}

type LaneSummary struct {
	Name        string   `json:"name"`
	Dir         string   `json:"dir"`
	Purpose     string   `json:"purpose"`
	MaxAgents   int      `json:"max_agents"`
	Artifacts   []string `json:"artifacts,omitempty"`
	UserAction  string   `json:"user_action,omitempty"`
	AgentCount  int      `json:"agent_count"`
	ActiveCount int      `json:"active_count"`
}

type ModuleSummary struct {
	Name  string        `json:"name"`
	Dir   string        `json:"dir"`
	Lanes []LaneSummary `json:"lanes"`
}

type ProjectInfoOutput struct {
	Dir           string          `json:"dir"`
	Modules       []ModuleSummary `json:"modules"`
	DefaultAgent  string          `json:"default_agent"`
	MaxAgents     int             `json:"max_agents"`
	SleepInterval int             `json:"sleep_interval_seconds"`
}

const tnProjectInfo = "orqen_project_info"

func init() {
	tools[tnProjectInfo] = &mcp.Tool{
		Description: "Returns the full project structure: modules, lanes, item counts, and configuration. Use this to understand the overall project layout.",
	}
}

func ProjectInfoHandler(ctx context.Context, req *mcp.CallToolRequest, input *ProjectInfoInput, proj *project.Project) (*mcp.CallToolResult, ProjectInfoOutput, error) {
	if proj == nil {
		return nil, ProjectInfoOutput{}, nil
	}

	out := ProjectInfoOutput{
		Dir:           proj.DirAbs,
		DefaultAgent:  proj.Agents.Default,
		MaxAgents:     proj.Execution.MaxAgents,
		SleepInterval: proj.Execution.SleepIntervalSeconds,
	}

	for _, mod := range proj.Modules {
		modSummary := ModuleSummary{
			Name: mod.Name,
			Dir:  mod.DirAbs,
		}

		for _, lane := range mod.Lanes {
			laneSummary := LaneSummary{
				Name:        lane.Name,
				Dir:         lane.Dir,
				Purpose:     lane.Purpose,
				MaxAgents:   lane.MaxAgents,
				Artifacts:   lane.Artifacts,
				UserAction:  lane.UserAction,
				AgentCount:  len(lane.ListItems()),
				ActiveCount: lane.ActiveItemCount(),
			}
			modSummary.Lanes = append(modSummary.Lanes, laneSummary)
		}

		out.Modules = append(out.Modules, modSummary)
	}

	return nil, out, nil
}
