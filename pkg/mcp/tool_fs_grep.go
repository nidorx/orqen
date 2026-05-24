package mcp

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/paths"
	"github.com/nidorx/orqen/pkg/engine"
)

type FsGrepInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Pattern    string  `json:"pattern" jsonschema:"regex pattern to search for"`
	Filepath   string  `json:"filepath" jsonschema:"path to file to search"`
	IgnoreCase *bool   `json:"ignore_case,omitempty" jsonschema:"perform case-insensitive matching (default: false)"`
	MaxResults *int    `json:"max_results,omitempty" jsonschema:"maximum number of matching lines (default: 1000)"`
}

func (i *FsGrepInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type GrepMatch struct {
	LineNum int    `json:"line_num"`
	Content string `json:"content"`
}

type FsGrepOutput struct {
	Matches []GrepMatch `json:"matches,omitempty"`
	Count   int         `json:"count"`
	Error   string      `json:"error,omitempty"`
}

const tnFsGrep = "fs_grep"

func init() {
	tools[tnFsGrep] = &mcp.Tool{
		Description: "Search for regex pattern in file contents. Returns matching lines with line numbers.",
	}
}

func FsGrepHandler(ctx context.Context, req *mcp.CallToolRequest, input *FsGrepInput, proj *engine.Project) (*mcp.CallToolResult, FsGrepOutput, error) {
	out := FsGrepOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Pattern == "" {
		out.Error = "pattern is required"
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

	// Compile regex
	pattern := input.Pattern
	if input.IgnoreCase != nil && *input.IgnoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		out.Error = "invalid regex pattern: " + err.Error()
		return nil, out, nil
	}

	data, readErr := os.ReadFile(fileAbs)
	if readErr != nil {
		out.Error = "failed to read file: " + readErr.Error()
		return nil, out, nil
	}

	lines := strings.Split(string(data), "\n")
	maxResults := 1000
	if input.MaxResults != nil {
		maxResults = *input.MaxResults
	}

	for i, line := range lines {
		if len(out.Matches) >= maxResults {
			break
		}

		if re.MatchString(line) {
			out.Matches = append(out.Matches, GrepMatch{
				LineNum: i + 1,
				Content: line,
			})
		}
	}

	out.Count = len(out.Matches)
	return nil, out, nil
}
