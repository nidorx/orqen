package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// ── mem_context ────────────────────────────────────────────────────
// Get recent memory context from previous sessions.

type MemContextInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Project    *string `json:"project,omitempty" jsonschema:"Filter by project (omit for all projects)"`
	Scope      *string `json:"scope,omitempty" jsonschema:"Filter observations by scope: project (default) or personal"`
}

func (i *MemContextInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemContextOutput struct {
	Message           string `json:"message"`
	Project           string `json:"project"`
	ProjectSource     string `json:"project_source"`
	ProjectPath       string `json:"project_path"`
	TotalSessions     int    `json:"total_sessions,omitempty"`
	TotalObservations int    `json:"total_observations,omitempty"`
	Projects          string `json:"projects,omitempty"`
	Nudge             string `json:"nudge,omitempty"`
	Error             string `json:"error,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
}

const tnMemContext = "mem_context"

func init() {
	tools[tnMemContext] = &mcp2.Tool{
		Description: "Get recent memory context from previous sessions. Shows recent sessions and observations to understand what was done before.",
	}
}

// MemContextHandler migrates from handleContext in mcp.go.
func MemContextHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemContextInput, proj *project.Project) (*mcp2.CallToolResult, MemContextOutput, error) {
	out := MemContextOutput{}

	// TODO: Wire up actual store.FormatContext call
	// TODO: Wire up project resolution (resolveReadProject)
	// TODO: Wire up stats call

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Context for project: %s", derefStr(input.Project)))

	out.Message = b.String()

	return nil, out, nil
}

func add_tool_mem_context(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_context",
			mcp.WithDescription("Get recent memory context from previous sessions. Shows recent sessions and observations to understand what was done before."),
			mcp.WithTitleAnnotation("Get Memory Context"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("project",
				mcp.Description("Filter by project (omit for all projects)"),
			),
			mcp.WithString("scope",
				mcp.Description("Filter observations by scope: project (default) or personal"),
			),
			// JW7: limit param removed — schema advertised it but handleContext never read it.
		),
		handleContext(s, cfg, activity),
	)
}

func handleContext(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectOverride, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)

		// Resolve project: validate override or auto-detect (REQ-310, REQ-311)
		detRes, err := resolveReadProject(s, projectOverride)
		if err != nil {
			var upe *unknownProjectError
			if errors.As(err, &upe) {
				return errorWithMeta("unknown_project",
					fmt.Sprintf("Project %q not found in store", upe.Name),
					upe.AvailableProjects,
				), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Project resolution failed: %s", err)), nil
		}
		project := detRes.Project
		project, _ = store.NormalizeProject(project)
		detRes.Project = project // JR2-1: keep envelope in sync with normalized query project

		sessionID := defaultSessionID(project)
		activity.RecordToolCall(sessionID)

		contextResult, err := s.FormatContext(project, scope)
		if err != nil {
			return mcp.NewToolResultError("Failed to get context: " + err.Error()), nil
		}

		if contextResult == "" {
			return respondWithProject(detRes, "No previous session memories found.", nil), nil
		}

		stats, _ := s.Stats()
		var projects string
		if len(stats.Projects) > 0 {
			projects = strings.Join(stats.Projects, ", ")
		} else {
			projects = "none"
		}

		result := fmt.Sprintf("%s\n---\nMemory stats: %d sessions, %d observations across projects: %s",
			contextResult, stats.TotalSessions, stats.TotalObservations, projects)

		if nudge := activity.NudgeIfNeeded(sessionID); nudge != "" {
			result += nudge
		}

		return respondWithProject(detRes, result, nil), nil
	}
}
