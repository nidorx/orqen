package mcp

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcp2 "github.com/modelcontextprotocol/go-sdk/mcp"
	projectpkg "github.com/nidorx/orqen/pkg/memory/project"
	"github.com/nidorx/orqen/pkg/memory/store"
	"github.com/nidorx/orqen/pkg/project"
)

// ── mem_save ───────────────────────────────────────────────────────
// Save an important observation to persistent memory.

type MemSaveInput struct {
	WorkItemID          *string `json:"workitem_id,omitempty" jsonschema:"Work Item ID (auto-injected)"`
	Title               string  `json:"title" jsonschema:"Short, searchable title (e.g. 'JWT auth middleware', 'Fixed N+1 query')"`
	Content             string  `json:"content" jsonschema:"Structured content using **What**, **Why**, **Where**, **Learned** format"`
	Type                *string `json:"type,omitempty" jsonschema:"Category: decision, architecture, bugfix, pattern, config, discovery, learning (default: manual)"`
	SessionID           *string `json:"session_id,omitempty" jsonschema:"Session ID to associate with (default: manual-save-{project})"`
	Scope               *string `json:"scope,omitempty" jsonschema:"Scope for this observation: project (default) or personal"`
	TopicKey            *string `json:"topic_key,omitempty" jsonschema:"Optional topic identifier for upserts (e.g. architecture/auth-model)"`
	Project             *string `json:"project,omitempty" jsonschema:"Optional recovery target only after ambiguous_project"`
	ProjectChoiceReason *string `json:"project_choice_reason,omitempty" jsonschema:"Must be user_selected_after_ambiguous_project"`
	CapturePrompt       *bool   `json:"capture_prompt,omitempty" jsonschema:"Automatically capture the current user prompt when available (default: true)"`
}

func (i *MemSaveInput) SetWorkItemID(workItemID string) {
	i.WorkItemID = &workItemID
}

type MemSaveCandidate struct {
	ID         string  `json:"id"`
	SyncID     string  `json:"sync_id"`
	Title      string  `json:"title"`
	Type       string  `json:"type"`
	Score      float64 `json:"score"`
	JudgmentID string  `json:"judgment_id"`
	TopicKey   *string `json:"topic_key,omitempty"`
}

type MemSaveOutput struct {
	Message           string             `json:"message"`
	Project           string             `json:"project"`
	ProjectSource     string             `json:"project_source"`
	ProjectPath       string             `json:"project_path"`
	ID                int64              `json:"id,omitempty"`
	SyncID            string             `json:"sync_id,omitempty"`
	SuggestedTopicKey string             `json:"suggested_topic_key,omitempty"`
	JudgmentRequired  bool               `json:"judgment_required,omitempty"`
	JudgmentStatus    string             `json:"judgment_status,omitempty"`
	JudgmentID        string             `json:"judgment_id,omitempty"`
	Candidates        []MemSaveCandidate `json:"candidates,omitempty"`
	Truncated         bool               `json:"truncated,omitempty"`
	Warning           string             `json:"warning,omitempty"`
	Error             string             `json:"error,omitempty"`
	ErrorCode         string             `json:"error_code,omitempty"`
}

const tnMemSave = "mem_save"

func init() {
	tools[tnMemSave] = &mcp2.Tool{
		Description: `Save an important observation to persistent memory. Call this PROACTIVELY after completing significant work — don't wait to be asked.

WHEN to save (call this after each of these):
- Architectural decisions or tradeoffs
- Bug fixes (what was wrong, why, how you fixed it)
- New patterns or conventions established
- Configuration changes or environment setup
- Important discoveries or gotchas
- File structure changes

FORMAT for content — use this structured format:
  **What**: [concise description of what was done]
  **Why**: [the reasoning, user request, or problem that drove it]
  **Where**: [files/paths affected, e.g. src/auth/middleware.ts, internal/store/store.go]
  **Learned**: [any gotchas, edge cases, or decisions made — omit if none]

TITLE should be short and searchable, like: "JWT auth middleware", "FTS5 query sanitization", "Fixed N+1 in user list"

Examples:
  title: "Switched from sessions to JWT"
  type: "decision"
  content: "**What**: Replaced express-session with jsonwebtoken for auth\n**Why**: Session storage doesn't scale across multiple instances\n**Where**: src/middleware/auth.ts, src/routes/login.ts\n**Learned**: Must set httpOnly and secure flags on the cookie, refresh tokens need separate rotation logic"

  title: "Fixed FTS5 syntax error on special chars"
  type: "bugfix"
  content: "**What**: Wrapped each search term in quotes before passing to FTS5 MATCH\n**Why**: Users typing queries like 'fix auth bug' would crash because FTS5 interprets special chars as operators\n**Where**: internal/store/store.go — sanitizeFTS() function\n**Learned**: FTS5 MATCH syntax is NOT the same as LIKE — always sanitize user input"`,
		Title: "Save Memory",
		Annotations: &mcp2.ToolAnnotations{
			Title:           "Save Memory",
			ReadOnlyHint:    false,
			DestructiveHint: ptrBool(false),
			IdempotentHint:  false,
			OpenWorldHint:   ptrBool(false),
		},
	}
}

// MemSaveHandler migrates from handleSave in mcp.go.
func MemSaveHandler(ctx context.Context, req *mcp2.CallToolRequest, input *MemSaveInput, proj *project.Project) (*mcp2.CallToolResult, MemSaveOutput, error) {
	return nil, MemSaveOutput{}, nil
}

func add_tool_mem_save(srv *server.MCPServer, s *store.Store, cfg MCPConfig, activity *SessionActivity) {
	srv.AddTool(
		mcp.NewTool("mem_save",
			mcp.WithTitleAnnotation("Save Memory"),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(false),
			mcp.WithDescription(`Save an important observation to persistent memory. Call this PROACTIVELY after completing significant work — don't wait to be asked.

WHEN to save (call this after each of these):
- Architectural decisions or tradeoffs
- Bug fixes (what was wrong, why, how you fixed it)
- New patterns or conventions established
- Configuration changes or environment setup
- Important discoveries or gotchas
- File structure changes

FORMAT for content — use this structured format:
  **What**: [concise description of what was done]
  **Why**: [the reasoning, user request, or problem that drove it]
  **Where**: [files/paths affected, e.g. src/auth/middleware.ts, internal/store/store.go]
  **Learned**: [any gotchas, edge cases, or decisions made — omit if none]

TITLE should be short and searchable, like: "JWT auth middleware", "FTS5 query sanitization", "Fixed N+1 in user list"

Examples:
  title: "Switched from sessions to JWT"
  type: "decision"
  content: "**What**: Replaced express-session with jsonwebtoken for auth\n**Why**: Session storage doesn't scale across multiple instances\n**Where**: src/middleware/auth.ts, src/routes/login.ts\n**Learned**: Must set httpOnly and secure flags on the cookie, refresh tokens need separate rotation logic"

  title: "Fixed FTS5 syntax error on special chars"
  type: "bugfix"
  content: "**What**: Wrapped each search term in quotes before passing to FTS5 MATCH\n**Why**: Users typing queries like 'fix auth bug' would crash because FTS5 interprets special chars as operators\n**Where**: internal/store/store.go — sanitizeFTS() function\n**Learned**: FTS5 MATCH syntax is NOT the same as LIKE — always sanitize user input"`),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("Short, searchable title (e.g. 'JWT auth middleware', 'Fixed N+1 query')"),
			),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("Structured content using **What**, **Why**, **Where**, **Learned** format"),
			),
			mcp.WithString("type",
				mcp.Description("Category: decision, architecture, bugfix, pattern, config, discovery, learning (default: manual)"),
			),
			mcp.WithString("session_id",
				mcp.Description("Session ID to associate with (default: manual-save-{project})"),
			),
			mcp.WithString("scope",
				mcp.Description("Scope for this observation: project (default) or personal"),
			),
			mcp.WithString("topic_key",
				mcp.Description("Optional topic identifier for upserts (e.g. architecture/auth-model). Reuses and updates the latest observation in same project+scope."),
			),
			mcp.WithString("project",
				mcp.Description("Optional recovery target only after ambiguous_project. Ignored unless project_choice_reason is user_selected_after_ambiguous_project."),
			),
			mcp.WithString("project_choice_reason",
				mcp.Description("Must be user_selected_after_ambiguous_project, and only after the user explicitly chose one of available_projects from an ambiguous_project error."),
			),
			mcp.WithBoolean("capture_prompt",
				mcp.Description("Automatically capture the current user prompt when available (default: true). Set false for SDD artifacts or automated saves."),
			),
		),
		queuedWriteHandler(getWriteQueue(), handleSave(s, cfg, activity)),
	)
}

func handleSave(s *store.Store, cfg MCPConfig, activity *SessionActivity) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, _ := req.GetArguments()["title"].(string)
		content, _ := req.GetArguments()["content"].(string)
		typ, _ := req.GetArguments()["type"].(string)
		sessionID, _ := req.GetArguments()["session_id"].(string)
		scope, _ := req.GetArguments()["scope"].(string)
		topicKey, _ := req.GetArguments()["topic_key"].(string)
		projectChoice, _ := req.GetArguments()["project"].(string)
		projectChoiceReason, _ := req.GetArguments()["project_choice_reason"].(string)
		capturePrompt := boolArg(req, "capture_prompt", true)

		// Auto-detect project from cwd; only allow explicit user-selected recovery
		// after ErrAmbiguousProject (issue #306).
		detRes, err := resolveWriteProjectWithChoice(projectChoice, projectChoiceReason)
		if err != nil {
			return writeProjectErrorResult(detRes, err), nil
		}
		project := detRes.Project

		// Normalize project name and capture warning
		normalized, normWarning := store.NormalizeProject(project)
		project = normalized

		if typ == "" {
			typ = "manual"
		}
		if sessionID == "" {
			sessionID = defaultSessionID(project)
		}
		suggestedTopicKey := suggestTopicKey(typ, title, content)

		// Check for similar existing projects (only when this project has no existing observations)
		var similarWarning string
		if project != "" {
			existingNames, _ := s.ListProjectNames()
			isNew := true
			for _, e := range existingNames {
				if e == project {
					isNew = false
					break
				}
			}
			if isNew && len(existingNames) > 0 {
				matches := projectpkg.FindSimilar(project, existingNames, 3)
				if len(matches) > 0 {
					bestMatch := matches[0].Name
					obsCount, _ := s.CountObservationsForProject(bestMatch)
					similarWarning = fmt.Sprintf("⚠️ Project %q has no memories. Similar project found: %q (%d memories). Consider using that name instead.", project, bestMatch, obsCount)
				}
			}
		}

		// Ensure the implicit MCP session exists with the current working directory.
		_ = ensureImplicitSessionWithCWD(s, sessionID, project)

		truncated := len(content) > s.MaxObservationLength()

		savedID, err := s.AddObservation(store.AddObservationParams{
			SessionID: sessionID,
			Type:      typ,
			Title:     title,
			Content:   content,
			Project:   project,
			Scope:     scope,
			TopicKey:  topicKey,
		})
		if err != nil {
			return mcp.NewToolResultError("Failed to save: " + err.Error()), nil
		}

		if capturePrompt && activity != nil {
			if prompt, ok := activity.CurrentPrompt(sessionID, project); ok {
				if _, _, promptErr := addPromptIfMissing(s, store.AddPromptParams{
					SessionID: sessionID,
					Content:   prompt,
					Project:   project,
				}); promptErr != nil {
					fmt.Fprintf(os.Stderr, "engram: auto prompt capture error (non-fatal): %v\n", promptErr)
				}
			}
		}

		if activity != nil {
			activity.RecordSave(sessionID)
		}

		msg := fmt.Sprintf("Memory saved: %q (%s)", title, typ)
		if topicKey == "" && suggestedTopicKey != "" {
			msg += fmt.Sprintf("\nSuggested topic_key: %s", suggestedTopicKey)
		}
		if truncated {
			msg += fmt.Sprintf("\n⚠ WARNING: Content was truncated from %d to %d chars. Consider splitting into smaller observations.", len(content), s.MaxObservationLength())
		}
		if normWarning != "" {
			msg += "\n" + normWarning
		}
		if similarWarning != "" {
			msg += "\n" + similarWarning
		}

		// Post-transaction conflict candidate detection (REQ-001).
		// Errors are logged and swallowed — detection failure never fails the save.
		extra := map[string]any{}
		// Build CandidateOptions, forwarding any MCPConfig overrides.
		// nil fields mean "use store defaults"; explicit pointer values override.
		candOpts := store.CandidateOptions{
			Project:   project,
			Scope:     scope,
			BM25Floor: cfg.BM25Floor, // nil → store default (-2.0); explicit value overrides
		}
		if cfg.Limit != nil {
			candOpts.Limit = *cfg.Limit
		}
		candidates, candErr := s.FindCandidates(savedID, candOpts)
		if candErr != nil {
			// Log only — do not fail the save.
			fmt.Fprintf(os.Stderr, "engram: FindCandidates error (non-fatal): %v\n", candErr)
		}

		// Fetch the saved observation's sync_id for the envelope (REQ-001).
		var savedSyncID string
		if obs, obsErr := s.GetObservation(savedID); obsErr == nil {
			savedSyncID = obs.SyncID
			extra["id"] = savedID
			extra["sync_id"] = savedSyncID
		}

		if len(candidates) > 0 {
			extra["judgment_required"] = true
			extra["judgment_status"] = "pending"
			extra["judgment_id"] = candidates[0].JudgmentID // first candidate's rel sync_id (design convenience)

			candList := make([]map[string]any, 0, len(candidates))
			for _, c := range candidates {
				entry := map[string]any{
					"id":          c.ID,
					"sync_id":     c.SyncID,
					"title":       c.Title,
					"type":        c.Type,
					"score":       c.Score,
					"judgment_id": c.JudgmentID,
				}
				if c.TopicKey != nil {
					entry["topic_key"] = *c.TopicKey
				}
				candList = append(candList, entry)
			}
			extra["candidates"] = candList

			msg += fmt.Sprintf("\nCONFLICT REVIEW PENDING — %d candidate(s); use mem_judge to record verdicts.", len(candidates))
		} else {
			extra["judgment_required"] = false
		}

		// Update detRes to reflect normalized project for envelope accuracy
		detRes.Project = project
		return respondWithProject(detRes, msg, extra), nil
	}
}
