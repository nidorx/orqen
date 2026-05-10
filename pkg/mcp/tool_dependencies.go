package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	project "github.com/nidorx/orqen/pkg/engine"
)

// ── orqen_dependencies ─────────────────────────────────────────────
// Checks dependency status for the current work item.
// Scans for DEP_XXX files and resolves them to actual work items.

type DependenciesInput struct {
	WorkItemID *string `json:"workitem_id" jsonschema:"Work Item ID (auto-injected)"`
}

func (i *DependenciesInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type DependencyInfo struct {
	DepID    int    `json:"dep_id"`
	ItemName string `json:"item_name,omitempty"`
	Lane     string `json:"lane,omitempty"`
	Module   string `json:"module,omitempty"`
	Status   string `json:"status"` // "resolved", "in_progress", "missing"
	Error    string `json:"error,omitempty"`
}

type DependenciesOutput struct {
	ItemID       string           `json:"item_id"`
	ItemSeq      int              `json:"item_seq"`
	ItemName     string           `json:"item_name"`
	Dependencies []DependencyInfo `json:"dependencies"`
	Error        string           `json:"error,omitempty"`
}

const tnDependencies = "orqen_dependencies"

func init() {
	tools[tnDependencies] = &mcp.Tool{
		Description: "Checks dependency status for the current work item. Scans for DEP_XXX files and resolves them to actual work items with their status.",
	}
}

func DependenciesHandler(ctx context.Context, req *mcp.CallToolRequest, input *DependenciesInput, proj *engine.Project) (*mcp.CallToolResult, DependenciesOutput, error) {
	out := DependenciesOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.WorkItemID == nil || *input.WorkItemID == "" {
		out.Error = "no job id provided"
		return nil, out, nil
	}

	workItemID := *input.WorkItemID

	// Find the current work item by WorkItemID
	var currentItem *engine.WorkItem
	var currentLane *project.Lane
	var currentMod *engine.Module

	for _, mod := range proj.Modules {
		if item := mod.GetWorkItemById(workItemID); item != nil {
			currentItem = item
			currentLane = item.Lane
			currentMod = mod
			break
		}
	}

	if currentItem == nil || currentLane == nil {
		out.Error = fmt.Sprintf("work item with job id %q not found", workItemID)
		return nil, out, nil
	}

	out.ItemID = currentItem.ID
	out.ItemSeq = currentItem.Seq
	out.ItemName = currentItem.Name

	// Build item directory path
	// @TODO: Mudar lógica para usar metadaodos
	itemDir := filepath.Join(currentLane.DirAbs, currentItem.Name)
	entries, err := os.ReadDir(itemDir)
	if err != nil {
		out.Error = fmt.Sprintf("cannot read item directory: %v", err)
		return nil, out, nil
	}

	// Scan for DEP_XXX files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "DEP_") {
			continue
		}

		depID := extractDepID(entry.Name())
		if depID <= 0 {
			continue
		}

		depInfo := DependencyInfo{DepID: depID}

		// Try to find the dependency item across all lanes in the module
		depItem := currentMod.GetWorkItemBySeq(depID)
		if depItem != nil {
			depInfo.ItemName = depItem.Name
			depInfo.Lane = depItem.Lane.Name
			depInfo.Module = currentMod.Name

			// Determine status based on lane
			switch depItem.Lane.Name {
			case "done", "archived":
				depInfo.Status = "resolved"
			case "doing":
				depInfo.Status = "in_progress"
			default:
				depInfo.Status = "pending"
			}
		} else {
			depInfo.Status = "missing"
			depInfo.Error = fmt.Sprintf("no work item found with ID %d", depID)
		}

		out.Dependencies = append(out.Dependencies, depInfo)
	}

	return nil, out, nil
}

func extractDepID(name string) int {
	trimmed := strings.TrimPrefix(name, "DEP_")
	// Remove extension if present
	if idx := strings.LastIndex(trimmed, "."); idx > 0 {
		trimmed = trimmed[:idx]
	}
	id, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0
	}
	return id
}
