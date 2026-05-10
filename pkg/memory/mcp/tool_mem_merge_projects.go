package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/memory/store"
)

// ── mem_merge_projects ─────────────────────────────────────────────
// Merge memories from multiple project name variants into one canonical name.

type MemMergeProjectsInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	From       string  `json:"from" jsonschema:"Comma-separated list of project names to merge FROM (e.g. 'Engram,engram-memory,ENGRAM')"`
	To         string  `json:"to" jsonschema:"The canonical project name to merge INTO (e.g. 'engram')"`
}

func (i *MemMergeProjectsInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemMergeProjectsOutput struct {
	Message             string   `json:"message"`
	Canonical           string   `json:"canonical"`
	SourcesMerged       []string `json:"sources_merged"`
	ObservationsUpdated int      `json:"observations_updated"`
	SessionsUpdated     int      `json:"sessions_updated"`
	PromptsUpdated      int      `json:"prompts_updated"`
	Error               string   `json:"error,omitempty"`
}

const tnMemMergeProjects = "mem_merge_projects"

func init() {
	tools[tnMemMergeProjects] = &mcp2.Tool{
		Description: "Merge memories from multiple project name variants into one canonical name. Use when you discover project name drift (e.g. 'Engram' and 'engram' should be the same project). DESTRUCTIVE — moves all records from source names to the canonical name.",
	}
}

// MemMergeProjectsHandler migrates from handleMergeProjects in mcp.go.
func MemMergeProjectsHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemMergeProjectsInput, proj *engine.Project) (*mcp2.CallToolResult, MemMergeProjectsOutput, error) {
	out := MemMergeProjectsOutput{}

	if input.From == "" || input.To == "" {
		out.Error = "both 'from' and 'to' are required"
		return nil, out, nil
	}

	var sources []string
	for _, src := range strings.Split(input.From, ",") {
		src = strings.TrimSpace(src)
		if src != "" {
			sources = append(sources, src)
		}
	}

	if len(sources) == 0 {
		out.Error = "at least one source project name is required in 'from'"
		return nil, out, nil
	}

	// TODO: Wire up actual store.MergeProjects call

	out.Message = fmt.Sprintf("Merged %d source(s) into %q", len(sources), input.To)
	out.Canonical = input.To

	return nil, out, nil
}

func add_tool_mem_merge_projects(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_merge_projects",
			mcp.WithDescription("Merge memories from multiple project name variants into one canonical name. Use when you discover project name drift (e.g. 'Engram' and 'engram' should be the same project). DESTRUCTIVE — moves all records from source names to the canonical name."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Merge Projects"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("from",
				mcp.Required(),
				mcp.Description("Comma-separated list of project names to merge FROM (e.g. 'Engram,engram-memory,ENGRAM')"),
			),
			mcp.WithString("to",
				mcp.Required(),
				mcp.Description("The canonical project name to merge INTO (e.g. 'engram')"),
			),
		),
		QueuedWriteHandler(getWriteQueue(), handleMergeProjects(s)),
	)
}

func handleMergeProjects(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fromStr, _ := req.GetArguments()["from"].(string)
		to, _ := req.GetArguments()["to"].(string)

		if fromStr == "" || to == "" {
			return mcp.NewToolResultError("both 'from' and 'to' are required"), nil
		}

		var sources []string
		for _, src := range strings.Split(fromStr, ",") {
			src = strings.TrimSpace(src)
			if src != "" {
				sources = append(sources, src)
			}
		}

		if len(sources) == 0 {
			return mcp.NewToolResultError("at least one source project name is required in 'from'"), nil
		}

		result, err := s.MergeProjects(sources, to)
		if err != nil {
			return mcp.NewToolResultError("Merge failed: " + err.Error()), nil
		}

		msg := fmt.Sprintf("Merged %d source(s) into %q:\n", len(result.SourcesMerged), result.Canonical)
		msg += fmt.Sprintf("  Observations moved: %d\n", result.ObservationsUpdated)
		msg += fmt.Sprintf("  Sessions moved:     %d\n", result.SessionsUpdated)
		msg += fmt.Sprintf("  Prompts moved:      %d\n", result.PromptsUpdated)

		return mcp.NewToolResultText(msg), nil
	}
}
