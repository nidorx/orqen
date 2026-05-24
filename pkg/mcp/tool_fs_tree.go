package mcp

import (
	"context"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsTreeInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Dir        string  `json:"dir" jsonschema:"directory path to tree"`
	MaxDepth   *int    `json:"max_depth,omitempty" jsonschema:"maximum depth to traverse (default: 3)"`
}

func (i *FsTreeInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type FsTreeOutput struct {
	Tree  []string `json:"tree,omitempty"`
	Error string   `json:"error,omitempty"`
}

const tnFsTree = "orqen_fs_tree"

func init() {
	tools[tnFsTree] = &mcp.Tool{
		Description: "Display directory tree structure with indentation. Default max depth is 3.",
	}
}

func FsTreeHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsTreeInput, proj *engine.Project) (*mcp.CallToolResult, FsTreeOutput, error) {
	out := FsTreeOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Dir == "" {
		out.Error = "dir is required"
		return nil, out, nil
	}

	dirAbs, err := paths.SafeFilePath(proj, input.Dir)
	if err != nil {
		out.Error = "invalid directory path: " + err.Error()
		return nil, out, nil
	}

	info, err := os.Stat(dirAbs)
	if err != nil {
		if os.IsNotExist(err) {
			out.Error = "directory does not exist: " + input.Dir
			return nil, out, nil
		}
		out.Error = "failed to stat directory: " + err.Error()
		return nil, out, nil
	}

	if !info.IsDir() {
		out.Error = "path is not a directory: " + input.Dir
		return nil, out, nil
	}

	maxDepth := 3
	if input.MaxDepth != nil {
		maxDepth = *input.MaxDepth
	}

	var lines []string
	lines = append(lines, filepath.Base(dirAbs)+"/")

	err = walkTree(dirAbs, "", true, 0, maxDepth, &lines)
	if err != nil {
		out.Error = "failed to walk tree: " + err.Error()
		return nil, out, nil
	}

	out.Tree = lines
	return nil, out, nil
}

func walkTree(dir, prefix string, isLast bool, depth, maxDepth int, lines *[]string) error {
	if depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Filter blocked paths
	var filtered []os.DirEntry
	for _, entry := range entries {
		if paths.IsBlockedPath(entry.Name()) {
			continue
		}
		filtered = append(filtered, entry)
	}

	for i, entry := range filtered {
		isLastEntry := i == len(filtered)-1

		connector := "├── "
		if isLastEntry {
			connector = "└── "
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}

		*lines = append(*lines, prefix+connector+name)

		if entry.IsDir() {
			newPrefix := prefix
			if isLastEntry {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}

			fullPath := filepath.Join(dir, entry.Name())
			if err := walkTree(fullPath, newPrefix, isLastEntry, depth+1, maxDepth, lines); err != nil {
				return err
			}
		}
	}

	return nil
}