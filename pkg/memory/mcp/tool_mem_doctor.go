package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/memory/diagnostic"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// ── mem_doctor ─────────────────────────────────────────────────────
// Run read-only operational diagnostics.

type MemDoctorInput struct {
	WorkItemID *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Project    *string `json:"project,omitempty" jsonschema:"Project to diagnose (omit for auto-detect)"`
	Check      *string `json:"check,omitempty" jsonschema:"Optional diagnostic check code to run"`
}

func (i *MemDoctorInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemDoctorOutput struct {
	Report        *diagnostic.Report `json:"report"`
	Project       string             `json:"project"`
	ProjectSource string             `json:"project_source"`
	ProjectPath   string             `json:"project_path"`
	Error         string             `json:"error,omitempty"`
	IsError       bool               `json:"is_error,omitempty"`
}

const tnMemDoctor = "mem_doctor"

func init() {
	tools[tnMemDoctor] = &mcp2.Tool{
		Description: "Run read-only operational diagnostics. Returns the same structured envelope as `engram doctor --json`.",
	}
}

// MemDoctorHandler migrates from handleDoctor in mcp.go.
func MemDoctorHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemDoctorInput, proj *project.Project) (*mcp2.CallToolResult, MemDoctorOutput, error) {
	out := MemDoctorOutput{}

	// TODO: Wire up actual diagnostic.NewRunner() call
	// TODO: Wire up project resolution (resolveReadProject)
	// TODO: Wire up diagnostic.RunAll or RunOne

	out.IsError = false

	return nil, out, nil
}

func add_tool_mem_doctor(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_doctor",
			mcp.WithDescription("Run read-only operational diagnostics. Returns the same structured envelope as `engram doctor --json`."),
			mcp.WithDeferLoading(true),
			mcp.WithTitleAnnotation("Run Engram Doctor"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithString("project", mcp.Description("Project to diagnose (omit for auto-detect)")),
			mcp.WithString("check", mcp.Description("Optional diagnostic check code to run")),
		),
		handleDoctor(s),
	)
}

func handleDoctor(s *store.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectOverride, _ := req.GetArguments()["project"].(string)
		check, _ := req.GetArguments()["check"].(string)
		detRes, err := resolveReadProject(s, projectOverride)
		if err != nil {
			var upe *unknownProjectError
			if errors.As(err, &upe) {
				return errorWithMeta("unknown_project", fmt.Sprintf("Project %q not found in store", upe.Name), upe.AvailableProjects), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Project resolution failed: %s", err)), nil
		}
		project := detRes.Project
		project, _ = store.NormalizeProject(project)
		runner := diagnostic.NewRunner()
		scope := diagnostic.Scope{Store: s, Project: project, Now: time.Now()}
		var report diagnostic.Report
		if strings.TrimSpace(check) != "" {
			report, err = runner.RunOne(ctx, scope, check)
		} else {
			report, err = runner.RunAll(ctx, scope)
		}
		if err != nil {
			report = diagnostic.ErrorReport(project, err)
		}
		out, marshalErr := jsonMarshal(report)
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Doctor JSON error: %s", marshalErr)), nil
		}
		result := mcp.NewToolResultText(string(out))
		if report.Status == diagnostic.StatusError {
			result.IsError = true
		}
		return result, nil
	}
}
