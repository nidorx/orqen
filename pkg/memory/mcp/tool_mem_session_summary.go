package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/memory/store"
)

// ── mem_session_summary ────────────────────────────────────────────
// Save a comprehensive end-of-session summary.

type MemSessionSummaryInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Content    string  `json:"content" jsonschema:"Full session summary using the Goal/Instructions/Discoveries/Accomplished/Files format"`
	SessionID  *string `json:"session_id,omitempty" jsonschema:"Session ID (default: manual-save-{project})"`
}

func (i *MemSessionSummaryInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSessionSummaryOutput struct {
	Message       string `json:"message"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
	ActivityScore string `json:"activity_score,omitempty"`
	Error         string `json:"error,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
}

const tnMemSessionSummary = "mem_session_summary"

func init() {
	tools[tnMemSessionSummary] = &mcp2.Tool{
		Description: `Save a comprehensive end-of-session summary. Call this when a session is ending or when significant work is complete.

FORMAT — use this exact structure in the content field:

## Goal
[One sentence: what were we building/working on in this session]

## Instructions
[User preferences, constraints, or context discovered during this session]

## Discoveries
- [Technical finding, gotcha, or learning 1]
- [Technical finding 2]

## Accomplished
- ✅ [Completed task 1]
- ✅ [Completed task 2]
- 🔲 [Identified but not yet done]

## Relevant Files
- path/to/file.ts — [what it does or what changed]`,
	}
}

// MemSessionSummaryHandler migrates from handleSessionSummary in mcp.go.
func MemSessionSummaryHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSessionSummaryInput, proj *engine.Project) (*mcp2.CallToolResult, MemSessionSummaryOutput, error) {
	out := MemSessionSummaryOutput{}

	if input.Content == "" {
		out.Error = "content is required"
		return nil, out, nil
	}

	// TODO: Wire up actual store.AddObservation call with type "session_summary"
	// TODO: Wire up project resolution (resolveWriteProject)

	out.Message = fmt.Sprintf("Session summary saved")

	return nil, out, nil
}

func add_tool_mem_session_summary(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_session_summary",
			mcp.WithTitleAnnotation("Save Session Summary"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithDescription(`Save a comprehensive end-of-session summary. Call this when a session is ending or when significant work is complete. This creates a structured summary that future sessions will use to understand what happened.

FORMAT — use this exact structure in the content field:

## Goal
[One sentence: what were we building/working on in this session]

## Instructions
[User preferences, constraints, or context discovered during this session. Things a future agent needs to know about HOW the user wants things done. Skip if nothing notable.]

## Discoveries
- [Technical finding, gotcha, or learning 1]
- [Technical finding 2]
- [Important API behavior, config quirk, etc.]

## Accomplished
- ✅ [Completed task 1 — with key implementation details]
- ✅ [Completed task 2 — mention files changed]
- 🔲 [Identified but not yet done — for next session]

## Relevant Files
- path/to/file.ts — [what it does or what changed]
- path/to/other.go — [role in the architecture]

GUIDELINES:
- Be CONCISE but don't lose important details (file paths, error messages, decisions)
- Focus on WHAT and WHY, not HOW (the code itself is in the repo)
- Include things that would save a future agent time
- The Discoveries section is the most valuable — capture gotchas and non-obvious learnings
- Relevant Files should only include files that were significantly changed or are important for context`),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("Full session summary using the Goal/Instructions/Discoveries/Accomplished/Files format"),
			),
			mcp.WithString("session_id",
				mcp.Description("Session ID (default: manual-save-{project})"),
			),
			// project field intentionally omitted — auto-detect only (REQ-308 write-tool contract)
		),
		QueuedWriteHandler(getWriteQueue(), handleSessionSummary(s, cfg, activity)),
	)
}

func handleSessionSummary(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		// project field intentionally not read — auto-detect only (REQ-308 write-tool contract)

		// Auto-detect project from cwd; fail fast on ambiguous (REQ-308, REQ-309)
		detRes, err := resolveWriteProject()
		if err != nil {
			return writeProjectErrorResult(detRes, err), nil
		}
		project, _ := store.NormalizeProject(detRes.Project)

		if sessionID == "" {
			sessionID = defaultSessionID(project)
		}

		// Ensure the implicit MCP session exists with the current working directory.
		_ = ensureImplicitSessionWithCWD(s, sessionID, project)

		_, err = s.AddObservation(store.AddObservationParams{
			SessionID: sessionID,
			Type:      "session_summary",
			Title:     fmt.Sprintf("Session summary: %s", project),
			Content:   content,
			Project:   project,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save session summary: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Session summary saved for project %q", project)
		if score := activity.ActivityScore(defaultSessionID(project)); score != "" {
			msg += "\n" + score
		}
		detRes.Project = project
		return respondWithProject(detRes, msg, nil), nil
	}
}
