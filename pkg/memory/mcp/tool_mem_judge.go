package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// tools is the local registry for this package. Each tool file registers via init().
var tools = map[string]*mcp2.Tool{}

// ── mem_judge ──────────────────────────────────────────────────────
// Record a verdict on a pending memory conflict surfaced by mem_save.

type MemJudgeInput struct {
	WorkItemID *string  `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	JudgmentID string   `json:"judgment_id" jsonschema:"The judgment_id from candidates[] in the mem_save response (format: rel-<hex>)"`
	Relation   string   `json:"relation" jsonschema:"Verdict: related | compatible | scoped | conflicts_with | supersedes | not_conflict"`
	Reason     *string  `json:"reason,omitempty" jsonschema:"Free-text explanation of the verdict"`
	Evidence   *string  `json:"evidence,omitempty" jsonschema:"Supporting evidence (JSON or free text)"`
	Confidence *float64 `json:"confidence,omitempty" jsonschema:"Confidence score 0.0..1.0 (default: 1.0)"`
	SessionID  *string  `json:"session_id,omitempty" jsonschema:"Session ID for provenance (default: auto)"`
}

func (i *MemJudgeInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemJudgeOutput struct {
	Relation *store.Relation `json:"relation"`
	Error    string          `json:"error,omitempty"`
}

const tnMemJudge = "mem_judge"

func init() {
	tools[tnMemJudge] = &mcp2.Tool{
		Description: `Record a verdict on a pending memory conflict surfaced by mem_save.

WHEN TO CALL: After mem_save returns judgment_required=true, iterate candidates[] and call mem_judge once per entry using that entry's judgment_id.

PARAMS:
  judgment_id (required) — from candidates[].judgment_id in the mem_save response
  relation    (required) — one of: related, compatible, scoped, conflicts_with, supersedes, not_conflict
  reason      (optional) — free-text explanation of the verdict
  evidence    (optional) — JSON or text supporting the verdict
  confidence  (optional) — float 0..1; default 1.0

WHEN TO ASK USER:
  - confidence < 0.7: surface to user before judging
  - relation in {supersedes, conflicts_with} AND type in {architecture, policy, decision}: ask user
  - otherwise: resolve silently

SUCCESS: Returns the updated relation row with judgment_status="judged".
ERROR: Returns IsError=true if judgment_id is unknown or relation verb is invalid.`,
	}
}

// MemJudgeHandler migrates from handleJudge in mcp.go.
func MemJudgeHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemJudgeInput, proj *project.Project) (*mcp2.CallToolResult, MemJudgeOutput, error) {
	out := MemJudgeOutput{}

	if input.JudgmentID == "" {
		out.Error = "judgment_id is required"
		return nil, out, nil
	}
	if input.Relation == "" {
		out.Error = "relation is required"
		return nil, out, nil
	}

	// Clamp confidence to [0, 1]
	if input.Confidence != nil {
		v := *input.Confidence
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		_ = &v // passed to store.JudgeRelation in full implementation
	}

	// TODO: Wire up actual store.JudgeRelation call

	out.Relation = &store.Relation{}

	return nil, out, nil
}

func add_tool_mem_judge(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_judge",
			mcp.WithDescription(`Record a verdict on a pending memory conflict surfaced by mem_save.

WHEN TO CALL: After mem_save returns judgment_required=true, iterate candidates[] and call mem_judge once per entry using that entry's judgment_id.

PARAMS:
  judgment_id (required) — from candidates[].judgment_id in the mem_save response
  relation    (required) — one of: related, compatible, scoped, conflicts_with, supersedes, not_conflict
  reason      (optional) — free-text explanation of the verdict
  evidence    (optional) — JSON or text supporting the verdict
  confidence  (optional) — float 0..1; default 1.0

WHEN TO ASK USER:
  - confidence < 0.7: surface to user before judging
  - relation in {supersedes, conflicts_with} AND type in {architecture, policy, decision}: ask user
  - otherwise: resolve silently

SUCCESS: Returns the updated relation row with judgment_status="judged".
ERROR: Returns IsError=true if judgment_id is unknown or relation verb is invalid. Row is NOT mutated on error.

Re-judging an already-judged ID overwrites the verdict (deliberate revision).`),
			mcp.WithTitleAnnotation("Judge Memory Conflict"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("judgment_id",
				mcp.Required(),
				mcp.Description("The judgment_id from candidates[] in the mem_save response (format: rel-<hex>)"),
			),
			mcp.WithString("relation",
				mcp.Required(),
				mcp.Description("Verdict: related | compatible | scoped | conflicts_with | supersedes | not_conflict"),
			),
			mcp.WithString("reason",
				mcp.Description("Free-text explanation of the verdict"),
			),
			mcp.WithString("evidence",
				mcp.Description("Supporting evidence (JSON or free text)"),
			),
			mcp.WithNumber("confidence",
				mcp.Description("Confidence score 0.0..1.0 (default: 1.0)"),
			),
			mcp.WithString("session_id",
				mcp.Description("Session ID for provenance (default: auto)"),
			),
		),
		queuedWriteHandler(getWriteQueue(), handleJudge(s, activity)),
	)
}

// handleJudge implements mem_judge. It validates params, calls JudgeRelation,
// and returns the updated relation row as JSON.
//
// Tool description contract (Design §6.1):
// "Record a verdict on a pending memory conflict surfaced by mem_save.
// When mem_save returns judgment_required=true, call mem_judge once per
// candidate (judgment_id is in candidates[]). Use to mark SUPERSEDES,
// CONFLICTS_WITH, NOT_CONFLICT, RELATED, COMPATIBLE, or SCOPED.
// Ask the user when ambiguous."
func handleJudge(s *store.Store, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		judgmentID, _ := req.GetArguments()["judgment_id"].(string)
		relation, _ := req.GetArguments()["relation"].(string)

		if judgmentID == "" {
			return mcp.NewToolResultError("judgment_id is required"), nil
		}
		if relation == "" {
			return mcp.NewToolResultError("relation is required"), nil
		}

		// Collect optional fields.
		var reason *string
		if v, ok := req.GetArguments()["reason"].(string); ok && v != "" {
			reason = &v
		}
		var evidence *string
		if v, ok := req.GetArguments()["evidence"].(string); ok && v != "" {
			evidence = &v
		}
		var confidence *float64
		if v, ok := req.GetArguments()["confidence"].(float64); ok {
			// Clamp to [0, 1] per design §6.3.
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			confidence = &v
		}

		// Session context for provenance.
		sessionID, _ := req.GetArguments()["session_id"].(string)
		// Actor defaults to "agent" kind for MCP tool calls.
		markedByActor := "agent"
		markedByKind := "agent"
		markedByModel := "" // No model ID available at MCP layer without explicit param.

		result, err := s.JudgeRelation(store.JudgeRelationParams{
			JudgmentID:    judgmentID,
			Relation:      relation,
			Reason:        reason,
			Evidence:      evidence,
			Confidence:    confidence,
			MarkedByActor: markedByActor,
			MarkedByKind:  markedByKind,
			MarkedByModel: markedByModel,
			SessionID:     sessionID,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		envelope := map[string]any{
			"relation": result,
		}
		out, _ := jsonMarshal(envelope)
		return mcp.NewToolResultText(string(out)), nil
	}
}
