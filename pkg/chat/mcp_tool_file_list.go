package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatFileListInput struct {
	Path      string `json:"path,omitempty" jsonschema:"Relative path to list (default: project root)"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"Whether to list recursively"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum entries to return (default: 50)"`
}

type ChatFileListOutput struct {
	Entries []FileEntry `json:"entries"`
	Error   string      `json:"error,omitempty"`
}

const tnChatFileList = "chat_file_list"

func init() {
	chatTools[tnChatFileList] = &mcp.Tool{
		Description: "List files in the project directory, optionally filtered by path.",
	}
}

func ChatFileListHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatFileListInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatFileListOutput, error) {
	out := ChatFileListOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	searchPath := proj.DirAbs
	if input.Path != "" {
		searchPath = filepath.Join(proj.DirAbs, input.Path)
	}

	count := 0
	err := filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		rel, relErr := filepath.Rel(proj.DirAbs, path)
		if relErr != nil {
			rel = path
		}

		// Skip blocked paths
		if isBlockedPath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, statErr := d.Info()
		size := int64(0)
		if statErr == nil {
			size = info.Size()
		}

		entryType := "file"
		if d.IsDir() {
			entryType = "directory"
		}

		out.Entries = append(out.Entries, FileEntry{
			Path: filepath.ToSlash(rel),
			Type: entryType,
			Size: size,
		})
		count++

		if count >= limit {
			return filepath.SkipDir
		}

		if !input.Recursive && d.IsDir() && rel != "." {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		out.Error = fmt.Sprintf("failed to list files: %v", err)
		return nil, out, nil
	}

	// Sort entries for consistent output
	sort.Slice(out.Entries, func(i, j int) bool {
		if out.Entries[i].Type != out.Entries[j].Type {
			return out.Entries[i].Type < out.Entries[j].Type
		}
		return out.Entries[i].Path < out.Entries[j].Path
	})

	return nil, out, nil
}
