package chat

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatWorkitemListInput struct {
	Lane  string `json:"lane,omitempty" jsonschema:"Optional lane name to filter by"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum items to return (default: 20)"`
}

type ChatWorkitemListOutput struct {
	Items []WorkitemSummary `json:"items"`
	Error string            `json:"error,omitempty"`
}

const tnChatWorkitemList = "chat_workitem_list"

func init() {
	chatTools[tnChatWorkitemList] = &mcp.Tool{
		Description: "List workitems, optionally filtered by lane.",
	}
}

func ChatWorkitemListHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatWorkitemListInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatWorkitemListOutput, error) {
	out := ChatWorkitemListOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	count := 0
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			if input.Lane != "" && lane.Name != input.Lane {
				continue
			}
			// WorkItems() panics if cache not initialized; recover gracefully
			func() {
				defer func() { recover() }()
				for wi := range lane.WorkItems() {
					if count >= limit {
						break
					}
					title, _ := wi.Attributes["title"].(string)
					if title == "" {
						title = wi.Name
					}
					out.Items = append(out.Items, WorkitemSummary{
						ID:    wi.ID,
						Name:  wi.Name,
						Lane:  lane.Name,
						Title: title,
					})
					count++
				}
			}()
			if count >= limit {
				break
			}
		}
		if count >= limit {
			break
		}
	}

	return nil, out, nil
}
