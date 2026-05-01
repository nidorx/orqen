package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
	"github.com/nidorx/orqen/pkg/storage"
)

// ── orqen_scan_module ──────────────────────────────────────────────
// Scans all markdown files in a module directory, parses front matter,
// and returns files matching the given filter criteria.

type ScanModuleInput struct {
	JobId   *string        `json:"job_id,omitempty" jsonschema:"job id (auto-injected)"`
	Module  *string        `json:"module,omitempty" jsonschema:"module name (omit for current module)"`
	Filters map[string]any `json:"filters,omitempty" jsonschema:"front matter filters with operators (eq, ne, contains, exists, in, prefix, suffix)"`
}

func (i *ScanModuleInput) SetJobId(jobId string) {
	i.JobId = &jobId
}

type ScannedFile struct {
	Path        string         `json:"path"`
	Name        string         `json:"name"`
	FrontMatter map[string]any `json:"front_matter"`
	Body        string         `json:"body"`
}

type ScanModuleOutput struct {
	Module string        `json:"module"`
	Dir    string        `json:"dir"`
	Count  int           `json:"count"`
	Files  []ScannedFile `json:"files"`
	Error  string        `json:"error,omitempty"`
}

const tnScanModule = "orqen_scan_module"

func init() {
	tools[tnScanModule] = &mcp.Tool{
		Description: "Scans all markdown files in a module directory, parses front matter YAML, and returns files matching filter criteria. Supports operators: eq, ne, contains, exists, in, prefix, suffix.",
	}
}

func ScanModuleHandler(ctx context.Context, req *mcp.CallToolRequest, input *ScanModuleInput, proj *project.Project) (*mcp.CallToolResult, ScanModuleOutput, error) {
	out := ScanModuleOutput{}

	if proj == nil {
		out.Error = "project not available"
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

	out.Module = targetModule.Name
	out.Dir = targetModule.DirAbs

	// Scan on-demand, no caching
	matched, err := storage.ScanModule(targetModule.DirAbs, proj.DirAbs, input.Filters)
	if err != nil {
		out.Error = fmt.Sprintf("scan error: %v", err)
		return nil, out, nil
	}

	out.Count = len(matched)
	for _, f := range matched {
		out.Files = append(out.Files, ScannedFile{
			Path:        f.Path,
			Name:        f.Name,
			FrontMatter: f.FrontMatter,
			Body:        f.Body,
		})
	}

	return nil, out, nil
}
