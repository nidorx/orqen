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

// ── mem_timeline ───────────────────────────────────────────────────
// Show chronological context around a specific observation.

type MemTimelineInput struct {
	WorkItemID    *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	ObservationID int64   `json:"observation_id" jsonschema:"The observation ID to center the timeline on"`
	Before        *int    `json:"before,omitempty" jsonschema:"Number of observations to show before the focus (default: 5)"`
	After         *int    `json:"after,omitempty" jsonschema:"Number of observations to show after the focus (default: 5)"`
	Project       *string `json:"project,omitempty" jsonschema:"Filter by project name (omit for auto-detect)"`
}

func (i *MemTimelineInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemTimelineEntry struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type MemTimelineSessionInfo struct {
	Project      string  `json:"project"`
	StartedAt    string  `json:"started_at"`
	Summary      *string `json:"summary,omitempty"`
	TotalInRange int     `json:"total_in_range"`
}

type MemTimelineOutput struct {
	Message       string                  `json:"message"`
	Project       string                  `json:"project"`
	ProjectSource string                  `json:"project_source"`
	ProjectPath   string                  `json:"project_path"`
	SessionInfo   *MemTimelineSessionInfo `json:"session_info,omitempty"`
	Before        []MemTimelineEntry      `json:"before,omitempty"`
	Focus         MemTimelineEntry        `json:"focus"`
	After         []MemTimelineEntry      `json:"after,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

const tnMemTimeline = "mem_timeline"

func init() {
	tools[tnMemTimeline] = &mcp2.Tool{
		Description: "Show chronological context around a specific observation. Use after mem_search to drill into the timeline of events surrounding a search result.",
	}
}

// MemTimelineHandler migrates from handleTimeline in mcp.go.
func MemTimelineHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemTimelineInput, proj *project.Project) (*mcp2.CallToolResult, MemTimelineOutput, error) {
	out := MemTimelineOutput{}

	if input.ObservationID == 0 {
		out.Error = "observation_id is required"
		return nil, out, nil
	}

	before := 5
	if input.Before != nil {
		before = *input.Before
	}
	after := 5
	if input.After != nil {
		after = *input.After
	}

	// TODO: Wire up actual store.Timeline call
	// TODO: Wire up project resolution (resolveReadProject)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Timeline for observation #%d (before=%d, after=%d)", input.ObservationID, before, after))

	out.Message = b.String()

	return nil, out, nil
}

func add_tool_mem_timeline(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_timeline",
			mcp.WithDescription("Show chronological context around a specific observation. Use after mem_search to drill into the timeline of events surrounding a search result. This is the progressive disclosure pattern: search first, then timeline to understand context."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Memory Timeline"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithNumber("observation_id",
				mcp.Required(),
				mcp.Description("The observation ID to center the timeline on (from mem_search results)"),
			),
			mcp.WithNumber("before",
				mcp.Description("Number of observations to show before the focus (default: 5)"),
			),
			mcp.WithNumber("after",
				mcp.Description("Number of observations to show after the focus (default: 5)"),
			),
			mcp.WithString("project",
				mcp.Description("Filter by project name (omit for auto-detect)"),
			),
		),
		handleTimeline(s),
	)
}

func handleTimeline(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		observationID := int64(intArg(req, "observation_id", 0))
		if observationID == 0 {
			return mcp.NewToolResultError("observation_id is required"), nil
		}
		before := intArg(req, "before", 5)
		after := intArg(req, "after", 5)
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

		result, err := s.Timeline(observationID, before, after)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Timeline error: %s", err)), nil
		}

		var b strings.Builder

		// Session header
		if result.SessionInfo != nil {
			summary := ""
			if result.SessionInfo.Summary != nil {
				summary = fmt.Sprintf(" — %s", truncate(*result.SessionInfo.Summary, 100))
			}
			fmt.Fprintf(&b, "Session: %s (%s)%s\n", result.SessionInfo.Project, result.SessionInfo.StartedAt, summary)
			fmt.Fprintf(&b, "Total observations in session: %d\n\n", result.TotalInRange)
		}

		// Before entries
		if len(result.Before) > 0 {
			b.WriteString("─── Before ───\n")
			for _, e := range result.Before {
				fmt.Fprintf(&b, "  #%d [%s] %s — %s\n", e.ID, e.Type, e.Title, truncate(e.Content, 150))
			}
			b.WriteString("\n")
		}

		// Focus observation (highlighted)
		fmt.Fprintf(&b, ">>> #%d [%s] %s <<<\n", result.Focus.ID, result.Focus.Type, result.Focus.Title)
		fmt.Fprintf(&b, "    %s\n", truncate(result.Focus.Content, 500))
		fmt.Fprintf(&b, "    %s\n\n", result.Focus.CreatedAt)

		// After entries
		if len(result.After) > 0 {
			b.WriteString("─── After ───\n")
			for _, e := range result.After {
				fmt.Fprintf(&b, "  #%d [%s] %s — %s\n", e.ID, e.Type, e.Title, truncate(e.Content, 150))
			}
		}

		return respondWithProject(detRes, b.String(), nil), nil
	}
}
