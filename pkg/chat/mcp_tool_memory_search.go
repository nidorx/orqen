package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatMemorySearchInput struct {
	Query  string `json:"query" jsonschema:"Search query text"`
	UserID string `json:"user_id,omitempty" jsonschema:"User ID to scope search (omit for current user)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum results (default: 10)"`
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
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatMemorySearchOutput, error) {
	out := ChatMemorySearchOutput{}

	if input.Query == "" {
		out.Error = "query is required"
		return nil, out, nil
	}

	if input.UserID == "" {
		out.Error = "user_id is required"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = SearchLimit
	}

	results, err := chatStore.Search(input.UserID, input.Query, limit)
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
