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

// ── mem_delete ─────────────────────────────────────────────────────
// Delete an observation by ID.

type MemDeleteInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	ID         int64   `json:"id" jsonschema:"Observation ID to delete"`
	HardDelete *bool   `json:"hard_delete,omitempty" jsonschema:"If true, permanently deletes the observation"`
}

func (i *MemDeleteInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemDeleteOutput struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

const tnMemDelete = "mem_delete"

func init() {
	tools[tnMemDelete] = &mcp2.Tool{
		Description: "Delete an observation by ID. Soft-delete by default; set hard_delete=true for permanent deletion.",
		Title:       "Delete Memory",
		Annotations: &mcp2.ToolAnnotations{
			Title:           "Delete Memory",
			ReadOnlyHint:    false,
			DestructiveHint: ptrBool(true),
			IdempotentHint:  false,
			OpenWorldHint:   ptrBool(false),
		},
	}
}

// MemDeleteHandler migrates from handleDelete in mcp.go.
func MemDeleteHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemDeleteInput, proj *engine.Project) (*mcp2.CallToolResult, MemDeleteOutput, error) {
	// out := MemDeleteOutput{}
	// s := getStore()

	// if input.ID == 0 {
	// 	return mcp2.NewTextResult("id is required"), out, nil
	// }

	// hardDelete := false
	// if input.HardDelete != nil {
	// 	hardDelete = *input.HardDelete
	// }

	// if err := s.DeleteObservation(input.ID, hardDelete); err != nil {
	// 	return mcp2.NewTextResult("Failed to delete memory: " + err.Error()), out, nil
	// }

	// mode := "soft-deleted"
	// if hardDelete {
	// 	mode = "permanently deleted"
	// }
	// msg := fmt.Sprintf("Memory #%d %s", input.ID, mode)
	// out.Message = msg
	// return mcp2.NewTextResult(msg), out, nil
	return nil, MemDeleteOutput{}, nil
}

func add_tool_mem_delete(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_delete",
			mcp.WithDescription("Delete an observation by ID. Soft-delete by default; set hard_delete=true for permanent deletion."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Delete Memory"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithNumber("id",
				mcp.Required(),
				mcp.Description("Observation ID to delete"),
			),
			mcp.WithBoolean("hard_delete",
				mcp.Description("If true, permanently deletes the observation"),
			),
		),
		QueuedWriteHandler(getWriteQueue(), handleDelete(s)),
	)
}

func handleDelete(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(intArg(req, "id", 0))
		if id == 0 {
			return mcp.NewToolResultError("id is required"), nil
		}

		hardDelete := boolArg(req, "hard_delete", false)
		if err := s.DeleteObservation(id, hardDelete); err != nil {
			return mcp.NewToolResultError("Failed to delete memory: " + err.Error()), nil
		}

		mode := "soft-deleted"
		if hardDelete {
			mode = "permanently deleted"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Memory #%d %s", id, mode)), nil
	}
}
