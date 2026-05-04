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

// ── mem_get_observation ────────────────────────────────────────────
// Get the full content of a specific observation by ID.

type MemGetObservationInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	ID         int64   `json:"id" jsonschema:"The observation ID to retrieve"`
}

func (i *MemGetObservationInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemGetObservationOutput struct {
	Message        string `json:"message"`
	Project        string `json:"project"`
	ProjectSource  string `json:"project_source"`
	ProjectPath    string `json:"project_path"`
	ID             int64  `json:"id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	SessionID      string `json:"session_id"`
	ProjectName    string `json:"project_name,omitempty"`
	Scope          string `json:"scope"`
	TopicKey       string `json:"topic_key,omitempty"`
	ToolName       string `json:"tool_name,omitempty"`
	DuplicateCount int    `json:"duplicate_count"`
	RevisionCount  int    `json:"revision_count"`
	CreatedAt      string `json:"created_at"`
	Error          string `json:"error,omitempty"`
}

const tnMemGetObservation = "mem_get_observation"

func init() {
	tools[tnMemGetObservation] = &mcp2.Tool{
		Description: "Get the full content of a specific observation by ID. Use when you need the complete, untruncated content of an observation found via mem_search or mem_timeline.",
	}
}

// MemGetObservationHandler migrates from handleGetObservation in mcp.go.
func MemGetObservationHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemGetObservationInput, proj *project.Project) (*mcp2.CallToolResult, MemGetObservationOutput, error) {
	out := MemGetObservationOutput{}

	if input.ID == 0 {
		out.Error = "id is required"
		return nil, out, nil
	}

	// TODO: Wire up actual store.GetObservation call
	// TODO: Wire up project resolution (resolveReadProject)

	out.Message = fmt.Sprintf("#%d observation", input.ID)

	return nil, out, nil
}

func add_tool_mem_get_observation(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_get_observation",
			mcp.WithDescription("Get the full content of a specific observation by ID. Use when you need the complete, untruncated content of an observation found via mem_search or mem_timeline."),
			mcp.WithTitleAnnotation("Get Observation"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithNumber("id",
				mcp.Required(),
				mcp.Description("The observation ID to retrieve"),
			),
		),
		handleGetObservation(s),
	)
}

func handleGetObservation(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		obs, err := s.GetObservation(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Observation #%d not found", id)), nil
		}

		// Resolve project from cwd (REQ-310, REQ-314). No override possible for
		// get-by-ID — always auto-detect. JW5: use resolveReadProject (read semantics).
		// Tolerant: don't fail the fetch on resolution error; degrade to plain text.
		detRes, detErr := resolveReadProject(s, "")

		obsProject := ""
		if obs.Project != nil {
			obsProject = fmt.Sprintf("\nProject: %s", *obs.Project)
		}
		scope := fmt.Sprintf("\nScope: %s", obs.Scope)
		topic := ""
		if obs.TopicKey != nil {
			topic = fmt.Sprintf("\nTopic: %s", *obs.TopicKey)
		}
		toolName := ""
		if obs.ToolName != nil {
			toolName = fmt.Sprintf("\nTool: %s", *obs.ToolName)
		}
		duplicateMeta := fmt.Sprintf("\nDuplicates: %d", obs.DuplicateCount)
		revisionMeta := fmt.Sprintf("\nRevisions: %d", obs.RevisionCount)

		result := fmt.Sprintf("#%d [%s] %s\n%s\nSession: %s%s%s\nCreated: %s",
			obs.ID, obs.Type, obs.Title,
			obs.Content,
			obs.SessionID, obsProject+scope+topic, toolName+duplicateMeta+revisionMeta,
			obs.CreatedAt,
		)

		if detErr != nil {
			// Degraded path: resolution failed (e.g. ambiguous cwd). Return
			// the observation content without envelope rather than erroring.
			return mcp.NewToolResultText(result), nil
		}
		return respondWithProject(detRes, result, nil), nil
	}
}
