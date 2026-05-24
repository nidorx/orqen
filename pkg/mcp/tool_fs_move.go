package mcp

import (
	"context"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsMoveInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Src        string  `json:"src" jsonschema:"source file or directory path"`
	Dst        string  `json:"dst" jsonschema:"destination file or directory path"`
}

func (i *FsMoveInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type FsMoveOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const tnFsMove = "orqen_fs_move"

func init() {
	tools[tnFsMove] = &mcp.Tool{
		Description: "Move file or directory from source to destination. Handles cross-device moves with copy+delete fallback.",
	}
}

func FsMoveHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsMoveInput, proj *engine.Project) (*mcp.CallToolResult, FsMoveOutput, error) {
	out := FsMoveOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Src == "" {
		out.Error = "source path is required"
		return nil, out, nil
	}
	if input.Dst == "" {
		out.Error = "destination path is required"
		return nil, out, nil
	}

	srcAbs, err := paths.SafeFilePath(proj, input.Src)
	if err != nil {
		out.Error = "invalid source path: " + err.Error()
		return nil, out, nil
	}

	dstAbs, err := paths.SafeFilePath(proj, input.Dst)
	if err != nil {
		out.Error = "invalid destination path: " + err.Error()
		return nil, out, nil
	}

	if _, err := os.Stat(srcAbs); os.IsNotExist(err) {
		out.Error = "source does not exist: " + input.Src
		return nil, out, nil
	}

	err = os.Rename(srcAbs, dstAbs)
	if err != nil {
		// Cross-device fallback: copy then delete
		srcInfo, statErr := os.Stat(srcAbs)
		if statErr != nil {
			out.Error = "failed to stat source: " + statErr.Error()
			return nil, out, nil
		}

		if srcInfo.IsDir() {
			err = copyDir(srcAbs, dstAbs)
		} else {
			err = copyFile(srcAbs, dstAbs)
		}

		if err != nil {
			out.Error = "failed to move (copy fallback failed): " + err.Error()
			return nil, out, nil
		}

		if rmErr := os.RemoveAll(srcAbs); rmErr != nil {
			out.Error = "copied but failed to remove source: " + rmErr.Error()
			return nil, out, nil
		}
	}

	out.Success = true
	return nil, out, nil
}
