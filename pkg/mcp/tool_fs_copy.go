package mcp

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsCopyInput struct {
	Src string `json:"src" jsonschema:"source file or directory path"`
	Dst string `json:"dst" jsonschema:"destination file or directory path"`
}

type FsCopyOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

const tnFsCopy = "fs_copy"

func init() {
	tools[tnFsCopy] = &mcp.Tool{
		Description: "Copy file or directory from source to destination.",
	}
}

func FsCopyHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsCopyInput, proj *engine.Project) (*mcp.CallToolResult, FsCopyOutput, error) {
	out := FsCopyOutput{}

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

	srcInfo, err := os.Stat(srcAbs)
	if err != nil {
		if os.IsNotExist(err) {
			out.Error = "source does not exist: " + input.Src
			return nil, out, nil
		}
		out.Error = "failed to stat source: " + err.Error()
		return nil, out, nil
	}

	if srcInfo.IsDir() {
		err = copyDir(srcAbs, dstAbs)
	} else {
		err = copyFile(srcAbs, dstAbs)
	}

	if err != nil {
		out.Error = "failed to copy: " + err.Error()
		return nil, out, nil
	}

	out.Success = true
	return nil, out, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
