package mcp

import (
	"context"
	"io"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsCatInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Filepath   string  `json:"filepath" jsonschema:"path to the file to read"`
}

func (i *FsCatInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type FsCatOutput struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tnFsCat = "orqen_fs_cat"

const maxCatSize = 100 * 1024 // 100KB

func init() {
	tools[tnFsCat] = &mcp.Tool{
		Description: "Display file contents. Limited to 100KB to prevent excessive output.",
	}
}

func FsCatHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsCatInput, proj *engine.Project) (*mcp.CallToolResult, FsCatOutput, error) {
	out := FsCatOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Filepath == "" {
		out.Error = "filepath is required"
		return nil, out, nil
	}

	fileAbs, err := paths.SafeFilePath(proj, input.Filepath)
	if err != nil {
		out.Error = "invalid file path: " + err.Error()
		return nil, out, nil
	}

	info, err := os.Stat(fileAbs)
	if err != nil {
		if os.IsNotExist(err) {
			out.Error = "file does not exist: " + input.Filepath
			return nil, out, nil
		}
		out.Error = "failed to stat file: " + err.Error()
		return nil, out, nil
	}

	if info.IsDir() {
		out.Error = "path is a directory, not a file: " + input.Filepath
		return nil, out, nil
	}

	if info.Size() > maxCatSize {
		f, openErr := os.Open(fileAbs)
		if openErr != nil {
			out.Error = "failed to open file: " + openErr.Error()
			return nil, out, nil
		}
		defer f.Close()

		buf := make([]byte, maxCatSize)
		n, readErr := io.ReadFull(f, buf)
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			out.Error = "failed to read file: " + readErr.Error()
			return nil, out, nil
		}
		out.Content = string(buf[:n]) + "\n... [truncated, file larger than 100KB]"
		return nil, out, nil
	}

	data, err := os.ReadFile(fileAbs)
	if err != nil {
		out.Error = "failed to read file: " + err.Error()
		return nil, out, nil
	}

	out.Content = string(data)
	return nil, out, nil
}
