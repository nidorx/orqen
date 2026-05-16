package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatWorkitemGetInput struct {
	WorkItemID string `json:"workitem_id" jsonschema:"Work item ID to look up"`
}

type ChatWorkitemGetOutput struct {
	Found     bool             `json:"found"`
	WorkItem  *engine.WorkItem `json:"workitem,omitempty"`
	Lane      string           `json:"lane,omitempty"`
	Module    string           `json:"module,omitempty"`
	FileCount int              `json:"file_count,omitempty"`
	Error     string           `json:"error,omitempty"`
}

const tnChatWorkitemGet = "chat_workitem_get"

func init() {
	chatTools[tnChatWorkitemGet] = &mcp.Tool{
		Description: "Get details of a specific workitem including its files and attributes.",
	}
}

func ChatWorkitemGetHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatWorkitemGetInput,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (*mcp.CallToolResult, ChatWorkitemGetOutput, error) {
	out := ChatWorkitemGetOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.WorkItemID == "" {
		out.Error = "workitem_id is required"
		return nil, out, nil
	}

	// GetWorkItemById panics if cache not initialized; recover gracefully
	var wi *engine.WorkItem
	func() {
		defer func() { recover() }()
		wi = proj.GetWorkItemById(input.WorkItemID)
	}()
	if wi == nil {
		out.Error = fmt.Sprintf("workitem %q not found", input.WorkItemID)
		return nil, out, nil
	}

	out.Found = true
	out.WorkItem = wi
	out.Lane = wi.Lane.Name
	out.Module = wi.Lane.Module.Name
	out.FileCount = len(wi.Files)

	return nil, out, nil
}
