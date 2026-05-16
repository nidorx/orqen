package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

// Chat tool handler type and adapter

// ToolChatHandler is the generic handler signature for chat MCP tools.
type ToolChatHandler[In, Out any] func(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input In,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (result *mcp.CallToolResult, output Out, err error)

// chatHandler2MCP adapts a ToolChatHandler into an mcp.ToolHandlerFor by closing
// over proj, chatStore, and sessionMgr.
func chatHandler2MCP[In, Out any](
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
	h ToolChatHandler[In, Out],
) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		return h(ctx, req, input, proj, chatStore, sessionMgr)
	}
}

// chatTools holds all registered chat tools. Populated by init() in each section.
var chatTools = map[string]*mcp.Tool{}

// addChatToolWithHandler registers a tool with its handler.
func addChatToolWithHandler[In, Out any](s *mcp.Server, name string, h ToolChatHandler[In, Out], proj *engine.Project, chatStore *memory.ChatStore, sessionMgr *memory.SessionManager) {
	tool := chatTools[name]
	if tool == nil {
		panic(fmt.Sprintf("chat: tool %q not registered in chatTools map", name))
	}
	tool.Name = name
	mcp.AddTool(s, tool, chatHandler2MCP(proj, chatStore, sessionMgr, h))
}

// Shared output view types

type MessageView struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

type SearchResultView struct {
	Content   string  `json:"content"`
	Role      string  `json:"role"`
	SessionID string  `json:"session_id"`
	Rank      float64 `json:"rank"`
	Time      string  `json:"time"`
}

type WorkitemSummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Lane  string `json:"lane"`
	Title string `json:"title"`
}

type FileEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "directory"
	Size int64  `json:"size"`
}

type ChatLaneDetail struct {
	Module    string `json:"module"`
	Name      string `json:"name"`
	Purpose   string `json:"purpose"`
	MaxAgents int    `json:"max_agents"`
	ItemCount int    `json:"item_count"`
}

type ModuleSummary struct {
	Name      string `json:"name"`
	ItemCount int    `json:"item_count"`
	LaneCount int    `json:"lane_count"`
}

type LaneStatSummary struct {
	Module string `json:"module"`
	Name   string `json:"lane"`
	Count  int    `json:"item_count"`
}

// JSON helpers

// toJSON marshals a value to a JSON string for MCP result content.
func toJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RegisterAllTools

// RegisterAllTools registers all chat tools with the given MCP server.
func RegisterAllTools(s *mcp.Server, proj *engine.Project, chatStore *memory.ChatStore, sessionMgr *memory.SessionManager) {
	addChatToolWithHandler(s, tnChatHistoryGet, ChatHistoryGetHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatMemorySearch, ChatMemorySearchHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatWorkitemCreate, ChatWorkitemCreateHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatWorkitemList, ChatWorkitemListHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatWorkitemGet, ChatWorkitemGetHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatFileList, ChatFileListHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatFileRead, ChatFileReadHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatFileEdit, ChatFileEditHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatProjectGet, ChatProjectGetHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatLaneList, ChatLaneListHandler, proj, chatStore, sessionMgr)
}
