package mcp

import (
	"context"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsListInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Dir        string  `json:"dir" jsonschema:"directory path to list"`
}

func (i *FsListInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type FsEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
	Size int64  `json:"size,omitempty"`
}

type FsListOutput struct {
	Entries []FsEntry `json:"entries,omitempty"`
	Error   string    `json:"error,omitempty"`
}

const tnFsList = "orqen_fs_list"

func init() {
	tools[tnFsList] = &mcp.Tool{
		Description: "List directory contents. Excludes .orqen/ and .git/ paths.",
	}
}

func FsListHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsListInput, proj *engine.Project) (*mcp.CallToolResult, FsListOutput, error) {
	out := FsListOutput{}

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

	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		out.Error = "failed to read directory: " + err.Error()
		return nil, out, nil
	}

	for _, entry := range entries {
		// Skip blocked paths
		if paths.IsBlockedPath(entry.Name()) {
			continue
		}

		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}

		e := FsEntry{
			Name: entry.Name(),
			Type: "file",
		}

		if entry.IsDir() {
			e.Type = "dir"
		} else {
			e.Size = info.Size()
		}

		out.Entries = append(out.Entries, e)
	}

	return nil, out, nil
}