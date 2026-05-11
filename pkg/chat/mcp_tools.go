package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// ── Chat tool handler type and adapter ───────────────────────────────────────

// ToolChatHandler is the generic handler signature for chat MCP tools.
type ToolChatHandler[In, Out any] func(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input In,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (result *mcp.CallToolResult, output Out, err error)

// chatHandler2MCP adapts a ToolChatHandler into an mcp.ToolHandlerFor by closing
// over proj, chatStore, and sessionMgr.
func chatHandler2MCP[In, Out any](
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
	h ToolChatHandler[In, Out],
) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		return h(ctx, req, input, proj, chatStore, sessionMgr)
	}
}

// chatTools holds all registered chat tools. Populated by init() in each section.
var chatTools = map[string]*mcp.Tool{}

// addChatToolWithHandler registers a tool with its handler.
func addChatToolWithHandler[In, Out any](s *mcp.Server, name string, h ToolChatHandler[In, Out], proj *engine.Project, chatStore *ChatStore, sessionMgr *SessionManager) {
	tool := chatTools[name]
	if tool == nil {
		panic(fmt.Sprintf("chat: tool %q not registered in chatTools map", name))
	}
	tool.Name = name
	mcp.AddTool(s, tool, chatHandler2MCP(proj, chatStore, sessionMgr, h))
}

// ── Blocked paths for file operations ────────────────────────────────────────

var blockedPathPrefixes = []string{".orqen/", ".git/"}

func isBlockedPath(p string) bool {
	clean := filepath.Clean(p)
	for _, prefix := range blockedPathPrefixes {
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

// safeFilePath validates and resolves a relative path against the project directory.
func safeFilePath(proj *engine.Project, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}
	if isBlockedPath(relPath) {
		return "", fmt.Errorf("access denied: %q is a protected path", relPath)
	}
	abs := filepath.Join(proj.DirAbs, relPath)
	// Ensure the resolved path is still within the project directory
	if !strings.HasPrefix(abs, proj.DirAbs) {
		return "", fmt.Errorf("access denied: path escapes project directory")
	}
	return abs, nil
}

// ── Tool: chat_history_get ──────────────────────────────────────────────────

type ChatHistoryGetInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID (omit for current session)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum number of messages to return (default: 20)"`
}

type ChatHistoryGetOutput struct {
	Messages []MessageView `json:"messages"`
	Error    string        `json:"error,omitempty"`
}

type MessageView struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

const tnChatHistoryGet = "chat_history_get"

func init() {
	chatTools[tnChatHistoryGet] = &mcp.Tool{
		Description: "Get recent conversation messages from the current chat session.",
	}
}

func ChatHistoryGetHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatHistoryGetInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatHistoryGetOutput, error) {
	out := ChatHistoryGetOutput{}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	sessionID := input.SessionID
	if sessionID == "" {
		out.Error = "session_id is required"
		return nil, out, nil
	}

	msgs, err := sessionMgr.GetSessionHistory(sessionID, limit)
	if err != nil {
		out.Error = fmt.Sprintf("failed to get history: %v", err)
		return nil, out, nil
	}

	for _, m := range msgs {
		out.Messages = append(out.Messages, MessageView{
			Role:    string(m.Role),
			Content: m.Content,
			Time:    m.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, out, nil
}

// ── Tool: chat_memory_search ────────────────────────────────────────────────

type ChatMemorySearchInput struct {
	Query  string `json:"query" jsonschema:"Search query text"`
	UserID string `json:"user_id,omitempty" jsonschema:"User ID to scope search (omit for current user)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum results (default: 10)"`
}

type ChatMemorySearchOutput struct {
	Results []SearchResultView `json:"results"`
	Error   string             `json:"error,omitempty"`
}

type SearchResultView struct {
	Content   string  `json:"content"`
	Role      string  `json:"role"`
	SessionID string  `json:"session_id"`
	Rank      float64 `json:"rank"`
	Time      string  `json:"time"`
}

const tnChatMemorySearch = "chat_memory_search"

func init() {
	chatTools[tnChatMemorySearch] = &mcp.Tool{
		Description: "Search past conversations using full-text search across all your chat sessions.",
	}
}

func ChatMemorySearchHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatMemorySearchInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatMemorySearchOutput, error) {
	out := ChatMemorySearchOutput{}

	if input.Query == "" {
		out.Error = "query is required"
		return nil, out, nil
	}

	if input.UserID == "" {
		out.Error = "user_id is required"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = SearchLimit
	}

	results, err := chatStore.Search(input.UserID, input.Query, limit)
	if err != nil {
		out.Error = fmt.Sprintf("search failed: %v", err)
		return nil, out, nil
	}

	for _, r := range results {
		out.Results = append(out.Results, SearchResultView{
			Content:   r.Content,
			Role:      r.Role,
			SessionID: r.SessionID,
			Rank:      r.Rank,
			Time:      r.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil, out, nil
}

// ── Tool: chat_workitem_create ───────────────────────────────────────────────

type ChatWorkitemCreateInput struct {
	Lane    string `json:"lane" jsonschema:"Destination lane name"`
	Title   string `json:"title" jsonschema:"Work item title"`
	Content string `json:"content,omitempty" jsonschema:"Optional content/description"`
}

type ChatWorkitemCreateOutput struct {
	Success  bool             `json:"success"`
	WorkItem *engine.WorkItem `json:"workitem,omitempty"`
	Error    string           `json:"error,omitempty"`
}

const tnChatWorkitemCreate = "chat_workitem_create"

func init() {
	chatTools[tnChatWorkitemCreate] = &mcp.Tool{
		Description: "Create a new workitem in a specified lane. Creates the directory and .yaml file.",
	}
}

func ChatWorkitemCreateHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatWorkitemCreateInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatWorkitemCreateOutput, error) {
	out := ChatWorkitemCreateOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Lane == "" {
		out.Error = "lane is required"
		return nil, out, nil
	}

	if input.Title == "" {
		out.Error = "title is required"
		return nil, out, nil
	}

	// Find the lane across all modules
	var targetLane *engine.Lane
	for _, mod := range proj.Modules {
		if l := mod.GetLane(input.Lane); l != nil {
			targetLane = l
			break
		}
	}
	if targetLane == nil {
		out.Error = fmt.Sprintf("lane %q not found", input.Lane)
		return nil, out, nil
	}

	wi, err := targetLane.CreateWorkItem(input.Title)
	if err != nil {
		out.Error = fmt.Sprintf("failed to create workitem: %v", err)
		return nil, out, nil
	}

	// If content was provided, write it to the yaml file
	if input.Content != "" {
		yamlPath := filepath.Join(wi.Lane.DirAbs, wi.Name+".yaml")
		if err := os.WriteFile(yamlPath, []byte(input.Content), 0644); err != nil {
			out.Error = fmt.Sprintf("created workitem but failed to write content: %v", err)
			return nil, out, nil
		}
	}

	out.Success = true
	out.WorkItem = wi
	return nil, out, nil
}

// ── Tool: chat_workitem_list ────────────────────────────────────────────────

type ChatWorkitemListInput struct {
	Lane  string `json:"lane,omitempty" jsonschema:"Optional lane name to filter by"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum items to return (default: 20)"`
}

type ChatWorkitemListOutput struct {
	Items []WorkitemSummary `json:"items"`
	Error string            `json:"error,omitempty"`
}

type WorkitemSummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Lane  string `json:"lane"`
	Title string `json:"title"`
}

const tnChatWorkitemList = "chat_workitem_list"

func init() {
	chatTools[tnChatWorkitemList] = &mcp.Tool{
		Description: "List workitems, optionally filtered by lane.",
	}
}

func ChatWorkitemListHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatWorkitemListInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatWorkitemListOutput, error) {
	out := ChatWorkitemListOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	count := 0
	for _, mod := range proj.Modules {
		for _, lane := range mod.Lanes {
			if input.Lane != "" && lane.Name != input.Lane {
				continue
			}
			// WorkItems() panics if cache not initialized; recover gracefully
			func() {
				defer func() { recover() }()
				for wi := range lane.WorkItems() {
					if count >= limit {
						break
					}
					title, _ := wi.Attributes["title"].(string)
					if title == "" {
						title = wi.Name
					}
					out.Items = append(out.Items, WorkitemSummary{
						ID:    wi.ID,
						Name:  wi.Name,
						Lane:  lane.Name,
						Title: title,
					})
					count++
				}
			}()
			if count >= limit {
				break
			}
		}
		if count >= limit {
			break
		}
	}

	return nil, out, nil
}

// ── Tool: chat_workitem_get ─────────────────────────────────────────────────

type ChatWorkitemGetInput struct {
	WorkItemID string `json:"workitem_id" jsonschema:"Work item ID to look up"`
}

type ChatWorkitemGetOutput struct {
	Found     bool             `json:"found"`
	WorkItem  *engine.WorkItem `json:"workitem,omitempty"`
	Lane      string           `json:"lane,omitempty"`
	Module    string           `json:"module,omitempty"`
	FileCount int              `json:"file_count,omitempty"`
	Error     string           `json:"error,omitempty"`
}

const tnChatWorkitemGet = "chat_workitem_get"

func init() {
	chatTools[tnChatWorkitemGet] = &mcp.Tool{
		Description: "Get details of a specific workitem including its files and attributes.",
	}
}

func ChatWorkitemGetHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatWorkitemGetInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatWorkitemGetOutput, error) {
	out := ChatWorkitemGetOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.WorkItemID == "" {
		out.Error = "workitem_id is required"
		return nil, out, nil
	}

	// GetWorkItemById panics if cache not initialized; recover gracefully
	var wi *engine.WorkItem
	func() {
		defer func() { recover() }()
		wi = proj.GetWorkItemById(input.WorkItemID)
	}()
	if wi == nil {
		out.Error = fmt.Sprintf("workitem %q not found", input.WorkItemID)
		return nil, out, nil
	}

	out.Found = true
	out.WorkItem = wi
	out.Lane = wi.Lane.Name
	out.Module = wi.Lane.Module.Name
	out.FileCount = len(wi.Files)

	return nil, out, nil
}

// ── Tool: chat_file_list ────────────────────────────────────────────────────

type ChatFileListInput struct {
	Path      string `json:"path,omitempty" jsonschema:"Relative path to list (default: project root)"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"Whether to list recursively"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum entries to return (default: 50)"`
}

type ChatFileListOutput struct {
	Entries []FileEntry `json:"entries"`
	Error   string      `json:"error,omitempty"`
}

type FileEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "directory"
	Size int64  `json:"size"`
}

const tnChatFileList = "chat_file_list"

func init() {
	chatTools[tnChatFileList] = &mcp.Tool{
		Description: "List files in the project directory, optionally filtered by path.",
	}
}

func ChatFileListHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatFileListInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatFileListOutput, error) {
	out := ChatFileListOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	searchPath := proj.DirAbs
	if input.Path != "" {
		searchPath = filepath.Join(proj.DirAbs, input.Path)
	}

	count := 0
	err := filepath.WalkDir(searchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		rel, relErr := filepath.Rel(proj.DirAbs, path)
		if relErr != nil {
			rel = path
		}

		// Skip blocked paths
		if isBlockedPath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, statErr := d.Info()
		size := int64(0)
		if statErr == nil {
			size = info.Size()
		}

		entryType := "file"
		if d.IsDir() {
			entryType = "directory"
		}

		out.Entries = append(out.Entries, FileEntry{
			Path: filepath.ToSlash(rel),
			Type: entryType,
			Size: size,
		})
		count++

		if count >= limit {
			return filepath.SkipDir
		}

		if !input.Recursive && d.IsDir() && rel != "." {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		out.Error = fmt.Sprintf("failed to list files: %v", err)
		return nil, out, nil
	}

	// Sort entries for consistent output
	sort.Slice(out.Entries, func(i, j int) bool {
		if out.Entries[i].Type != out.Entries[j].Type {
			return out.Entries[i].Type < out.Entries[j].Type
		}
		return out.Entries[i].Path < out.Entries[j].Path
	})

	return nil, out, nil
}

// ── Tool: chat_file_read ────────────────────────────────────────────────────

type ChatFileReadInput struct {
	Path  string `json:"path" jsonschema:"File path relative to project root"`
	Line  int    `json:"line,omitempty" jsonschema:"Start line (1-indexed, default: 1)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Number of lines to read (default: 100)"`
}

type ChatFileReadOutput struct {
	Content string `json:"content"`
	Lines   int    `json:"lines_read"`
	Total   int    `json:"total_lines,omitempty"`
	Error   string `json:"error,omitempty"`
}

const tnChatFileRead = "chat_file_read"

const maxFileReadSize = 50 * 1024 // 50KB

func init() {
	chatTools[tnChatFileRead] = &mcp.Tool{
		Description: "Read a file's content from the project directory. Large files are truncated.",
	}
}

func ChatFileReadHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatFileReadInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatFileReadOutput, error) {
	out := ChatFileReadOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Path == "" {
		out.Error = "path is required"
		return nil, out, nil
	}

	abs, err := safeFilePath(proj, input.Path)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}

	// Read with size limit
	info, statErr := os.Stat(abs)
	if statErr != nil {
		out.Error = fmt.Sprintf("cannot stat file: %v", statErr)
		return nil, out, nil
	}

	readLimit := int64(maxFileReadSize)
	if info.Size() > readLimit {
		// Read only up to the limit
		data := make([]byte, readLimit)
		f, openErr := os.Open(abs)
		if openErr != nil {
			out.Error = fmt.Sprintf("cannot open file: %v", openErr)
			return nil, out, nil
		}
		n, readErr := f.Read(data)
		f.Close()
		if readErr != nil && readErr.Error() != "EOF" {
			// Read may return nil or EOF at end
		}
		out.Content = string(data[:n]) + "\n...[truncated, file is large]..."
	} else {
		data, readErr := os.ReadFile(abs)
		if readErr != nil {
			out.Error = fmt.Sprintf("cannot read file: %v", readErr)
			return nil, out, nil
		}
		out.Content = string(data)
	}

	// Apply line pagination
	allLines := strings.Split(out.Content, "\n")
	out.Total = len(allLines)

	startLine := input.Line
	if startLine <= 0 {
		startLine = 1
	}
	lineLimit := input.Limit
	if lineLimit <= 0 {
		lineLimit = 100
	}

	startIdx := startLine - 1
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(allLines) {
		out.Content = ""
		out.Lines = 0
		return nil, out, nil
	}

	endIdx := startIdx + lineLimit
	if endIdx > len(allLines) {
		endIdx = len(allLines)
	}

	out.Content = strings.Join(allLines[startIdx:endIdx], "\n")
	out.Lines = endIdx - startIdx

	return nil, out, nil
}

// ── Tool: chat_file_edit ────────────────────────────────────────────────────

type ChatFileEditInput struct {
	Path      string `json:"path" jsonschema:"File path relative to project root"`
	Content   string `json:"content" jsonschema:"Full proposed file content"`
	Reason    string `json:"reason,omitempty" jsonschema:"Reason for the edit"`
	SessionID string `json:"session_id,omitempty" jsonschema:"Session ID for the pending edit"`
}

type ChatFileEditOutput struct {
	EditID   int64  `json:"edit_id"`
	FilePath string `json:"file_path"`
	Diff     string `json:"diff_preview"`
	Error    string `json:"error,omitempty"`
}

const tnChatFileEdit = "chat_file_edit"

func init() {
	chatTools[tnChatFileEdit] = &mcp.Tool{
		Description: "Propose a file edit. Does NOT apply the edit — creates a pending edit requiring user confirmation.",
	}
}

func ChatFileEditHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatFileEditInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatFileEditOutput, error) {
	out := ChatFileEditOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	if input.Path == "" {
		out.Error = "path is required"
		return nil, out, nil
	}

	if input.Content == "" {
		out.Error = "content is required"
		return nil, out, nil
	}

	if isBlockedPath(input.Path) {
		out.Error = fmt.Sprintf("access denied: %q is a protected path", input.Path)
		return nil, out, nil
	}

	abs, err := safeFilePath(proj, input.Path)
	if err != nil {
		out.Error = err.Error()
		return nil, out, nil
	}

	// Read current content
	currentContent := ""
	data, readErr := os.ReadFile(abs)
	if readErr == nil {
		currentContent = string(data)
	}

	// Generate diff
	diff := generateDiff(input.Path, currentContent, input.Content)
	out.Diff = diff
	out.FilePath = input.Path

	// Save as pending edit
	if input.SessionID == "" {
		out.Error = "session_id is required"
		return nil, out, nil
	}

	editID, saveErr := chatStore.SavePendingEdit(input.SessionID, input.Path, input.Content, input.Reason)
	if saveErr != nil {
		out.Error = fmt.Sprintf("failed to save pending edit: %v", saveErr)
		return nil, out, nil
	}
	out.EditID = editID

	return nil, out, nil
}

// generateDiff produces a simple unified diff between old and new content.
func generateDiff(path, oldContent, newContent string) string {
	if oldContent == newContent {
		return fmt.Sprintf("--- %s (no changes)\n+++ %s (no changes)\n", path, path)
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s (current)\n", path))
	sb.WriteString(fmt.Sprintf("+++ %s (proposed)\n", path))
	sb.WriteString("\n")

	// Simple line-by-line diff
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen && i < 50; i++ { // limit diff to 50 lines
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if i < len(oldLines) {
				sb.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if i < len(newLines) {
				sb.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	if maxLen > 50 {
		sb.WriteString(fmt.Sprintf("\n... diff truncated, %d more lines ...\n", maxLen-50))
	}

	return sb.String()
}

// ── Tool: chat_project_get ──────────────────────────────────────────────────

type ChatProjectGetInput struct{}

type ChatProjectGetOutput struct {
	DirAbs      string            `json:"directory"`
	ModuleCount int               `json:"module_count"`
	Modules     []ModuleSummary   `json:"modules"`
	AgentCount  int               `json:"agent_count"`
	LaneStats   []LaneStatSummary `json:"lane_stats"`
	Error       string            `json:"error,omitempty"`
}

type ModuleSummary struct {
	Name      string `json:"name"`
	ItemCount int    `json:"item_count"`
	LaneCount int    `json:"lane_count"`
}

type LaneStatSummary struct {
	Module string `json:"module"`
	Name   string `json:"lane"`
	Count  int    `json:"item_count"`
}

const tnChatProjectGet = "chat_project_get"

func init() {
	chatTools[tnChatProjectGet] = &mcp.Tool{
		Description: "Get an overview of the project structure including modules, lanes, and workitem counts.",
	}
}

func ChatProjectGetHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatProjectGetInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatProjectGetOutput, error) {
	out := ChatProjectGetOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	out.DirAbs = proj.DirAbs
	out.ModuleCount = len(proj.Modules)

	// ActiveAgentCount may panic without cache init
	agentCount := 0
	func() {
		defer func() { recover() }()
		agentCount = proj.ActiveAgentCount()
	}()
	out.AgentCount = agentCount

	for _, mod := range proj.Modules {
		itemCount := 0
		func() {
			defer func() { recover() }()
			for range mod.WorkItems() {
				itemCount++
			}
		}()
		out.Modules = append(out.Modules, ModuleSummary{
			Name:      mod.Name,
			ItemCount: itemCount,
			LaneCount: len(mod.Lanes),
		})

		for _, lane := range mod.Lanes {
			laneCount := 0
			func() {
				defer func() { recover() }()
				laneCount = lane.CountWorkItems()
			}()
			out.LaneStats = append(out.LaneStats, LaneStatSummary{
				Module: mod.Name,
				Name:   lane.Name,
				Count:  laneCount,
			})
		}
	}

	return nil, out, nil
}

// ── Tool: chat_lane_list ────────────────────────────────────────────────────

type ChatLaneListInput struct {
	Module string `json:"module,omitempty" jsonschema:"Filter by module name"`
}

type ChatLaneListOutput struct {
	Lanes []ChatLaneDetail `json:"lanes"`
	Error string           `json:"error,omitempty"`
}

type ChatLaneDetail struct {
	Module    string `json:"module"`
	Name      string `json:"name"`
	Purpose   string `json:"purpose"`
	MaxAgents int    `json:"max_agents"`
	ItemCount int    `json:"item_count"`
}

const tnChatLaneList = "chat_lane_list"

func init() {
	chatTools[tnChatLaneList] = &mcp.Tool{
		Description: "List all lanes in the project with their purpose and item counts.",
	}
}

func ChatLaneListHandler(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input *ChatLaneListInput,
	proj *engine.Project,
	chatStore *ChatStore,
	sessionMgr *SessionManager,
) (*mcp.CallToolResult, ChatLaneListOutput, error) {
	out := ChatLaneListOutput{}

	if proj == nil {
		out.Error = "project not available"
		return nil, out, nil
	}

	for _, mod := range proj.Modules {
		if input.Module != "" && mod.Name != input.Module {
			continue
		}
		for _, lane := range mod.Lanes {
			itemCount := 0
			// CountWorkItems panics if cache not initialized; recover gracefully
			func() {
				defer func() { recover() }()
				itemCount = lane.CountWorkItems()
			}()
			out.Lanes = append(out.Lanes, ChatLaneDetail{
				Module:    mod.Name,
				Name:      lane.Name,
				Purpose:   lane.Purpose,
				MaxAgents: lane.MaxAgents,
				ItemCount: itemCount,
			})
		}
	}

	return nil, out, nil
}

// ── RegisterAllTools ────────────────────────────────────────────────────────

// RegisterAllTools registers all chat tools with the given MCP server.
func RegisterAllTools(s *mcp.Server, proj *engine.Project, chatStore *ChatStore, sessionMgr *SessionManager) {
	addChatToolWithHandler(s, tnChatHistoryGet, ChatHistoryGetHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatMemorySearch, ChatMemorySearchHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatWorkitemCreate, ChatWorkitemCreateHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatWorkitemList, ChatWorkitemListHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatWorkitemGet, ChatWorkitemGetHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatFileList, ChatFileListHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatFileRead, ChatFileReadHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatFileEdit, ChatFileEditHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatProjectGet, ChatProjectGetHandler, proj, chatStore, sessionMgr)
	addChatToolWithHandler(s, tnChatLaneList, ChatLaneListHandler, proj, chatStore, sessionMgr)
}

// ── JSON helpers ─────────────────────────────────────────────────────────────

// toJSON marshals a value to a JSON string for MCP result content.
func toJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
