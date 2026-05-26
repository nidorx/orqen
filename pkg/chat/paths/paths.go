package paths

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nidorx/orqen/pkg/engine"
)

// Blocked paths for file operations

var BlockedPathPrefixes = []string{".orqen/", ".git/"}

func IsBlockedPath(p string) bool {
	clean := filepath.Clean(p)
	for _, prefix := range BlockedPathPrefixes {
		if strings.HasPrefix(clean, prefix) || strings.HasPrefix(clean, string(filepath.Separator)+prefix) {
			return true
		}
	}
	// Also block exact .orqen and .git at root
	base := strings.Split(clean, string(filepath.Separator))[0]
	if base == ".orqen" || base == ".git" {
		return true
	}
	return false
}

// SafeFilePath validates and resolves a relative path against the project directory.
// If an absolute path is provided, it validates that it's within the project directory.
func SafeFilePath(proj *engine.Project, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}
	if IsBlockedPath(relPath) {
		return "", fmt.Errorf("access denied: %q is a protected path", relPath)
	}

	var abs string
	if filepath.IsAbs(relPath) {
		// Absolute path: clean it and validate it's within the project
		abs = filepath.Clean(relPath)
		// Ensure the resolved path is still within the project directory
		if !strings.HasPrefix(abs, proj.DirAbs) {
			return "", fmt.Errorf("access denied: path escapes project directory")
		}
	} else {
		// Relative path: resolve against project directory
		abs = filepath.Join(proj.DirAbs, relPath)
		// Ensure the resolved path is still within the project directory
		if !strings.HasPrefix(abs, proj.DirAbs) {
			return "", fmt.Errorf("access denied: path escapes project directory")
		}
	}
	return abs, nil
}
