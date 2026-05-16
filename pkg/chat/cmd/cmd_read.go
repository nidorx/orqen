package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nidorx/orqen/pkg/chat/paths"
)

const readMaxLen = 2000

func init() {
	Register(Command{
		Name:        "read",
		Description: "Read a file's content",
		Handler:     readCommandHandler,
	})
}

func readCommandHandler(ctx context.Context, req *Request) (string, error) {
	proj := req.Project
	if proj == nil {
		return "**Error:** No project loaded.", nil
	}

	filePath := strings.TrimSpace(req.Content)
	if filePath == "" {
		return "**Usage:** `/read <file_path>`", nil
	}

	// Validate path
	if paths.IsBlockedPath(filePath) {
		return fmt.Sprintf("**Error:** Access denied: %q is a protected path.", filePath), nil
	}

	absPath, err := paths.SafeFilePath(proj, filePath)
	if err != nil {
		return err.Error(), nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("**Error:** Error reading file: %v", err), nil
	}

	text := string(content)
	truncated := false
	if len(text) > readMaxLen {
		text = text[:readMaxLen]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## File: `%s`\n\n", filePath))
	sb.WriteString("```\n")
	sb.WriteString(text)
	sb.WriteString("\n```")

	if truncated {
		sb.WriteString(fmt.Sprintf("\n\n*... (truncated to %d characters)*", readMaxLen))
	}

	return sb.String(), nil
}
