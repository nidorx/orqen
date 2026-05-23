package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test 1: Server creation - NewChatMCPServer returns a non-nil handler.
// ============================================================================
func TestNewChatMCPServer_NonNilHandler(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	handler := NewChatMCPServer(proj, store, mgr)
	if handler == nil {
		t.Fatal("NewChatMCPServer returned nil handler")
	}
}

// ============================================================================
// Test 2: Tool registration - verify all tools from chatTools are registered.
// ============================================================================
func TestNewChatMCPServer_AllToolsRegistered(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	handler := NewChatMCPServer(proj, store, mgr)
	client := newMCPTestClient(t, handler)
	client.Initialize()

	resp := client.ToolsList()
	tools, ok := resp["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array in result, got: %v", resp)
	}

	if len(tools) != len(chatTools) {
		t.Errorf("expected %d tools registered, got %d", len(chatTools), len(tools))
	}
}

// ============================================================================
// Test 3: Tool call - chat_history_get returns messages.
// ============================================================================
func TestMCPServer_ChatHistoryGet(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Add some messages
	for i := 1; i <= 3; i++ {
		if err := store.AddMessage(sess.ID, memory.RoleUser, "test message"); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	handler := NewChatMCPServer(proj, store, mgr)
	client := newMCPTestClient(t, handler)
	client.Initialize()

	result := client.CallTool("chat_history_get", map[string]any{
		"session_id": sess.ID,
		"limit":      3,
	})

	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array with items, got: %v", result)
	}

	// The content should contain a text element with the JSON output
	found := false
	for _, c := range content {
		cmap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if _, hasText := cmap["text"]; hasText {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected text content in result, got: %v", result)
	}
}

// ============================================================================
// Test 4: Tool call - chat_memory_search returns results.
// ============================================================================
func TestMCPServer_ChatMemorySearch(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Add a message with a keyword
	if err := store.AddMessage(sess.ID, memory.RoleUser, "error: connection failed"); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	handler := NewChatMCPServer(proj, store, mgr)
	client := newMCPTestClient(t, handler)
	client.Initialize()

	result := client.CallTool("chat_memory_search", map[string]any{
		"query": "error",
		"limit": 10,
	})

	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array with items, got: %v", result)
	}
}

// ============================================================================
// Test 5: Tool call - error handling with invalid input.
// ============================================================================
func TestMCPServer_InvalidInput_ErrorHandling(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	handler := NewChatMCPServer(proj, store, mgr)
	client := newMCPTestClient(t, handler)
	client.Initialize()

	result := client.CallTool("chat_history_get", map[string]any{
		// missing session_id
		"limit": 10,
	})

	// Check for error in structuredContent or in the text content
	sc, ok := result["structuredContent"].(map[string]any)
	if ok {
		if errMsg, hasError := sc["error"].(string); hasError && errMsg != "" {
			return // test passes
		}
	}

	// Fallback: check content text for error
	content, _ := result["content"].([]any)
	for _, c := range content {
		cmap, _ := c.(map[string]any)
		text, _ := cmap["text"].(string)
		if strings.Contains(text, `"error"`) {
			return // test passes
		}
	}

	t.Errorf("expected error in result, got: %v", result)
}

// ============================================================================
// Test 6: Tool call - chat_file_edit returns pending edit ID.
// ============================================================================
func TestMCPServer_ChatFileEdit(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	sess, err := mgr.GetOrCreateSession("user1")
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// Create a test file in the project
	testFilePath := "test_file.txt"
	if err := storeFileInProject(proj, testFilePath, "initial content"); err != nil {
		t.Fatalf("storeFileInProject: %v", err)
	}

	handler := NewChatMCPServer(proj, store, mgr)
	client := newMCPTestClient(t, handler)
	client.Initialize()

	result := client.CallTool("chat_file_edit", map[string]any{
		"session_id": sess.ID,
		"path":       testFilePath,
		"content":    "new content",
		"reason":     "test edit",
	})

	// Check for edit_id in structuredContent or parsed from text content
	sc, ok := result["structuredContent"].(map[string]any)
	if ok {
		if _, hasEditID := sc["edit_id"]; hasEditID {
			return // test passes
		}
	}

	// Fallback: parse from text content
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("expected content array, got: %v", result)
	}

	cmap := content[0].(map[string]any)
	text, _ := cmap["text"].(string)
	if text == "" {
		t.Fatalf("expected text content, got: %v", content)
	}

	var output map[string]any
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	if _, hasEditID := output["edit_id"]; !hasEditID {
		t.Errorf("expected edit_id in output, got: %v", output)
	}
}

// ============================================================================
// Test 7: Streamable HTTP - two consecutive POST requests complete.
// ============================================================================
func TestMCPServer_StreamableHTTP_ConsecutiveRequests(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	handler := NewChatMCPServer(proj, store, mgr)
	client := newMCPTestClient(t, handler)
	client.Initialize()

	// Send tools/list twice
	client.ToolsList()
	client.ToolsList()
}

// ============================================================================
// Test 8: Concurrent requests - 5 concurrent tool calls complete.
// ============================================================================
func TestMCPServer_ConcurrentRequests(t *testing.T) {
	store, mgr := newTestChatEnv(t)
	proj := newTestProject(t)

	handler := NewChatMCPServer(proj, store, mgr)

	const n = 5
	var wg sync.WaitGroup
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := newMCPTestClient(t, handler)
			client.Initialize()
			_ = client.ToolsList()
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent request failed: %v", err)
	}
}

// ============================================================================
// Helper types and functions
// ============================================================================

type httpError struct {
	id   int
	code int
	body string
	err  error
}

func (e *httpError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "request failed"
}

// storeFileInProject writes content to a file within the project directory.
func storeFileInProject(proj *engine.Project, path, content string) error {
	abs := filepath.Join(proj.DirAbs, path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), 0644)
}

// parseSSEResponse extracts the JSON payload from an SSE-formatted response.
// SSE format are lines like "event: message\ndata: {...}\n\n"
func parseSSEResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()

	s := string(body)

	// If it looks like SSE, extract the data portion
	if strings.HasPrefix(s, "event:") {
		lines := strings.Split(s, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				data = strings.TrimSpace(data)

				var resp map[string]any
				if err := json.Unmarshal([]byte(data), &resp); err != nil {
					t.Fatalf("failed to parse SSE data payload: %v\nRaw: %s", err, data)
				}
				return resp
			}
		}
		t.Fatalf("no data line found in SSE response: %s", s)
	}

	// Plain JSON
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nRaw: %s", err, s)
	}
	return resp
}

// mcpTestClient is a helper that handles MCP initialization and tool calls.
type mcpTestClient struct {
	t         *testing.T
	handler   http.Handler
	sessionID string
}

func newMCPTestClient(t *testing.T, handler http.Handler) *mcpTestClient {
	return &mcpTestClient{t: t, handler: handler}
}

// Initialize sends the MCP initialize request and waits for the response.
func (c *mcpTestClient) Initialize() {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "0.1.0",
			},
		},
	}
	c.do(reqBody)
}

// ToolsList sends a tools/list request and returns the result (not the full response).
func (c *mcpTestClient) ToolsList() map[string]any {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	resp := c.do(reqBody)
	if result, ok := resp["result"].(map[string]any); ok {
		return result
	}
	return resp
}

// CallTool sends a tools/call request and returns the result (not the full response).
func (c *mcpTestClient) CallTool(name string, arguments map[string]any) map[string]any {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	}
	resp := c.do(reqBody)
	// Extract the "result" from the JSON-RPC response
	if result, ok := resp["result"].(map[string]any); ok {
		return result
	}
	return resp
}

func (c *mcpTestClient) do(reqBody map[string]any) map[string]any {
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/mcp/test-project", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	// Echo back the session ID if we have one
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	rec := httptest.NewRecorder()

	c.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		c.t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Capture the session ID from the response headers
	if sid := rec.Header().Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	return parseSSEResponse(c.t, rec.Body.Bytes())
}
