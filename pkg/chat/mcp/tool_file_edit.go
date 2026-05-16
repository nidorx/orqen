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

type ChatFileEditInput struct {
	Path      string `json:"path" jsonschema:"File path relative to project root"`
	Content   string `json:"content" jsonschema:"Full proposed file content"`
	Reason    string `json:"reason,omitempty" jsonschema:"Reason for the edit"`
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for the pending edit"`
}

type ChatFileEditOutput struct {
	EditID   int64  `json:"edit_id"`
	FilePath string `json:"file_path"`
	Diff     string `json:"diff_preview"`
	Error    string `json:"error,omitempty"`
}

const tnChatFileEdit = "chat_file_edit"

func init() {
	chatTools[tnChatFileEdit] = &mcp.Tool{
		Description: "Propose a file edit. Does NOT apply the edit - creates a pending edit requiring user confirmation.",
	}
}

func ChatFileEditHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatFileEditInput,
	proj *engine.Project,
	chatStore *memory.ChatStore,
	sessionMgr *memory.SessionManager,
) (*mcp.CallToolResult, ChatFileEditOutput, error) {
	out := ChatFileEditOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Path == "" {
		out.Error = "path is required"
		return nil, out, nil
	}

	if input.Content == "" {
		out.Error = "content is required"
		return nil, out, nil
	}

	if paths.IsBlockedPath(input.Path) {
		out.Error = fmt.Sprintf("access denied: %q is a protected path", input.Path)
		return nil, out, nil
	}

	abs, err := paths.SafeFilePath(proj, input.Path)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}

	// Read current content
	currentContent := ""
	data, readErr := os.ReadFile(abs)
	if readErr == nil {
		currentContent = string(data)
	}

	// Generate diff
	diff := generateDiff(input.Path, currentContent, input.Content)
	out.Diff = diff
	out.FilePath = input.Path

	// Save as pending edit
	if input.SessionID == "" {
		out.Error = "session_id is required"
		return nil, out, nil
	}

	editID, saveErr := chatStore.SavePendingEdit(input.SessionID, input.Path, input.Content, input.Reason)
	if saveErr != nil {
		out.Error = fmt.Sprintf("failed to save pending edit: %v", saveErr)
		return nil, out, nil
	}
	out.EditID = editID

	return nil, out, nil
}

// generateDiff produces a simple unified diff between old and new content.
func generateDiff(path, oldContent, newContent string) string {
	if oldContent == newContent {
		return fmt.Sprintf("--- %s (no changes)\n+++ %s (no changes)\n", path, path)
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s (current)\n", path))
	sb.WriteString(fmt.Sprintf("+++ %s (proposed)\n", path))
	sb.WriteString("\n")

	// Simple line-by-line diff
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen && i < 50; i++ { // limit diff to 50 lines
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if i < len(oldLines) {
				sb.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if i < len(newLines) {
				sb.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	if maxLen > 50 {
		sb.WriteString(fmt.Sprintf("\n... diff truncated, %d more lines ...\n", maxLen-50))
	}

	return sb.String()
}
