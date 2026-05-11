package chat

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const readMaxLen = 2000

func init() {
	RegisterCommand(CommandDef{
		Name:        "read",
		Description: "Read a file's content",
		Handler:     handleRead,
	})
}

func handleRead(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	proj := bot.Project
	if proj == nil {
		return "No project loaded.", nil
	}

	filePath := strings.TrimSpace(args)
	if filePath == "" {
		return "Usage: /read <file_path>", nil
	}

	// Validate path
	if isBlockedPath(filePath) {
		return fmt.Sprintf("Access denied: %q is a protected path.", filePath), nil
	}

	absPath, err := safeFilePath(proj, filePath)
	if err != nil {
		return err.Error(), nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err), nil
	}

	text := string(content)
	truncated := false
	if len(text) > readMaxLen {
		text = text[:readMaxLen]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\n\n", filePath))
	sb.WriteString("```\n")
	sb.WriteString(text)
	sb.WriteString("\n```")

	if truncated {
		sb.WriteString(fmt.Sprintf("\n\n... (truncated to %d characters)", readMaxLen))
	}

	return sb.String(), nil
}
