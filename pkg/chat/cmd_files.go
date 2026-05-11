package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const filesLimit = 50

func init() {
	RegisterCommand(CommandDef{
		Name:        "files",
		Description: "List files in the project directory",
		Handler:     handleFiles,
	})
}

func handleFiles(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	proj := bot.Project
	if proj == nil {
		return "No project loaded.", nil
	}

	relPath := strings.TrimSpace(args)
	if relPath == "" {
		relPath = "."
	}

	// Validate path
	if isBlockedPath(relPath) {
		return fmt.Sprintf("Access denied: %q is a protected path.", relPath), nil
	}

	absPath := filepath.Join(proj.DirAbs, relPath)

	// Ensure path is within project
	cleanAbs, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		cleanAbs = absPath
	}
	if !strings.HasPrefix(filepath.Clean(cleanAbs), proj.DirAbs) {
		return "Access denied: path escapes project directory.", nil
	}

	entries, err := os.ReadDir(cleanAbs)
	if err != nil {
		return fmt.Sprintf("Error reading directory: %v", err), nil
	}

	// Filter out blocked entries
	var filtered []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		skip := false
		for _, prefix := range blockedPathPrefixes {
			basePrefix := strings.TrimSuffix(prefix, "/")
			if name == basePrefix {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}

	// Sort: directories first, then files
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].IsDir() != filtered[j].IsDir() {
			return filtered[i].IsDir()
		}
		return filtered[i].Name() < filtered[j].Name()
	})

	limit := filesLimit
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	var sb strings.Builder
	displayPath := relPath
	if displayPath == "." {
		displayPath = "/"
	}
	sb.WriteString(fmt.Sprintf("Directory: %s\n\n", displayPath))

	for _, e := range filtered {
		prefix := "  "
		if e.IsDir() {
			prefix = "📁 "
		} else {
			prefix = "📄 "
		}
		sb.WriteString(fmt.Sprintf("%s%s\n", prefix, e.Name()))
	}

	if len(entries) > limit {
		sb.WriteString(fmt.Sprintf("\n... (%d of %d entries shown)\n", limit, len(entries)))
	}

	return sb.String(), nil
}
