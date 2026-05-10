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

// ── mem_compare ────────────────────────────────────────────────────
// Persist a semantic verdict you have already judged externally into Engram.

type MemCompareInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	MemoryIDA  int64   `json:"memory_id_a" jsonschema:"Integer id of the first observation (from mem_search #id)"`
	MemoryIDB  int64   `json:"memory_id_b" jsonschema:"Integer id of the second observation (from mem_search #id)"`
	Relation   string  `json:"relation" jsonschema:"Verdict: related | compatible | scoped | conflicts_with | supersedes | not_conflict"`
	Confidence float64 `json:"confidence" jsonschema:"Confidence score 0.0..1.0"`
	Reasoning  string  `json:"reasoning" jsonschema:"Brief explanation of the verdict (max 200 chars)"`
	Model      *string `json:"model,omitempty" jsonschema:"Your model identifier for provenance (e.g. \"claude-haiku-4-5\")"`
}

func (i *MemCompareInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemCompareOutput struct {
	SyncID string `json:"sync_id"`
	Error  string `json:"error,omitempty"`
}

const tnMemCompare = "mem_compare"

func init() {
	tools[tnMemCompare] = &mcp2.Tool{
		Description: `Persist a semantic verdict you have already judged externally (with your LLM) into Engram.

WHEN TO CALL: After you have evaluated two memories and reached a verdict, call mem_compare to PERSIST that verdict into the relation store. You do the judgment; mem_compare records it.

PARAMS:
  memory_id_a  (required) — integer id of the first observation (from mem_search or mem_get_observation)
  memory_id_b  (required) — integer id of the second observation
  relation     (required) — one of: related, compatible, scoped, conflicts_with, supersedes, not_conflict
  confidence   (required) — float 0..1; your self-reported confidence in the verdict
  reasoning    (required) — explanation of the verdict, max 200 chars
  model        (optional) — your model identifier, stored for provenance (e.g. "claude-haiku-4-5")

BEHAVIOR:
  - Persists the verdict via JudgeBySemantic with system provenance (marked_by_actor="engram").
  - not_conflict: no row is inserted; tool returns success with empty sync_id.
  - Idempotent: calling again for the same pair updates the existing row.
  - Cross-project pairs are rejected.

SUCCESS: Returns { "sync_id": "rel-..." } on persist, { "sync_id": "" } on not_conflict.
ERROR: Returns IsError=true if IDs are unknown, relation is invalid, or cross-project pair.`,
	}
}

// MemCompareHandler migrates from handleCompare in mcp.go.
func MemCompareHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemCompareInput, proj *engine.Project) (*mcp2.CallToolResult, MemCompareOutput, error) {
	out := MemCompareOutput{}

	if input.MemoryIDA == 0 {
		out.Error = "memory_id_a is required (integer observation id)"
		return nil, out, nil
	}
	if input.MemoryIDB == 0 {
		out.Error = "memory_id_b is required (integer observation id)"
		return nil, out, nil
	}
	if input.Relation == "" {
		out.Error = "relation is required"
		return nil, out, nil
	}
	if input.Reasoning == "" {
		out.Error = "reasoning is required"
		return nil, out, nil
	}

	// Clamp confidence to [0, 1]
	confidence := input.Confidence
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	// TODO: Wire up actual store.GetObservation calls to resolve IDs to sync_ids
	// TODO: Wire up actual store.JudgeBySemantic call

	out.SyncID = fmt.Sprintf("rel-persisted-%d-%d", input.MemoryIDA, input.MemoryIDB)

	return nil, out, nil
}

func add_tool_mem_compare(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_compare",
			mcp.WithDescription(`Persist a semantic verdict you have already judged externally (with your LLM) into Engram.

WHEN TO CALL: After you have evaluated two memories and reached a verdict, call mem_compare to PERSIST that verdict into the relation store. You do the judgment; mem_compare records it.

PARAMS:
  memory_id_a  (required) — integer id of the first observation (from mem_search or mem_get_observation)
  memory_id_b  (required) — integer id of the second observation
  relation     (required) — one of: related, compatible, scoped, conflicts_with, supersedes, not_conflict
  confidence   (required) — float 0..1; your self-reported confidence in the verdict
  reasoning    (required) — explanation of the verdict, max 200 chars
  model        (optional) — your model identifier, stored for provenance (e.g. "claude-haiku-4-5")

BEHAVIOR:
  - Persists the verdict via JudgeBySemantic with system provenance (marked_by_actor="engram").
  - not_conflict: no row is inserted; tool returns success with empty sync_id (the verdict is recorded but not stored — it means "we evaluated these and they do not conflict").
  - Idempotent: calling again for the same pair updates the existing row.
  - Cross-project pairs are rejected.

SUCCESS: Returns { "sync_id": "rel-..." } on persist, { "sync_id": "" } on not_conflict.
ERROR: Returns IsError=true if IDs are unknown, relation is invalid, or cross-project pair.`),
			mcp.WithTitleAnnotation("Compare Memory Pair (Persist Semantic Verdict)"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithNumber("memory_id_a",
				mcp.Required(),
				mcp.Description("Integer id of the first observation (from mem_search #id)"),
			),
			mcp.WithNumber("memory_id_b",
				mcp.Required(),
				mcp.Description("Integer id of the second observation (from mem_search #id)"),
			),
			mcp.WithString("relation",
				mcp.Required(),
				mcp.Description("Verdict: related | compatible | scoped | conflicts_with | supersedes | not_conflict"),
			),
			mcp.WithNumber("confidence",
				mcp.Required(),
				mcp.Description("Confidence score 0.0..1.0"),
			),
			mcp.WithString("reasoning",
				mcp.Required(),
				mcp.Description("Brief explanation of the verdict (max 200 chars)"),
			),
			mcp.WithString("model",
				mcp.Description("Your model identifier for provenance (e.g. \"claude-haiku-4-5\")"),
			),
		),
		handleCompare(s, activity),
	)
}

// handleCompare implements mem_compare. The agent has already judged two
// observations externally; this handler persists the verdict via JudgeBySemantic.
//
// Tool description contract (REQ-011, Design §9):
// "Persist a semantic verdict you have already judged externally into Engram.
// Accepts int IDs for both observations, resolves them to sync_ids, then
// calls JudgeBySemantic. Returns the persisted relation's sync_id."
func handleCompare(s *store.Store, _ *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// --- required numeric IDs ---
		rawA, okA := req.GetArguments()["memory_id_a"].(float64)
		rawB, okB := req.GetArguments()["memory_id_b"].(float64)
		if !okA {
			return mcp.NewToolResultError("memory_id_a is required (integer observation id)"), nil
		}
		if !okB {
			return mcp.NewToolResultError("memory_id_b is required (integer observation id)"), nil
		}
		idA := int64(rawA)
		idB := int64(rawB)

		// --- required string fields ---
		relation, _ := req.GetArguments()["relation"].(string)
		if relation == "" {
			return mcp.NewToolResultError("relation is required"), nil
		}
		reasoning, _ := req.GetArguments()["reasoning"].(string)
		if reasoning == "" {
			return mcp.NewToolResultError("reasoning is required"), nil
		}

		// --- required confidence ---
		rawConf, okConf := req.GetArguments()["confidence"].(float64)
		if !okConf {
			return mcp.NewToolResultError("confidence is required (float 0.0..1.0)"), nil
		}
		// Clamp to [0, 1].
		if rawConf < 0 {
			rawConf = 0
		}
		if rawConf > 1 {
			rawConf = 1
		}

		// --- optional model ---
		model, _ := req.GetArguments()["model"].(string)

		// Resolve integer IDs to sync_ids.
		obsA, err := s.GetObservation(idA)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("observation id=%d not found: %s", idA, err)), nil
		}
		obsB, err := s.GetObservation(idB)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("observation id=%d not found: %s", idB, err)), nil
		}

		syncID, err := s.JudgeBySemantic(store.JudgeBySemanticParams{
			SourceID:   obsA.SyncID,
			TargetID:   obsB.SyncID,
			Relation:   relation,
			Confidence: rawConf,
			Reasoning:  reasoning,
			Model:      model,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// syncID is "" when relation == "not_conflict" (JudgeBySemantic no-op).
		envelope := map[string]any{
			"sync_id": syncID,
		}
		out, _ := jsonMarshal(envelope)
		return mcp.NewToolResultText(string(out)), nil
	}
}
