package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// ── mem_capture_passive ────────────────────────────────────────────
// Extract and save structured learnings from text output.

type MemCapturePassiveInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Content    string  `json:"content" jsonschema:"The text output containing a '## Key Learnings:' section"`
	SessionID  *string `json:"session_id,omitempty" jsonschema:"Session ID (default: manual-save-{project})"`
	Source     *string `json:"source,omitempty" jsonschema:"Source identifier (e.g. 'subagent-stop', 'session-end')"`
}

func (i *MemCapturePassiveInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemCapturePassiveOutput struct {
	Message       string `json:"message"`
	Project       string `json:"project"`
	ProjectSource string `json:"project_source"`
	ProjectPath   string `json:"project_path"`
	Extracted     int    `json:"extracted"`
	Saved         int    `json:"saved"`
	Duplicates    int    `json:"duplicates"`
	Error         string `json:"error,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
}

const tnMemCapturePassive = "mem_capture_passive"

func init() {
	tools[tnMemCapturePassive] = &mcp2.Tool{
		Description: `Extract and save structured learnings from text output. Use this at the end of a task to anticipate knowledge automatically.

The tool looks for sections like "## Key Learnings:" or "## Aprendizajes Clave:" and extracts numbered or bulleted items. Each item is saved as a separate observation.

Duplicates are automatically detected and skipped — safe to call multiple times with the same content.`,
	}
}

// MemCapturePassiveHandler migrates from handleCapturePassive in mcp.go.
func MemCapturePassiveHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemCapturePassiveInput, proj *project.Project) (*mcp2.CallToolResult, MemCapturePassiveOutput, error) {
	out := MemCapturePassiveOutput{}

	if input.Content == "" {
		out.Error = "content is required — include text with a '## Key Learnings:' section"
		return nil, out, nil
	}

	// TODO: Wire up actual store.PassiveCapture call
	// TODO: Wire up project resolution (resolveWriteProject)

	out.Message = "Passive capture complete"

	return nil, out, nil
}

func add_tool_mem_capture_passive(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_capture_passive",
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Capture Learnings"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithDescription(`Extract and save structured learnings from text output. Use this at the end of a task to capture knowledge automatically.

The tool looks for sections like "## Key Learnings:" or "## Aprendizajes Clave:" and extracts numbered or bulleted items. Each item is saved as a separate observation.

Duplicates are automatically detected and skipped — safe to call multiple times with the same content.`),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("The text output containing a '## Key Learnings:' section with numbered or bulleted items"),
			),
			mcp.WithString("session_id",
				mcp.Description("Session ID (default: manual-save-{project})"),
			),
			mcp.WithString("source",
				mcp.Description("Source identifier (e.g. 'subagent-stop', 'session-end')"),
			),
		),
		queuedWriteHandler(getWriteQueue(), handleCapturePassive(s, cfg, activity)),
	)
}

func handleCapturePassive(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, _ := req.GetArguments()["content"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		source, _ := req.GetArguments()["source"].(string)
		// project field intentionally not read — auto-detect only (REQ-308)

		detRes, err := resolveWriteProject()
		if err != nil {
			return writeProjectErrorResult(detRes, err), nil
		}
		project, _ := store.NormalizeProject(detRes.Project)

		activity.RecordToolCall(defaultSessionID(project))

		if content == "" {
			return mcp.NewToolResultError("content is required — include text with a '## Key Learnings:' section"), nil
		}

		if sessionID == "" {
			sessionID = defaultSessionID(project)
			_ = ensureImplicitSessionWithCWD(s, sessionID, project)
		}

		if source == "" {
			source = "mcp-passive"
		}

		result, err := s.PassiveCapture(store.PassiveCaptureParams{
			SessionID: sessionID,
			Content:   content,
			Project:   project,
			Source:    source,
		})
		if err != nil {
			return mcp.NewToolResultError("Passive capture failed: " + err.Error()), nil
		}

		detRes.Project = project
		return respondWithProject(detRes, fmt.Sprintf(
			"Passive capture complete: extracted=%d saved=%d duplicates=%d",
			result.Extracted, result.Saved, result.Duplicates,
		), nil), nil
	}
}
