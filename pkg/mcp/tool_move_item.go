package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_move_item ────────────────────────────────────────────────
// Moves a work item directory from one lane to another within a module.
// Updates internal state to reflect the new lane position.

type MoveItemInput struct {
	JobId    *string `json:"job_id,omitempty" jsonschema:"job id (auto-injected)"`
	Module   *string `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
	ItemID   int     `json:"item_id" jsonschema:"numeric ID of the work item to move"`
	FromLane string  `json:"from_lane" jsonschema:"current lane name"`
	ToLane   string  `json:"to_lane" jsonschema:"destination lane name"`
}

func (i *MoveItemInput) SetJobId(jobId string) {
	i.JobId = &jobId
}

type MoveItemOutput struct {
	Success bool   `json:"success"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tnMoveItem = "orqen_move_item"

func init() {
	tools[tnMoveItem] = &mcp.Tool{
		Description: "Moves a work item directory from one lane to another within a module. Updates internal state to reflect the new lane position.",
	}
}

func MoveItemHandler(ctx context.Context, req *mcp.CallToolRequest, input *MoveItemInput, proj *project.Project) (*mcp.CallToolResult, MoveItemOutput, error) {
	out := MoveItemOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.ToLane == "" {
		out.Error = "to_lane is required"
		return nil, out, nil
	}

	var targetModule *project.Module
	if input.Module != nil && *input.Module != "" {
		targetModule = proj.GetModule(*input.Module)
		if targetModule == nil {
			out.Error = fmt.Sprintf("module not found: %s", *input.Module)
			return nil, out, nil
		}
	} else {
		// Try to resolve current module from JobId
		if input.JobId != nil && *input.JobId != "" {
			targetModule = findModuleByJobID(proj, *input.JobId)
		}
		if targetModule == nil && len(proj.Modules) == 1 {
			targetModule = proj.Modules[0]
		}
	}

	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter or ensure job_id is set"
		return nil, out, nil
	}

	// Resolve from lane
	fromLane := targetModule.GetLane(input.FromLane)
	if fromLane == nil {
		out.Error = "from_lane not found: " + input.FromLane
		return nil, out, nil
	}

	// Resolve to lane
	toLane := targetModule.GetLane(input.ToLane)
	if toLane == nil {
		out.Error = "to_lane not found: " + input.ToLane
		return nil, out, nil
	}

	// Find the work item
	var item *project.WorkItem
	if input.ItemID > 0 {
		item = targetModule.FindItemByID(input.ItemID)
	}

	// Fallback: try to find by JobId
	if item == nil && input.JobId != nil && *input.JobId != "" {
		for _, lane := range targetModule.Lanes {
			for _, it := range lane.ListItems() {
				if it.JobID == *input.JobId {
					item = it
					break
				}
			}
			if item != nil {
				break
			}
		}
	}

	if item == nil {
		out.Error = fmt.Sprintf("work item not found (id=%d)", input.ItemID)
		return nil, out, nil
	}

	// Build source and destination paths
	srcDir := filepath.Join(fromLane.DirAbs, item.Name)
	dstDir := filepath.Join(toLane.DirAbs, item.Name)

	// Verify source exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		out.Error = fmt.Sprintf("source directory not found: %s", srcDir)
		return nil, out, nil
	}

	// Ensure destination lane directory exists
	if err := os.MkdirAll(toLane.DirAbs, 0755); err != nil {
		out.Error = fmt.Sprintf("failed to create destination lane directory: %v", err)
		return nil, out, nil
	}

	// Move the directory
	if err := os.Rename(srcDir, dstDir); err != nil {
		out.Error = fmt.Sprintf("failed to move item directory: %v", err)
		return nil, out, nil
	}

	// Update internal state: refresh lane caches
	// Re-scan both lanes to update their item caches
	fromLane.ListItems()
	toLane.ListItems()

	out.Success = true
	out.From = fromLane.Dir
	out.To = toLane.Dir

	return nil, out, nil
}
