package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type ChatFileReadInput struct {
	Path  string `json:"path" jsonschema:"File path relative to project root"`
	Line  int    `json:"line,omitempty" jsonschema:"Start line (1-indexed, default: 1)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Number of lines to read (default: 100)"`
}

type ChatFileReadOutput struct {
	Content string `json:"content"`
	Lines   int    `json:"lines_read"`
	Total   int    `json:"total_lines,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tnChatFileRead = "chat_file_read"

const maxFileReadSize = 50 * 1024 // 50KB

func init() {
	chatTools[tnChatFileRead] = &mcp.Tool{
		Description: "Read a file's content from the project directory. Large files are truncated.",
	}
}

func ChatFileReadHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatFileReadInput,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (*mcp.CallToolResult, ChatFileReadOutput, error) {
	out := ChatFileReadOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Path == "" {
		out.Error = "path is required"
		return nil, out, nil
	}

	abs, err := paths.SafeFilePath(proj, input.Path)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}

	// Read with size limit
	info, statErr := os.Stat(abs)
	if statErr != nil {
		out.Error = fmt.Sprintf("cannot stat file: %v", statErr)
		return nil, out, nil
	}

	readLimit := int64(maxFileReadSize)
	if info.Size() > readLimit {
		// Read only up to the limit
		data := make([]byte, readLimit)
		f, openErr := os.Open(abs)
		if openErr != nil {
			out.Error = fmt.Sprintf("cannot open file: %v", openErr)
			return nil, out, nil
		}
		n, readErr := f.Read(data)
		f.Close()
		if readErr != nil && readErr.Error() != "EOF" {
			// Read may return nil or EOF at end
		}
		out.Content = string(data[:n]) + "\n...[truncated, file is large]..."
	} else {
		data, readErr := os.ReadFile(abs)
		if readErr != nil {
			out.Error = fmt.Sprintf("cannot read file: %v", readErr)
			return nil, out, nil
		}
		out.Content = string(data)
	}

	// Apply line pagination
	allLines := strings.Split(out.Content, "\n")
	out.Total = len(allLines)

	startLine := input.Line
	if startLine <= 0 {
		startLine = 1
	}
	lineLimit := input.Limit
	if lineLimit <= 0 {
		lineLimit = 100
	}

	startIdx := startLine - 1
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(allLines) {
		out.Content = ""
		out.Lines = 0
		return nil, out, nil
	}

	endIdx := startIdx + lineLimit
	if endIdx > len(allLines) {
		endIdx = len(allLines)
	}

	out.Content = strings.Join(allLines[startIdx:endIdx], "\n")
	out.Lines = endIdx - startIdx

	return nil, out, nil
}
