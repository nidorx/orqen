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

// ── mem_session_start ──────────────────────────────────────────────
// Register the start of a new coding session.

type MemSessionStartInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	ID         string  `json:"id" jsonschema:"Unique session identifier"`
	Directory  *string `json:"directory,omitempty" jsonschema:"Working directory"`
}

func (i *MemSessionStartInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSessionStartOutput struct {
	Message       string `json:"message"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
	Error         string `json:"error,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
}

const tnMemSessionStart = "mem_session_start"

func init() {
	tools[tnMemSessionStart] = &mcp2.Tool{
		Description: "Register the start of a new coding session. Call this at the beginning of a session to track activity.",
	}
}

// MemSessionStartHandler migrates from handleSessionStart in mcp.go.
func MemSessionStartHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSessionStartInput, proj *project.Project) (*mcp2.CallToolResult, MemSessionStartOutput, error) {
	out := MemSessionStartOutput{}

	if input.ID == "" {
		out.Error = "id is required"
		return nil, out, nil
	}

	// TODO: Wire up actual store.CreateSession call
	// TODO: Wire up project resolution (resolveSessionStartProject)

	directory := derefStr(input.Directory)
	out.Message = fmt.Sprintf("Session %q started (directory: %s)", input.ID, directory)

	return nil, out, nil
}

func add_tool_mem_session_start(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_session_start",
			mcp.WithDescription("Register the start of a new coding session. Call this at the beginning of a session to track activity."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Start Session"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Unique session identifier"),
			),
			mcp.WithString("directory",
				mcp.Description("Working directory"),
			),
		),
		queuedWriteHandler(getWriteQueue(), handleSessionStart(s, cfg, activity)),
	)
}

func handleSessionStart(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := req.GetArguments()["id"].(string)
		directory, _ := req.GetArguments()["directory"].(string)
		explicitDirectory := strings.TrimSpace(directory)
		// project field intentionally not read — auto-detect only (REQ-308)

		detRes, err := resolveSessionStartProject(explicitDirectory)
		if err != nil {
			return writeProjectErrorResult(detRes, err), nil
		}
		project, _ := store.NormalizeProject(detRes.Project)

		activity.RecordToolCall(defaultSessionID(project))
		if explicitDirectory == "" {
			directory = strings.TrimSpace(detRes.Path)
			if directory == "" {
				directory = currentWorkingDirectory()
			}
		}

		if err := s.CreateSession(id, project, directory); err != nil {
			return mcp.NewToolResultError("Failed to start session: " + err.Error()), nil
		}

		detRes.Project = project
		return respondWithProject(detRes, fmt.Sprintf("Session %q started for project %q", id, project), nil), nil
	}
}
