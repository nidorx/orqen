package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// ── mem_update ─────────────────────────────────────────────────────
// Update an existing observation by ID.

type MemUpdateInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	ID         int64   `json:"id" jsonschema:"Observation ID to update"`
	Title      *string `json:"title,omitempty" jsonschema:"New title"`
	Content    *string `json:"content,omitempty" jsonschema:"New content"`
	Type       *string `json:"type,omitempty" jsonschema:"New type/category"`
	Scope      *string `json:"scope,omitempty" jsonschema:"New scope: project or personal"`
	TopicKey   *string `json:"topic_key,omitempty" jsonschema:"New topic key (normalized internally)"`
}

func (i *MemUpdateInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemUpdateOutput struct {
	Message       string `json:"message"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
	Truncated     bool   `json:"truncated,omitempty"`
	Error         string `json:"error,omitempty"`
}

const tnMemUpdate = "mem_update"

func init() {
	tools[tnMemUpdate] = &mcp2.Tool{
		Description: "Update an existing observation by ID. Only provided fields are changed.",
		Title:       "Update Memory",
		Annotations: &mcp2.ToolAnnotations{
			Title:           "Update Memory",
			ReadOnlyHint:    false,
			DestructiveHint: ptrBool(false),
			IdempotentHint:  false,
			OpenWorldHint:   ptrBool(false),
		},
	}
}

// MemUpdateHandler migrates from handleUpdate in mcp.go.
func MemUpdateHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemUpdateInput, proj *project.Project) (*mcp2.CallToolResult, MemUpdateOutput, error) {
	return nil, MemUpdateOutput{}, nil
}

func add_tool_mem_update(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_update",
			mcp.WithDescription("Update an existing observation by ID. Only provided fields are changed."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Update Memory"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithNumber("id",
				mcp.Required(),
				mcp.Description("Observation ID to update"),
			),
			mcp.WithString("title",
				mcp.Description("New title"),
			),
			mcp.WithString("content",
				mcp.Description("New content"),
			),
			mcp.WithString("type",
				mcp.Description("New type/category"),
			),
			mcp.WithString("scope",
				mcp.Description("New scope: project or personal"),
			),
			mcp.WithString("topic_key",
				mcp.Description("New topic key (normalized internally)"),
			),
		),
		queuedWriteHandler(getWriteQueue(), handleUpdate(s)),
	)
}

func handleUpdate(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		update := store.UpdateObservationParams{}
		if v, ok := req.GetArguments()["title"].(string); ok {
			update.Title = &v
		}
		if v, ok := req.GetArguments()["content"].(string); ok {
			update.Content = &v
		}
		if v, ok := req.GetArguments()["type"].(string); ok {
			update.Type = &v
		}
		// Tolerant parse: project still accepted even though removed from schema (REQ-308).
		if v, ok := req.GetArguments()["project"].(string); ok && v != "" {
			update.Project = &v
		}
		if v, ok := req.GetArguments()["scope"].(string); ok {
			update.Scope = &v
		}
		if v, ok := req.GetArguments()["topic_key"].(string); ok {
			update.TopicKey = &v
		}

		if update.Title == nil && update.Content == nil && update.Type == nil && update.Project == nil && update.Scope == nil && update.TopicKey == nil {
			return mcp.NewToolResultError("provide at least one field to update"), nil
		}

		var contentLen int
		if update.Content != nil {
			contentLen = len(*update.Content)
		}

		obs, err := s.UpdateObservation(id, update)
		if err != nil {
			return mcp.NewToolResultError("Failed to update memory: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Memory updated: #%d %q (%s, scope=%s)", obs.ID, obs.Title, obs.Type, obs.Scope)
		if contentLen > s.MaxObservationLength() {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", contentLen, s.MaxObservationLength())
		}

		// Auto-detect for envelope; tolerant — don't fail update on resolution error
		detRes, detErr := resolveWriteProject()
		if detErr != nil {
			// Still return success for the update itself.
			return mcp.NewToolResultText(msg), nil
		}
		return respondWithProject(detRes, msg, nil), nil
	}
}
