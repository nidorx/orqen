package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

type WorkitemInput struct {
	Module      *string `json:"module,omitempty" jsonschema:"module name (omit if the project only has one module)"`
	WorkitemSeq int     `json:"workitem_seq" jsonschema:"sequential ID of the work item to move"`
}

type ChatWorkitemGetOutput struct {
	Found bool                  `json:"found"`
	Item  *engine.WorkItemAlias `json:"item,omitempty"`
	Error string                `json:"error,omitempty"`
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
	input *WorkitemInput,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (*mcp.CallToolResult, ChatWorkitemGetOutput, error) {
	out := ChatWorkitemGetOutput{}

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

	// Find the work item
	var item *engine.WorkItem
	if input.WorkitemSeq > 0 {
		item = targetModule.GetWorkItemBySeq(input.WorkitemSeq)
	}
	if item == nil {
		out.Error = fmt.Sprintf("work item not found (workitem_seq=%d)", input.WorkitemSeq)
		return nil, out, nil
	}

	out.Found = true
	out.Item = item.Alias()
	return nil, out, nil
}
