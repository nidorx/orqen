package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatWorkitemCreateInput struct {
	Lane    string `json:"lane" jsonschema:"Destination lane name"`
	Title   string `json:"title" jsonschema:"Work item title"`
	Content string `json:"content,omitempty" jsonschema:"Optional content/description"`
}

type ChatWorkitemCreateOutput struct {
	Success  bool             `json:"success"`
	WorkItem *engine.WorkItem `json:"workitem,omitempty"`
	Error    string           `json:"error,omitempty"`
}

const tnChatWorkitemCreate = "chat_workitem_create"

func init() {
	chatTools[tnChatWorkitemCreate] = &mcp.Tool{
		Description: "Create a new workitem in a specified lane. Creates the directory and .yaml file.",
	}
}

func ChatWorkitemCreateHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatWorkitemCreateInput,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (*mcp.CallToolResult, ChatWorkitemCreateOutput, error) {
	out := ChatWorkitemCreateOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Lane == "" {
		out.Error = "lane is required"
		return nil, out, nil
	}

	if input.Title == "" {
		out.Error = "title is required"
		return nil, out, nil
	}

	// Find the lane across all modules
	var targetLane *engine.Lane
	for _, mod := range proj.Modules {
		if l := mod.GetLane(input.Lane); l != nil {
			targetLane = l
			break
		}
	}
	if targetLane == nil {
		out.Error = fmt.Sprintf("lane %q not found", input.Lane)
		return nil, out, nil
	}

	wi, err := targetLane.CreateWorkItem(input.Title)
	if err != nil {
		out.Error = fmt.Sprintf("failed to create workitem: %v", err)
		return nil, out, nil
	}

	// If content was provided, write it to the yaml file
	if input.Content != "" {
		yamlPath := filepath.Join(wi.Lane.DirAbs, wi.Name+".yaml")
		if err := os.WriteFile(yamlPath, []byte(input.Content), 0644); err != nil {
			out.Error = fmt.Sprintf("created workitem but failed to write content: %v", err)
			return nil, out, nil
		}
	}

	out.Success = true
	out.WorkItem = wi
	return nil, out, nil
}
