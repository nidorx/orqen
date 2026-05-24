package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsDiffInput struct {
	File1   string `json:"file1" jsonschema:"first file path"`
	File2   string `json:"file2" jsonschema:"second file path"`
	Context *int   `json:"context,omitempty" jsonschema:"number of context lines (default: 3)"`
}

type FsDiffOutput struct {
	Diff  string `json:"diff,omitempty"`
	Error string `json:"error,omitempty"`
}

const tnFsDiff = "fs_diff"

func init() {
	tools[tnFsDiff] = &mcp.Tool{
		Description: "Show unified diff between two files (similar to diff -u).",
	}
}

func FsDiffHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsDiffInput, proj *engine.Project) (*mcp.CallToolResult, FsDiffOutput, error) {
	out := FsDiffOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.File1 == "" {
		out.Error = "file1 is required"
		return nil, out, nil
	}
	if input.File2 == "" {
		out.Error = "file2 is required"
		return nil, out, nil
	}

	file1Abs, err := paths.SafeFilePath(proj, input.File1)
	if err != nil {
		out.Error = "invalid file1 path: " + err.Error()
		return nil, out, nil
	}

	file2Abs, err := paths.SafeFilePath(proj, input.File2)
	if err != nil {
		out.Error = "invalid file2 path: " + err.Error()
		return nil, out, nil
	}

	data1, err1 := os.ReadFile(file1Abs)
	if err1 != nil {
		out.Error = "failed to read file1: " + err1.Error()
		return nil, out, nil
	}

	data2, err2 := os.ReadFile(file2Abs)
	if err2 != nil {
		out.Error = "failed to read file2: " + err2.Error()
		return nil, out, nil
	}

	lines1 := strings.Split(string(data1), "\n")
	lines2 := strings.Split(string(data2), "\n")

	contextLines := 3
	if input.Context != nil {
		contextLines = *input.Context
	}

	out.Diff = unifiedDiff(input.File1, input.File2, lines1, lines2, contextLines)
	return nil, out, nil
}

// unifiedDiff generates a unified diff similar to diff -u
func unifiedDiff(file1, file2 string, lines1, lines2 []string, context int) string {
	// Find the common prefix and suffix
	commonPrefix := 0
	for commonPrefix < len(lines1) && commonPrefix < len(lines2) && lines1[commonPrefix] == lines2[commonPrefix] {
		commonPrefix++
	}

	commonSuffix := 0
	for commonSuffix < len(lines1)-commonPrefix && commonSuffix < len(lines2)-commonPrefix &&
		lines1[len(lines1)-1-commonSuffix] == lines2[len(lines2)-1-commonSuffix] {
		commonSuffix++
	}

	// If files are identical
	if commonPrefix == len(lines1) && commonPrefix == len(lines2) {
		return ""
	}

	// Extract the differing sections
	diff1 := lines1[commonPrefix : len(lines1)-commonSuffix]
	diff2 := lines2[commonPrefix : len(lines2)-commonSuffix]

	// Calculate context boundaries
	start1 := max(0, commonPrefix-context)
	end1 := min(len(lines1), commonPrefix+len(diff1)+context)
	start2 := max(0, commonPrefix-context)
	_ = start2 // suppress unused variable warning

	// Build the diff output
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- %s\n", file1))
	sb.WriteString(fmt.Sprintf("+++ %s\n", file2))

	// Add context before
	contextBefore := lines1[start1:commonPrefix]
	if len(contextBefore) > 0 {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			start1+1, len(contextBefore)+len(diff1),
			start2+1, len(contextBefore)+len(diff2)))
		for _, line := range contextBefore {
			sb.WriteString(fmt.Sprintf(" %s\n", line))
		}
	} else {
		sb.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n",
			commonPrefix+1, len(diff1),
			commonPrefix+1, len(diff2)))
	}

	// Add removed lines
	for _, line := range diff1 {
		sb.WriteString(fmt.Sprintf("-%s\n", line))
	}

	// Add added lines
	for _, line := range diff2 {
		sb.WriteString(fmt.Sprintf("+%s\n", line))
	}

	// Add context after
	end2 := min(len(lines2), commonPrefix+len(diff2)+context)
	contextAfter := lines1[commonPrefix+len(diff1) : end1]
	for _, line := range contextAfter {
		sb.WriteString(fmt.Sprintf(" %s\n", line))
	}
	_ = end2 // will be used when context-after logic is enhanced

	return sb.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
