package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatMemorySearchInput struct {
	Query string `json:"query" jsonschema:"Search query text"`
	ExtId string `json:"ext_id,omitempty" jsonschema:"External chat id to scope search (omit for current chat)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum results (default: 10)"`
}

type ChatMemorySearchOutput struct {
	Results []SearchResultView `json:"results"`
	Error   string             `json:"error,omitempty"`
}

const tnChatMemorySearch = "chat_memory_search"

func init() {
	chatTools[tnChatMemorySearch] = &mcp.Tool{
		Description: "Search past conversations using full-text search across all your chat sessions.",
	}
}

func ChatMemorySearchHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatMemorySearchInput,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (*mcp.CallToolResult, ChatMemorySearchOutput, error) {
	out := ChatMemorySearchOutput{}

	if input.Query == "" {
		out.Error = "query is required"
		return nil, out, nil
	}

	if input.ExtId == "" {
		out.Error = "user_id is required"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = memory.SearchLimit
	}

	results, err := chatStore.Search(input.ExtId, input.Query, limit)
	if err != nil {
		out.Error = fmt.Sprintf("search failed: %v", err)
		return nil, out, nil
	}

	for _, r := range results {
		out.Results = append(out.Results, SearchResultView{
			Content:   r.Content,
			Role:      r.Role,
			SessionID: r.SessionID,
			Rank:      r.Rank,
			Time:      r.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, out, nil
}
