package mcp

import (
	"context"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsFindInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Pattern    string  `json:"pattern" jsonschema:"glob pattern to match (e.g., *.go)"`
	Dir        string  `json:"dir,omitempty" jsonschema:"directory to search in (defaults to project root)"`
	MaxResults *int    `json:"max_results,omitempty" jsonschema:"maximum number of results (default: 50)"`
	MaxDepth   *int    `json:"max_depth,omitempty" jsonschema:"maximum depth to traverse"`
	FileType   *string `json:"file_type,omitempty" jsonschema:"filter by type: 'f' for files, 'd' for directories"`
}

func (i *FsFindInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type FsFindOutput struct {
	Matches []string `json:"matches,omitempty"`
	Count   int      `json:"count"`
	Error   string   `json:"error,omitempty"`
}

const tnFsFind = "orqen_fs_find"

func init() {
	tools[tnFsFind] = &mcp.Tool{
		Description: "Find files/directories matching glob pattern. Supports -name, -type, -maxdepth style options.",
	}
}

func FsFindHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsFindInput, proj *engine.Project) (*mcp.CallToolResult, FsFindOutput, error) {
	out := FsFindOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Pattern == "" {
		out.Error = "pattern is required"
		return nil, out, nil
	}

	searchDir := proj.DirAbs
	if input.Dir != "" {
		var err error
		searchDir, err = paths.SafeFilePath(proj, input.Dir)
		if err != nil {
			out.Error = "invalid directory path: " + err.Error()
			return nil, out, nil
		}
	}

	maxResults := 50
	if input.MaxResults != nil {
		maxResults = *input.MaxResults
	}

	fileType := "f"
	if input.FileType != nil {
		fileType = *input.FileType
	}

	var matches []string

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors and continue
		}

		// Check blocked paths
		relPath, relErr := filepath.Rel(searchDir, path)
		if relErr == nil && paths.IsBlockedPath(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by type
		wantDirs := fileType == "d"
		if wantDirs && !d.IsDir() {
			return nil
		}
		if !wantDirs && d.IsDir() {
			return nil
		}

		// Get filename for glob matching
		name := d.Name()
		matched, matchErr := filepath.Match(input.Pattern, name)
		if matchErr != nil {
			return nil
		}

		if matched {
			relPath, relErr := filepath.Rel(searchDir, path)
			if relErr != nil {
				relPath = path
			}
			if relPath != "" {
				matches = append(matches, relPath)
			}
		}

		return nil
	})

	if err != nil {
		out.Error = "failed to walk directory: " + err.Error()
		return nil, out, nil
	}

	// Apply max results
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	out.Matches = matches
	out.Count = len(matches)
	return nil, out, nil
}
