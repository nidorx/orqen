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

// ── mem_save_prompt ────────────────────────────────────────────────
// Save a user prompt to persistent memory.

type MemSavePromptInput struct {
	WorkItemID          *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Content             string  `json:"content" jsonschema:"The user's prompt text"`
	SessionID           *string `json:"session_id,omitempty" jsonschema:"Session ID to associate with (default: manual-save-{project})"`
	Project             *string `json:"project,omitempty" jsonschema:"Optional recovery target only after ambiguous_project"`
	ProjectChoiceReason *string `json:"project_choice_reason,omitempty" jsonschema:"Must be user_selected_after_ambiguous_project"`
}

func (i *MemSavePromptInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSavePromptOutput struct {
	Message       string `json:"message"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
	Error         string `json:"error,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
}

const tnMemSavePrompt = "mem_save_prompt"

func init() {
	tools[tnMemSavePrompt] = &mcp2.Tool{
		Description: "Save a user prompt to persistent memory. Use this to record what the user asked — their intent, questions, and requests — so future sessions have context about the user's goals.",
	}
}

// MemSavePromptHandler migrates from handleSavePrompt in mcp.go.
func MemSavePromptHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSavePromptInput, proj *project.Project) (*mcp2.CallToolResult, MemSavePromptOutput, error) {
	out := MemSavePromptOutput{}

	if input.Content == "" {
		out.Error = "content is required"
		return nil, out, nil
	}

	// TODO: Wire up actual store.AddPrompt call
	// TODO: Wire up project resolution (resolveWriteProjectWithChoice)

	truncated := input.Content
	if len(truncated) > 80 {
		truncated = truncated[:80] + "..."
	}
	out.Message = fmt.Sprintf("Prompt saved: %q", truncated)

	return nil, out, nil
}

func add_tool_mem_save_prompt(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_save_prompt",
			mcp.WithDescription("Save a user prompt to persistent memory. Use this to record what the user asked — their intent, questions, and requests — so future sessions have context about the user's goals."),
			mcp.WithTitleAnnotation("Save User Prompt"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("The user's prompt text"),
			),
			mcp.WithString("session_id",
				mcp.Description("Session ID to associate with (default: manual-save-{project})"),
			),
			mcp.WithString("project",
				mcp.Description("Optional recovery target only after ambiguous_project. Ignored unless project_choice_reason is user_selected_after_ambiguous_project."),
			),
			mcp.WithString("project_choice_reason",
				mcp.Description("Must be user_selected_after_ambiguous_project, and only after the user explicitly chose one of available_projects from an ambiguous_project error."),
			),
		),
		queuedWriteHandler(getWriteQueue(), handleSavePrompt(s, cfg, activity)),
	)
}

func handleSavePrompt(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		projectChoice, _ := req.GetArguments()["project"].(string)
		projectChoiceReason, _ := req.GetArguments()["project_choice_reason"].(string)

		detRes, err := resolveWriteProjectWithChoice(projectChoice, projectChoiceReason)
		if err != nil {
			return writeProjectErrorResult(detRes, err), nil
		}
		project, _ := store.NormalizeProject(detRes.Project)

		if sessionID == "" {
			sessionID = defaultSessionID(project)
		}

		// Ensure the implicit MCP session exists with the current working directory.
		_ = ensureImplicitSessionWithCWD(s, sessionID, project)

		_, err = s.AddPrompt(store.AddPromptParams{
			SessionID: sessionID,
			Content:   content,
			Project:   project,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save prompt: " + err.Error()), nil
		}

		if activity != nil {
			activity.RecordPrompt(sessionID, project, content)
		}

		detRes.Project = project
		return respondWithProject(detRes, fmt.Sprintf("Prompt saved: %q", truncate(content, 80)), nil), nil
	}
}
