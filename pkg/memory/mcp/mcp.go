// Package mcp implements the Model Context Protocol server for Engram.
//
// This exposes memory tools via MCP stdio transport so ANY agent
// (OpenCode, Claude Code, Cursor, Windsurf, etc.) can use Engram's
// persistent memory just by adding it as an MCP server.
//
// Tool profiles allow agents to load only the tools they need:
//
//	engram mcp                    → all 18 tools (default)
//	engram mcp --tools=agent      → 14 tools agents actually use (per skill files)
//	engram mcp --tools=admin      → 4 tools for TUI/CLI (delete, stats, timeline, merge)
//	engram mcp --tools=agent,admin → combine profiles
//	engram mcp --tools=mem_save,mem_search → individual tool names
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	projectpkg "github.com/nidorx/orqen/pkg/memory/project"
	"github.com/nidorx/orqen/pkg/memory/store"
)

// MCPConfig holds configuration for the MCP server.
// JW6: DefaultProject removed — it was populated but never read (dead code).
// Project is always auto-detected from cwd at call time via resolveWriteProject/resolveReadProject.
type MCPConfig struct {
	// BM25Floor overrides the default BM25 score floor used by FindCandidates
	// during conflict candidate detection (REQ-001). The floor is the minimum
	// acceptable BM25 rank (negative; closer to 0 = better match). Candidates
	// whose score falls below this threshold are excluded.
	//
	// nil means "use the store default" (-2.0). An explicit pointer value
	// (including 0.0) is forwarded directly. Using a pointer avoids the
	// zero-value ambiguity where 0.0 would otherwise be indistinguishable
	// from "not set".
	BM25Floor *float64

	// Limit overrides the maximum number of conflict candidates returned per
	// mem_save call (REQ-001). nil means "use the store default" (3).
	// An explicit pointer value (including 0) is forwarded directly.
	Limit *int
}

var suggestTopicKey = store.SuggestTopicKey

var addPromptIfMissing = func(s *store.Store, params store.AddPromptParams) (int64, bool, error) {
	return s.AddPromptIfMissing(params)
}

var loadMCPStats = func(s *store.Store) (*store.Stats, error) {
	return s.Stats()
}

func currentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func ensureImplicitSessionWithCWD(s *store.Store, sessionID, project string) error {
	return s.CreateSession(sessionID, project, currentWorkingDirectory())
}

// ─── Tool Profiles ───────────────────────────────────────────────────────────
//
// "agent" — tools AI agents use during coding sessions:
//   mem_save, mem_search, mem_context, mem_session_summary,
//   mem_session_start, mem_session_end, mem_get_observation,
//   mem_suggest_topic_key, mem_capture_passive, mem_save_prompt
//
// "admin" — tools for manual curation, TUI, and dashboards:
//   mem_update, mem_delete, mem_stats, mem_timeline, mem_merge_projects
//
// "all" (default) — every tool registered.

// ProfileAgent contains the tool names that AI agents need.
// Sourced from actual skill files and memory protocol instructions
// across all 4 supported agents (Claude Code, OpenCode, Gemini CLI, Codex).
var ProfileAgent = map[string]bool{
	"mem_save":              true, // proactive save — referenced 17 times across protocols
	"mem_search":            true, // search past memories — referenced 6 times
	"mem_context":           true, // recent context from previous sessions — referenced 10 times
	"mem_session_summary":   true, // end-of-session summary — referenced 16 times
	"mem_session_start":     true, // register session start
	"mem_session_end":       true, // mark session completed
	"mem_get_observation":   true, // full observation content after search — referenced 4 times
	"mem_suggest_topic_key": true, // stable topic key for upserts — referenced 3 times
	"mem_capture_passive":   true, // extract learnings from text — referenced in Gemini/Codex protocol
	"mem_save_prompt":       true, // save user prompts
	"mem_update":            true, // update observation by ID — skills say "use mem_update when you have an exact ID to correct"
	"mem_current_project":   true, // detect current project — recommended first call for agents (REQ-313)
	"mem_judge":             true, // record verdict on a pending memory conflict (REQ-003, Phase D)
	"mem_compare":           true, // persist an agent-judged semantic verdict via JudgeBySemantic (REQ-011, Phase G)
	"mem_doctor":            true, // read-only operational diagnostics for agents
}

// ProfileAdmin contains tools for TUI, dashboards, and manual curation
// that are NOT referenced in any agent skill or memory protocol.
var ProfileAdmin = map[string]bool{
	"mem_delete":         true, // only in OpenCode's ENGRAM_TOOLS filter, not in any agent instructions
	"mem_stats":          true, // only in OpenCode's ENGRAM_TOOLS filter, not in any agent instructions
	"mem_timeline":       true, // only in OpenCode's ENGRAM_TOOLS filter, not in any agent instructions
	"mem_merge_projects": true, // destructive curation tool — not for agent use
}

// Profiles maps profile names to their tool sets.
var Profiles = map[string]map[string]bool{
	"agent": ProfileAgent,
	"admin": ProfileAdmin,
}

// ResolveTools takes a comma-separated string of profile names and/or
// individual tool names and returns the set of tool names to register.
// An empty input means "all" — every tool is registered.
func ResolveTools(input string) map[string]bool {
	input = strings.TrimSpace(input)
	if input == "" || input == "all" {
		return nil // nil means register everything
	}

	result := make(map[string]bool)
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == "all" {
			return nil
		}
		if profile, ok := Profiles[token]; ok {
			for tool := range profile {
				result[tool] = true
			}
		} else {
			// Treat as individual tool name
			result[token] = true
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// NewServer creates an MCP server with ALL tools registered (backwards compatible).
func NewServer(s *store.Store) *server.MCPServer {
	return NewServerWithConfig(s, MCPConfig{}, nil)
}

// serverInstructions tells MCP clients when to use Engram's tools.
// 6 core tools are eager (always in context). The rest are deferred
// and require ToolSearch to load.
const serverInstructions = `Engram provides persistent memory that survives across sessions and compactions.

CORE TOOLS (always available — use without ToolSearch):
  mem_save — save decisions, bugs, discoveries, conventions PROACTIVELY (do not wait to be asked)
  mem_search — find past work, decisions, or context from previous sessions
  mem_context — get recent session history (call at session start or after compaction)
  mem_session_summary — save end-of-session summary (MANDATORY before saying "done")
  mem_get_observation — get full untruncated content of a search result by ID
  mem_save_prompt — save user prompt for context
  mem_current_project — detect current project from cwd (recommended first call)

DEFERRED TOOLS (use ToolSearch when needed):
  mem_update, mem_suggest_topic_key, mem_session_start, mem_session_end,
  mem_stats, mem_delete, mem_timeline, mem_capture_passive, mem_merge_projects

PROACTIVE SAVE RULE: Call mem_save immediately after ANY decision, bug fix, discovery, or convention — not just when asked.

## CONFLICT SURFACING — when mem_save returns candidates

After every mem_save call, check the response envelope for judgment_required.

IF judgment_required IS TRUE:
  Iterate candidates[] and call mem_judge once per candidate using the
  judgment_id from that candidate entry. Do NOT use the top-level judgment_id
  for multiple candidates — each candidate has its own.

  HEURISTIC — when to ask the user vs. resolve autonomously:

  ASK the user (conversationally, as part of your next reply) when:
    - confidence is below 0.7, OR
    - the relation you would choose is supersedes or conflicts_with AND the
      observation type is architecture, policy, or decision

  RESOLVE silently (call mem_judge without asking) when:
    - confidence >= 0.7 AND the relation is not supersedes/conflicts_with, OR
    - the relation is related, compatible, scoped, or not_conflict

  HOW TO ASK (conversationally — never via blocking CLI or dashboard prompt):
    Raise it naturally in your next reply to the user. Example phrasing:
    "I noticed memory #abc123 might conflict with what we just saved.
     Want me to mark the new one as superseding it, or are they about
     different scopes? I can also mark them as compatible if both still apply."

  AFTER RESOLUTION (both paths):
    Call mem_judge with the chosen relation, a reason, and if the user gave
    explicit direction, include their words as the evidence field. This persists
    the verdict and closes the pending conflict row.`

// NewServerWithTools creates an MCP server registering only the tools in
// the allowlist. If allowlist is nil, all tools are registered.
func NewServerWithTools(s *store.Store, allowlist map[string]bool) *server.MCPServer {
	return NewServerWithConfig(s, MCPConfig{}, allowlist)
}

// NewServerWithConfig creates an MCP server with full configuration including
// default project detection and optional tool allowlist.
func NewServerWithConfig(s *store.Store, cfg MCPConfig, allowlist map[string]bool) *server.MCPServer {
	return newServerWithActivity(s, cfg, allowlist, NewSessionActivity(10*time.Minute))
}

func newServerWithActivity(s *store.Store, cfg MCPConfig, allowlist map[string]bool, activity *SessionActivity) *server.MCPServer {
	srv := server.NewMCPServer(
		"engram",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(serverInstructions),
	)

	registerTools(srv, s, cfg, allowlist, activity)
	return srv
}

// shouldRegister returns true if the tool should be registered given the
// allowlist. If allowlist is nil, everything is allowed.
func shouldRegister(name string, allowlist map[string]bool) bool {
	if allowlist == nil {
		return true
	}
	return allowlist[name]
}

func registerTools(srv *server.MCPServer, s *store.Store, cfg MCPConfig, allowlist map[string]bool, activity *SessionActivity) {

	// ─── mem_search (profile: agent, core — always in context) ─────────
	if shouldRegister("mem_search", allowlist) {
		add_tool_mem_search(srv, s, cfg, activity)
	}

	// ─── mem_save (profile: agent, core — always in context) ───────────
	if shouldRegister("mem_save", allowlist) {
		add_tool_mem_save(srv, s, cfg, activity)
	}

	// ─── mem_update (profile: agent, deferred) ──────────────────────────
	if shouldRegister("mem_update", allowlist) {
		add_tool_mem_update(srv, s, cfg, activity)
	}

	// ─── mem_suggest_topic_key (profile: agent, deferred) ───────────────
	if shouldRegister("mem_suggest_topic_key", allowlist) {
		add_tool_mem_suggest_topic_key(srv, s, cfg, activity)
	}

	// ─── mem_delete (profile: admin, deferred) ──────────────────────────
	if shouldRegister("mem_delete", allowlist) {
		add_tool_mem_delete(srv, s, cfg, activity)
	}

	// ─── mem_save_prompt (profile: agent, eager) ────────────────────────
	if shouldRegister("mem_save_prompt", allowlist) {
		add_tool_mem_save_prompt(srv, s, cfg, activity)
	}

	// ─── mem_context (profile: agent, core — always in context) ────────
	if shouldRegister("mem_context", allowlist) {
		add_tool_mem_context(srv, s, cfg, activity)
	}

	// ─── mem_stats (profile: admin, deferred) ───────────────────────────
	if shouldRegister("mem_stats", allowlist) {
		add_tool_mem_stats(srv, s, cfg, activity)
	}

	// ─── mem_timeline (profile: admin, deferred) ────────────────────────
	if shouldRegister("mem_timeline", allowlist) {
		add_tool_mem_timeline(srv, s, cfg, activity)
	}

	// ─── mem_get_observation (profile: agent, eager) ────────────────────
	if shouldRegister("mem_get_observation", allowlist) {
		add_tool_mem_get_observation(srv, s, cfg, activity)
	}

	// ─── mem_session_summary (profile: agent, core — always in context) ─
	if shouldRegister("mem_session_summary", allowlist) {
		add_tool_mem_session_summary(srv, s, cfg, activity)

	}

	// ─── mem_session_start (profile: agent, deferred) ───────────────────
	if shouldRegister("mem_session_start", allowlist) {
		add_tool_mem_session_start(srv, s, cfg, activity)
	}

	// ─── mem_session_end (profile: agent, deferred) ─────────────────────
	if shouldRegister("mem_session_end", allowlist) {
		add_tool_mem_session_end(srv, s, cfg, activity)
	}

	// ─── mem_capture_passive (profile: agent, deferred) ─────────────────
	if shouldRegister("mem_capture_passive", allowlist) {
		add_tool_mem_capture_passive(srv, s, cfg, activity)
	}

	// ─── mem_merge_projects (profile: admin, deferred) ──────────────────
	if shouldRegister("mem_merge_projects", allowlist) {
		add_tool_mem_merge_projects(srv, s, cfg, activity)
	}

	// ─── mem_current_project (profile: agent) ────────────────────────────
	if shouldRegister("mem_current_project", allowlist) {
		add_tool_mem_current_project(srv, s, cfg, activity)
	}

	// ─── mem_doctor (profile: agent, deferred) ──────────────────────────
	if shouldRegister("mem_doctor", allowlist) {
		add_tool_mem_doctor(srv, s, cfg, activity)
	}

	// ─── mem_judge (profile: agent, eager) — REQ-003, Design §6 ─────────
	if shouldRegister("mem_judge", allowlist) {
		add_tool_mem_judge(srv, s, cfg, activity)
	}

	// ─── mem_compare (profile: agent, eager) — REQ-011, Design §9 ────────
	if shouldRegister("mem_compare", allowlist) {
		add_tool_mem_compare(srv, s, cfg, activity)
	}
}

// ─── Tool Handlers ───────────────────────────────────────────────────────────

func DoctorToolHandler(s *store.Store) server.ToolHandlerFunc {
	return handleDoctor(s)
}

func resolveSessionStartProject(explicitDirectory string) (projectpkg.DetectionResult, error) {
	if explicitDirectory == "" {
		return resolveWriteProject()
	}
	res := projectpkg.DetectProjectFull(explicitDirectory)
	if res.Error != nil {
		return res, res.Error
	}
	if res.Source == projectpkg.SourceDirBasename {
		return resolveWriteProject()
	}
	return res, nil
}

// ─── Project Resolution Helpers ──────────────────────────────────────────────

// unknownProjectError is returned when a read tool receives a project override
// that does not exist in the store.
type unknownProjectError struct {
	Name              string
	AvailableProjects []string
}

func (e *unknownProjectError) Error() string {
	return "unknown project: " + e.Name
}

type invalidProjectChoiceError struct {
	Name              string
	AvailableProjects []string
}

func (e *invalidProjectChoiceError) Error() string {
	return "invalid project choice: " + e.Name
}

// resolveWriteProject detects the current project from the process working
// directory. Returns ErrAmbiguousProject if cwd is a parent of multiple repos.
func resolveWriteProject() (projectpkg.DetectionResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	res := projectpkg.DetectProjectFull(cwd)
	if res.Error != nil {
		return res, res.Error
	}
	return res, nil
}

// resolveWriteProjectWithChoice preserves normal write resolution authority and
// only uses an explicit project choice as a recovery path from ErrAmbiguousProject.
func resolveWriteProjectWithChoice(projectChoice, reason string) (projectpkg.DetectionResult, error) {
	res, err := resolveWriteProject()
	if err == nil {
		// Non-ambiguous config/git/autodetect remains authoritative. Ignore any
		// supplied project choice so agents cannot drift writes to arbitrary buckets.
		return res, nil
	}
	if !errors.Is(err, projectpkg.ErrAmbiguousProject) {
		return res, err
	}

	if strings.TrimSpace(reason) != projectpkg.SourceUserSelectedAfterAmbiguousProject {
		return res, err
	}

	choice := strings.TrimSpace(projectChoice)
	if choice == "" || !containsProjectChoice(res.AvailableProjects, choice) {
		return res, &invalidProjectChoiceError{
			Name:              choice,
			AvailableProjects: res.AvailableProjects,
		}
	}

	res.Project = choice
	res.Source = projectpkg.SourceUserSelectedAfterAmbiguousProject
	res.Path = resolveAmbiguousChoicePath(res.Path, choice)
	res.Warning = "project selected by user after ambiguous_project recovery"
	return res, nil
}

func containsProjectChoice(available []string, choice string) bool {
	choice = strings.TrimSpace(choice)
	for _, candidate := range available {
		if strings.TrimSpace(candidate) == choice {
			return true
		}
	}
	return false
}

func resolveAmbiguousChoicePath(ambiguousParent, choice string) string {
	parent := strings.TrimSpace(ambiguousParent)
	if parent == "" || strings.TrimSpace(choice) == "" {
		return ""
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Match the same name shape used by project.DetectProjectFull for
		// available_projects: trim + lowercase only. Do not use store.NormalizeProject
		// here because it collapses repeated '-'/'_' and can create collisions.
		if strings.TrimSpace(strings.ToLower(entry.Name())) != choice {
			continue
		}
		childPath := filepath.Join(parent, entry.Name())
		if _, err := os.Stat(filepath.Join(childPath, ".git")); err != nil {
			continue
		}
		absChild, err := filepath.Abs(childPath)
		if err != nil {
			return childPath
		}
		return absChild
	}
	return ""
}

// resolveReadProject validates an optional project override against the store.
// If override is empty, falls back to auto-detection from cwd.
// JW2: normalizes the override (lowercase+trim) before ProjectExists lookup so
// that e.g. "MyApp" and "  myapp  " both resolve to the stored "myapp".
func resolveReadProject(s *store.Store, override string) (projectpkg.DetectionResult, error) {
	override = strings.TrimSpace(override)
	if override == "" {
		return resolveWriteProject()
	}
	normalized, _ := store.NormalizeProject(override)
	exists, err := s.ProjectExists(normalized)
	if err != nil {
		return projectpkg.DetectionResult{}, err
	}
	if !exists {
		// Collect available projects for the error.
		stats, _ := s.Stats()
		return projectpkg.DetectionResult{}, &unknownProjectError{
			Name:              normalized,
			AvailableProjects: stats.Projects,
		}
	}
	return projectpkg.DetectionResult{
		Project: normalized,
		Source:  projectpkg.SourceExplicitOverride, // JR2-2: use named constant
		Path:    "",
	}, nil
}

// respondWithProject wraps a tool result by prepending the project envelope
// fields (project, project_source, project_path) to the text output.
// extra is an optional map of additional fields to include.
func respondWithProject(res projectpkg.DetectionResult, text string, extra map[string]any) *mcp.CallToolResult {
	envelope := map[string]any{
		"project":        res.Project,
		"project_source": res.Source,
		"project_path":   res.Path,
		"result":         text,
	}
	if res.Warning != "" {
		envelope["warning"] = res.Warning
	}
	for k, v := range extra {
		envelope[k] = v
	}
	out, _ := jsonMarshal(envelope)
	return mcp.NewToolResultText(string(out))
}

func writeProjectErrorResult(res projectpkg.DetectionResult, err error) *mcp.CallToolResult {
	code := "ambiguous_project"
	if errors.Is(err, projectpkg.ErrInvalidConfig) {
		code = "invalid_project_config"
	}
	var choiceErr *invalidProjectChoiceError
	if errors.As(err, &choiceErr) {
		if choiceErr.Name == "" {
			return errorWithMeta("invalid_project_choice",
				"Project choice is empty; choose exactly one value from available_projects and retry with project_choice_reason=user_selected_after_ambiguous_project",
				choiceErr.AvailableProjects,
			)
		}
		return errorWithMeta("invalid_project_choice",
			fmt.Sprintf("Project choice %q is not one of available_projects", choiceErr.Name),
			choiceErr.AvailableProjects,
		)
	}
	return errorWithMeta(code, fmt.Sprintf("Cannot determine project: %s", err), res.AvailableProjects)
}

// errorWithMeta returns a structured tool error result with error_code,
// message, available_projects, and a hint for resolution.
func errorWithMeta(code, msg string, availableProjects []string) *mcp.CallToolResult {
	envelope := map[string]any{
		"error_code":         code,
		"message":            msg,
		"available_projects": availableProjects,
	}
	switch code {
	case "ambiguous_project":
		envelope["hint"] = "Ask the user to choose one of available_projects, then retry mem_save or mem_save_prompt with project and project_choice_reason=user_selected_after_ambiguous_project; alternatively cd into the target repo or add repo .engram/config.json."
	case "invalid_project_choice":
		envelope["hint"] = "Use exactly one of available_projects after asking the user, or cd into the target repo, or add repo .engram/config.json."
	case "unknown_project":
		envelope["hint"] = "Use one of the available_projects values, or omit project to auto-detect."
	case "invalid_project_config":
		envelope["hint"] = "Fix .engram/config.json so project_name is a non-empty project name."
	}
	out, _ := jsonMarshal(envelope)
	result := mcp.NewToolResultText(string(out))
	result.IsError = true
	return result
}

// jsonMarshal marshals v to JSON. Named to allow test injection if needed.
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// defaultSessionID returns a project-scoped default session ID.
// If project is non-empty: "manual-save-{project}"
// If project is empty: "manual-save"
func defaultSessionID(project string) string {
	if project == "" {
		return "manual-save"
	}
	return "manual-save-" + project
}

func intArg(req mcp.CallToolRequest, key string, defaultVal int) int {
	v, ok := req.GetArguments()[key].(float64)
	if !ok {
		return defaultVal
	}
	return int(v)
}

func boolArg(req mcp.CallToolRequest, key string, defaultVal bool) bool {
	v, ok := req.GetArguments()[key].(bool)
	if !ok {
		return defaultVal
	}
	return v
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
