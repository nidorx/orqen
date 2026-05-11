package chat

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatLaneListInput struct {
	Module string `json:"module,omitempty" jsonschema:"Filter by module name"`
}

type ChatLaneListOutput struct {
	Lanes []ChatLaneDetail `json:"lanes"`
	Error string           `json:"error,omitempty"`
}

const tnChatLaneList = "chat_lane_list"

func init() {
	chatTools[tnChatLaneList] = &mcp.Tool{
		Description: "List all lanes in the project with their purpose and item counts.",
	}
}

func ChatLaneListHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatLaneListInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatLaneListOutput, error) {
	out := ChatLaneListOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	for _, mod := range proj.Modules {
		if input.Module != "" && mod.Name != input.Module {
			continue
		}
		for _, lane := range mod.Lanes {
			itemCount := 0
			// CountWorkItems panics if cache not initialized; recover gracefully
			func() {
				defer func() { recover() }()
				itemCount = lane.CountWorkItems()
			}()
			out.Lanes = append(out.Lanes, ChatLaneDetail{
				Module:    mod.Name,
				Name:      lane.Name,
				Purpose:   lane.Purpose,
				MaxAgents: lane.MaxAgents,
				ItemCount: itemCount,
			})
		}
	}

	return nil, out, nil
}
