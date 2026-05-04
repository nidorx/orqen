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

// ── mem_stats ──────────────────────────────────────────────────────
// Show memory system statistics.

type MemStatsInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Project    *string `json:"project,omitempty" jsonschema:"Project to echo in envelope context (omit for auto-detect)"`
}

func (i *MemStatsInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemStatsOutput struct {
	Message           string   `json:"message"`
	Project           string   `json:"project"`
	ProjectSource     string   `json:"project_source"`
	ProjectPath       string   `json:"project_path"`
	TotalSessions     int      `json:"total_sessions"`
	TotalObservations int      `json:"total_observations"`
	TotalPrompts      int      `json:"total_prompts"`
	Projects          []string `json:"projects"`
	Error             string   `json:"error,omitempty"`
	ErrorCode         string   `json:"error_code,omitempty"`
}

const tnMemStats = "mem_stats"

func init() {
	tools[tnMemStats] = &mcp2.Tool{
		Description: "Show memory system statistics — total sessions, observations, and projects tracked.",
	}
}

// MemStatsHandler migrates from handleStats in mcp.go.
func MemStatsHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemStatsInput, proj *project.Project) (*mcp2.CallToolResult, MemStatsOutput, error) {
	out := MemStatsOutput{}

	// TODO: Wire up actual store.Stats() call
	// TODO: Wire up project resolution (resolveReadProject)

	var projectsStr string
	if len(out.Projects) > 0 {
		projectsStr = strings.Join(out.Projects, ", ")
	} else {
		projectsStr = "none yet"
	}

	out.Message = fmt.Sprintf("Memory System Stats:\n- Sessions: %d\n- Observations: %d\n- Prompts: %d\n- Projects: %s",
		out.TotalSessions, out.TotalObservations, out.TotalPrompts, projectsStr)

	return nil, out, nil
}

func add_tool_mem_stats(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_stats",
			mcp.WithDescription("Show memory system statistics — total sessions, observations, and projects tracked."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Memory Stats"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("project",
				mcp.Description("Project to echo in envelope context (omit for auto-detect; stats themselves are global aggregates)"),
			),
		),
		handleStats(s),
	)
}

func handleStats(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectOverride, _ := req.GetArguments()["project"].(string)

		// Resolve project: validate override or auto-detect (REQ-310, REQ-311, REQ-314)
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

		stats, err := loadMCPStats(s)
		if err != nil {
			return mcp.NewToolResultError("Failed to get stats: " + err.Error()), nil
		}

		var projects string
		if len(stats.Projects) > 0 {
			projects = strings.Join(stats.Projects, ", ")
		} else {
			projects = "none yet"
		}

		result := fmt.Sprintf("Memory System Stats:\n- Sessions: %d\n- Observations: %d\n- Prompts: %d\n- Projects: %s",
			stats.TotalSessions, stats.TotalObservations, stats.TotalPrompts, projects)

		return respondWithProject(detRes, result, nil), nil
	}
}
