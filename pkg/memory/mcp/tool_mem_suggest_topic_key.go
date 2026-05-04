package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// ── mem_suggest_topic_key ──────────────────────────────────────────
// Suggest a stable topic_key for memory upserts.

type MemSuggestTopicKeyInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Type       *string `json:"type,omitempty" jsonschema:"Observation type/category, e.g. architecture, decision, bugfix"`
	Title      *string `json:"title,omitempty" jsonschema:"Observation title (preferred input for stable keys)"`
	Content    *string `json:"content,omitempty" jsonschema:"Observation content used as fallback if title is empty"`
}

func (i *MemSuggestTopicKeyInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSuggestTopicKeyOutput struct {
	TopicKey string `json:"topic_key"`
	Error    string `json:"error,omitempty"`
}

const tnMemSuggestTopicKey = "mem_suggest_topic_key"

func init() {
	tools[tnMemSuggestTopicKey] = &mcp2.Tool{
		Description: "Suggest a stable topic_key for memory upserts. Use this before mem_save when you want evolving topics (like architecture decisions) to update a single observation over time.",
		Title:       "Suggest Topic Key",
		Annotations: &mcp2.ToolAnnotations{
			Title:           "Suggest Topic Key",
			ReadOnlyHint:    true,
			DestructiveHint: ptrBool(false),
			IdempotentHint:  true,
			OpenWorldHint:   ptrBool(false),
		},
	}
}

// MemSuggestTopicKeyHandler migrates from handleSuggestTopicKey in mcp.go.
func MemSuggestTopicKeyHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSuggestTopicKeyInput, proj *project.Project) (*mcp2.CallToolResult, MemSuggestTopicKeyOutput, error) {
	return nil, MemSuggestTopicKeyOutput{}, nil
}

func add_tool_mem_suggest_topic_key(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_suggest_topic_key",
			mcp.WithDescription("Suggest a stable topic_key for memory upserts. Use this before mem_save when you want evolving topics (like architecture decisions) to update a single observation over time."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Suggest Topic Key"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("type",
				mcp.Description("Observation type/category, e.g. architecture, decision, bugfix"),
			),
			mcp.WithString("title",
				mcp.Description("Observation title (preferred input for stable keys)"),
			),
			mcp.WithString("content",
				mcp.Description("Observation content used as fallback if title is empty"),
			),
		),
		handleSuggestTopicKey(),
	)
}

func handleSuggestTopicKey() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typ, _ := req.GetArguments()["type"].(string)
		title, _ := req.GetArguments()["title"].(string)
		content, _ := req.GetArguments()["content"].(string)

		if strings.TrimSpace(title) == "" && strings.TrimSpace(content) == "" {
			return mcp.NewToolResultError("provide title or content to suggest a topic_key"), nil
		}

		topicKey := suggestTopicKey(typ, title, content)
		if topicKey == "" {
			return mcp.NewToolResultError("could not suggest topic_key from input"), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Suggested topic_key: %s", topicKey)), nil
	}
}
