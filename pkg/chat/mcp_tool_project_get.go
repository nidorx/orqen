package chat

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatProjectGetInput struct{}

type ChatProjectGetOutput struct {
	DirAbs      string            `json:"directory"`
	ModuleCount int               `json:"module_count"`
	Modules     []ModuleSummary   `json:"modules"`
	AgentCount  int               `json:"agent_count"`
	LaneStats   []LaneStatSummary `json:"lane_stats"`
	Error       string            `json:"error,omitempty"`
}

const tnChatProjectGet = "chat_project_get"

func init() {
	chatTools[tnChatProjectGet] = &mcp.Tool{
		Description: "Get an overview of the project structure including modules, lanes, and workitem counts.",
	}
}

func ChatProjectGetHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatProjectGetInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatProjectGetOutput, error) {
	out := ChatProjectGetOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	out.DirAbs = proj.DirAbs
	out.ModuleCount = len(proj.Modules)

	// ActiveAgentCount may panic without cache init
	agentCount := 0
	func() {
		defer func() { recover() }()
		agentCount = proj.ActiveAgentCount()
	}()
	out.AgentCount = agentCount

	for _, mod := range proj.Modules {
		itemCount := 0
		func() {
			defer func() { recover() }()
			for range mod.WorkItems() {
				itemCount++
			}
		}()
		out.Modules = append(out.Modules, ModuleSummary{
			Name:      mod.Name,
			ItemCount: itemCount,
			LaneCount: len(mod.Lanes),
		})

		for _, lane := range mod.Lanes {
			laneCount := 0
			func() {
				defer func() { recover() }()
				laneCount = lane.CountWorkItems()
			}()
			out.LaneStats = append(out.LaneStats, LaneStatSummary{
				Module: mod.Name,
				Name:   lane.Name,
				Count:  laneCount,
			})
		}
	}

	return nil, out, nil
}
