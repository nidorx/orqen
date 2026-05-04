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

// ── mem_search ─────────────────────────────────────────────────────
// Search your persistent memory across all sessions.

type MemSearchInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Query      string  `json:"query" jsonschema:"Search query — natural language or keywords"`
	Type       *string `json:"type,omitempty" jsonschema:"Filter by type: tool_use, file_change, command, file_read, search, manual, decision, architecture, bugfix, pattern"`
	Project    *string `json:"project,omitempty" jsonschema:"Filter by project name"`
	Scope      *string `json:"scope,omitempty" jsonschema:"Filter by scope: project (default) or personal"`
	Limit      *int    `json:"limit,omitempty" jsonschema:"Max results (default: 10, max: 20)"`
}

func (i *MemSearchInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSearchOutput struct {
	Result        string `json:"result"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
}

const tnMemSearch = "mem_search"

func init() {
	tools[tnMemSearch] = &mcp2.Tool{
		Description: "Search your persistent memory across all sessions. Use this to find past decisions, bugs fixed, patterns used, files changed, or any context from previous coding sessions.",
		Title:       "Search Memory",
		Annotations: &mcp2.ToolAnnotations{
			Title:           "Search Memory",
			ReadOnlyHint:    true,
			DestructiveHint: ptrBool(false),
			IdempotentHint:  true,
			OpenWorldHint:   ptrBool(false),
		},
	}
}

func MemSearchHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSearchInput, proj *project.Project) (*mcp2.CallToolResult, MemSearchOutput, error) {
	return nil, MemSearchOutput{}, nil
}

func add_tool_mem_search(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_search",
			mcp.WithDescription("Search your persistent memory across all sessions. Use this to find past decisions, bugs fixed, patterns used, files changed, or any context from previous coding sessions."),
			mcp.WithTitleAnnotation("Search Memory"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query — natural language or keywords"),
			),
			mcp.WithString("type",
				mcp.Description("Filter by type: tool_use, file_change, command, file_read, search, manual, decision, architecture, bugfix, pattern"),
			),
			mcp.WithString("project",
				mcp.Description("Filter by project name"),
			),
			mcp.WithString("scope",
				mcp.Description("Filter by scope: project (default) or personal"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Max results (default: 10, max: 20)"),
			),
		),
		handleSearch(s, cfg, activity),
	)
}

func handleSearch(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, _ := req.GetArguments()["query"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		projectOverride, _ := req.GetArguments()["project"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		limit := intArg(req, "limit", 10)

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

		results, err := s.Search(query, store.SearchOptions{
			Type:    typ,
			Project: project,
			Scope:   scope,
			Limit:   limit,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search error: %s. Try simpler keywords.", err)), nil
		}

		if len(results) == 0 {
			// JW4: use respondWithProject even for empty results.
			return respondWithProject(detRes, fmt.Sprintf("No memories found for: %q", query), nil), nil
		}

		// Batch-load relations for all results (REQ-002). Avoids N+1.
		syncIDs := make([]string, 0, len(results))
		for _, r := range results {
			if r.SyncID != "" {
				syncIDs = append(syncIDs, r.SyncID)
			}
		}
		relationsMap := map[string]store.ObservationRelations{}
		if len(syncIDs) > 0 {
			if rm, relErr := s.GetRelationsForObservations(syncIDs); relErr == nil {
				relationsMap = rm
			}
			// Errors from relation loading are swallowed — search must not fail.
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d memories:\n\n", len(results))
		anyTruncated := false
		for i, r := range results {
			projectDisplay := ""
			if r.Project != nil {
				projectDisplay = fmt.Sprintf(" | project: %s", *r.Project)
			}
			preview := truncate(r.Content, 300)
			if len(r.Content) > 300 {
				anyTruncated = true
				preview += " [preview]"
			}
			fmt.Fprintf(&b, "[%d] #%d (%s) — %s\n    %s\n    %s%s | scope: %s\n",
				i+1, r.ID, r.Type, r.Title,
				preview,
				r.CreatedAt, projectDisplay, r.Scope)

			// Append relation annotations. Skip orphaned (filtered by store).
			//
			// Annotation format contract (REQ-012, Design §7):
			//   supersedes: #<id> (<title>)            judged supersedes
			//   superseded_by: #<id> (<title>)         judged superseded_by
			//   conflicts: #<id> (<title>)             judged conflicts_with
			//   conflict: contested by #<id> (pending) pending (UNCHANGED from Phase 1)
			//
			// <id> is the observation's integer primary key. <title> is the related
			// observation's title; "(deleted)" when the observation is missing or soft-deleted.
			// Prefixes (supersedes:, superseded_by:, conflicts:) are stable across Phase 3.
			if rels, ok := relationsMap[r.SyncID]; ok {
				for _, rel := range rels.AsSource {
					switch {
					case rel.Relation == store.RelationSupersedes && rel.JudgmentStatus == store.JudgmentStatusJudged:
						title := rel.TargetTitle
						if rel.TargetMissing || title == "" {
							title = "deleted"
						}
						fmt.Fprintf(&b, "    supersedes: #%d (%s)\n", rel.TargetIntID, title)
					case rel.Relation == store.RelationConflictsWith && rel.JudgmentStatus == store.JudgmentStatusJudged:
						title := rel.TargetTitle
						if rel.TargetMissing || title == "" {
							title = "deleted"
						}
						fmt.Fprintf(&b, "    conflicts: #%d (%s)\n", rel.TargetIntID, title)
					case rel.JudgmentStatus == store.JudgmentStatusPending:
						// UNCHANGED from Phase 1 — byte-for-byte preserved.
						fmt.Fprintf(&b, "    conflict: contested by #%s (pending)\n", rel.TargetID)
					}
				}
				for _, rel := range rels.AsTarget {
					switch {
					case rel.Relation == store.RelationSupersedes && rel.JudgmentStatus == store.JudgmentStatusJudged:
						title := rel.SourceTitle
						if rel.SourceMissing || title == "" {
							title = "deleted"
						}
						fmt.Fprintf(&b, "    superseded_by: #%d (%s)\n", rel.SourceIntID, title)
					case rel.JudgmentStatus == store.JudgmentStatusPending:
						// UNCHANGED from Phase 1 — byte-for-byte preserved.
						fmt.Fprintf(&b, "    conflict: contested by #%s (pending)\n", rel.SourceID)
					}
				}
			}
			b.WriteString("\n")
		}
		if anyTruncated {
			fmt.Fprintf(&b, "---\nResults above are previews (300 chars). To read the full content of a specific memory, call mem_get_observation(id: <ID>).\n")
		}

		if nudge := activity.NudgeIfNeeded(sessionID); nudge != "" {
			b.WriteString(nudge)
		}

		// JW4: use respondWithProject for the success path (REQ-314).
		return respondWithProject(detRes, b.String(), nil), nil
	}
}
