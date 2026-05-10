package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	projectpkg "github.com/nidorx/orqen/pkg/memory/project"
	"github.com/nidorx/orqen/pkg/memory/store"
)

// ── mem_session_end ────────────────────────────────────────────────
// Mark a coding session as completed.

type MemSessionEndInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	ID         string  `json:"id" jsonschema:"Session identifier to close"`
	Summary    *string `json:"summary,omitempty" jsonschema:"Summary of what was accomplished"`
}

func (i *MemSessionEndInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSessionEndOutput struct {
	Message       string `json:"message"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
	Error         string `json:"error,omitempty"`
}

const tnMemSessionEnd = "mem_session_end"

func init() {
	tools[tnMemSessionEnd] = &mcp2.Tool{
		Description: "Mark a coding session as completed with an optional summary.",
	}
}

// MemSessionEndHandler migrates from handleSessionEnd in mcp.go.
func MemSessionEndHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSessionEndInput, proj *engine.Project) (*mcp2.CallToolResult, MemSessionEndOutput, error) {
	out := MemSessionEndOutput{}

	if input.ID == "" {
		out.Error = "id is required"
		return nil, out, nil
	}

	// TODO: Wire up actual store.EndSession call
	// TODO: Wire up project resolution (resolveWriteProject)

	out.Message = fmt.Sprintf("Session %q completed", input.ID)

	return nil, out, nil
}

func add_tool_mem_session_end(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_session_end",
			mcp.WithDescription("Mark a coding session as completed with an optional summary."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("End Session"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Session identifier to close"),
			),
			mcp.WithString("summary",
				mcp.Description("Summary of what was accomplished"),
			),
		),
		QueuedWriteHandler(getWriteQueue(), handleSessionEnd(s, cfg, activity)),
	)
}

func handleSessionEnd(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := req.GetArguments()["id"].(string)
		summary, _ := req.GetArguments()["summary"].(string)
		// project field intentionally not read — auto-detect only (REQ-308)

		detRes, err := resolveWriteProject()
		if err != nil {
			if errors.Is(err, projectpkg.ErrInvalidConfig) {
				return writeProjectErrorResult(detRes, err), nil
			}
			// For session end, still complete the operation even if project resolution fails.
			// Use basename fallback.
			cwd, _ := os.Getwd()
			detRes = projectpkg.DetectionResult{
				Project: projectpkg.DetectProject(cwd),
				Source:  "dir_basename",
				Path:    cwd,
			}
		}
		project, _ := store.NormalizeProject(detRes.Project)

		if err := s.EndSession(id, summary); err != nil {
			return mcp.NewToolResultError("Failed to end session: " + err.Error()), nil
		}

		activity.ClearSession(defaultSessionID(project))

		detRes.Project = project
		return respondWithProject(detRes, fmt.Sprintf("Session %q completed", id), nil), nil
	}
}
