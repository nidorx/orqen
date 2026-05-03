package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

// ── orqen_create_item ──────────────────────────────────────────────
// Creates a new work item in a specific lane of a module.
// Creates the directory and an empty .md file following the naming convention.

type CreateItemInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"job id (auto-injected)"`
	Module     *string `json:"module" jsonschema:"module name (e.g., task, adr, learning)"`
	Lane       string  `json:"lane" jsonschema:"destination lane name"`
	SimpleName string  `json:"simple_name" jsonschema:"kebab-case descriptive name for the item"`
}

func (i *CreateItemInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type CreateItemOutput struct {
	Success    bool   `json:"success"`
	ItemSeq    int    `json:"item_seq"`
	ItemName   string `json:"item_name"`
	DirPath    string `json:"dir_path"`
	FilePath   string `json:"file_path"`
	ModuleType string `json:"module_type"`
	Error      string `json:"error,omitempty"`
}

// kebabCasePattern validates kebab-case names (lowercase, numbers, hyphens).
var kebabCasePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

const tnCreateItem = "orqen_create_item"

func init() {
	tools[tnCreateItem] = &mcp.Tool{
		Description: "Creates a new work item in a specific lane of a module. Creates the directory following naming conventions (MOD_TYPE-NNNN-name) and an empty .md file.",
	}
}

func CreateItemHandler(ctx context.Context, req *mcp.CallToolRequest, input *CreateItemInput, proj *project.Project) (*mcp.CallToolResult, CreateItemOutput, error) {
	out := CreateItemOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Lane == "" {
		out.Error = "lane name is required"
		return nil, out, nil
	}

	if input.SimpleName == "" {
		out.Error = "simple_name is required"
		return nil, out, nil
	}

	// Validate kebab-case
	simpleName := strings.ToLower(strings.TrimSpace(input.SimpleName))
	if !kebabCasePattern.MatchString(simpleName) {
		out.Error = fmt.Sprintf("simple_name must be kebab-case (lowercase letters, numbers, hyphens): %q", input.SimpleName)
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
		// Try to resolve current module from WorkItemID
		if input.WorkItemID != nil && *input.WorkItemID != "" {
			targetModule = findModuleByWorkItemID(proj, *input.WorkItemID)
		}
		if targetModule == nil && len(proj.Modules) == 1 {
			targetModule = proj.Modules[0]
		}
	}

	if targetModule == nil {
		out.Error = "could not resolve target module — specify module parameter or ensure workitem_id is set"
		return nil, out, nil
	}

	out.ModuleType = strings.ToUpper(targetModule.Name)

	lane := targetModule.GetLane(input.Lane)
	if lane == nil {
		out.Error = fmt.Sprintf(
			"lane %q not found in module %s (available: %s)",
			input.Lane, *input.Module, laneNames(targetModule),
		)
		return nil, out, nil
	}

	// Get next sequence number
	nextSeq := targetModule.NextSequence()

	// Build names
	dirName := fmt.Sprintf("%s-%04d-%s", out.ModuleType, nextSeq, simpleName)
	fileName := fmt.Sprintf("%s-%04d.md", out.ModuleType, nextSeq)

	// Create directory
	dirPath := filepath.Join(lane.DirAbs, dirName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		out.Error = fmt.Sprintf("failed to create directory: %v", err)
		return nil, out, nil
	}

	// Create empty .md file
	// @TODO: Remover
	filePath := filepath.Join(dirPath, fileName)
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		out.Error = fmt.Sprintf("failed to create file: %v", err)
		return nil, out, nil
	}

	out.Success = true
	// @TODO: item id
	out.ItemSeq = nextSeq
	out.ItemName = dirName

	// Relative paths
	relDir, _ := filepath.Rel(proj.DirAbs, filepath.Clean(dirPath))
	relFile, _ := filepath.Rel(proj.DirAbs, filepath.Clean(filePath))
	out.DirPath = filepath.ToSlash(relDir)
	out.FilePath = filepath.ToSlash(relFile)

	return nil, out, nil
}

func laneNames(mod *project.Module) string {
	var names []string
	for _, l := range mod.Lanes {
		names = append(names, l.Name)
	}
	return strings.Join(names, ", ")
}
