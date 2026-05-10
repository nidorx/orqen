package mcp

import (
	"context"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	projectpkg "github.com/nidorx/orqen/pkg/memory/project"
	"github.com/nidorx/orqen/pkg/memory/store"
)

// ── mem_current_project ────────────────────────────────────────────
// Detect the current project from the working directory.

type MemCurrentProjectInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
}

func (i *MemCurrentProjectInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemCurrentProjectOutput struct {
	Project           string   `json:"project"`
	ProjectSource     string   `json:"project_source"`
	ProjectPath       string   `json:"project_path"`
	CWD               string   `json:"cwd"`
	AvailableProjects []string `json:"available_projects"`
	Warning           string   `json:"warning,omitempty"`
	ErrorHint         string   `json:"error_hint,omitempty"`
}

const tnMemCurrentProject = "mem_current_project"

func init() {
	tools[tnMemCurrentProject] = &mcp2.Tool{
		Description: "Detect the current project from the working directory. Returns project name, source (how it was detected), path, and available alternatives. NEVER errors — use this for discovery before writing. Recommended as the first call when starting a new session.",
	}
}

// MemCurrentProjectHandler migrates from handleCurrentProject in mcp.go.
func MemCurrentProjectHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemCurrentProjectInput, proj *engine.Project) (*mcp2.CallToolResult, MemCurrentProjectOutput, error) {
	out := MemCurrentProjectOutput{}

	// TODO: Wire up actual projectpkg.DetectProjectFull call
	// TODO: Wire up os.Getwd() for CWD

	out.ProjectSource = "auto-detect"

	return nil, out, nil
}

func add_tool_mem_current_project(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_current_project",
			mcp.WithDescription("Detect the current project from the working directory. Returns project name, source (how it was detected), path, and available alternatives. NEVER errors — use this for discovery before writing. Recommended as the first call when starting a new session."),
			mcp.WithTitleAnnotation("Detect Current Project"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		),
		handleCurrentProject(s),
	)
}

// handleCurrentProject implements mem_current_project. It NEVER returns an error
// even on ambiguous cwd — it always returns a success result with whatever
// detection info is available (REQ-313).
func handleCurrentProject(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cwd, _ := os.Getwd()
		res := projectpkg.DetectProjectFull(cwd)

		envelope := map[string]any{
			"project":            res.Project,
			"project_source":     res.Source,
			"project_path":       res.Path,
			"cwd":                cwd,
			"available_projects": res.AvailableProjects,
		}
		if res.Warning != "" {
			envelope["warning"] = res.Warning
		}
		if res.Error != nil {
			// REQ-313: not an error response — just surface the info.
			envelope["error_hint"] = res.Error.Error()
		}
		out, _ := jsonMarshal(envelope)
		return mcp.NewToolResultText(string(out)), nil
	}
}
