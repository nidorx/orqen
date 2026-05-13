package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
)

// mockHandler is a simple http.Handler that returns a fixed response.
type mockHandler struct {
	response string
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(m.response))
}

// setupTestService creates a Service with an in-memory mux for testing.
// It does NOT start the actual HTTP server.
func setupTestService() *Service {
	// Ensure conf.GetHttpServer returns a valid config
	if conf.GetHttpServer() == nil {
		conf.SetHttpServer(conf.HttpServer{
			IP:           "127.0.0.1",
			Port:         0,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		})
	}
	svc := New()
	// Reset chatMCPs for test isolation (New() already initializes it, but be explicit)
	svc.chatMCPs = make(map[string]http.Handler)
	return svc
}

// newTestProject creates a temporary project in a temp directory, registers it
// with the engine, and returns a cleanup function.
func newTestProject(t *testing.T) (*engine.Project, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	orqenDir := filepath.Join(tmpDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatalf("failed to create .orqen dir: %v", err)
	}

	// Minimal orqen.yaml with one module and one lane
	config := `
execution:
  maxAgents: 5
  sleepIntervalSeconds: 60
modules:
  - name: test
    lanes:
      - name: inbox
        purpose: "Test lane"
        agentBehavior:
          - "Do something"
agents:
  clients:
    test:
      command: ["echo", "test"]
`
	configPath := filepath.Join(orqenDir, "orqen.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	proj, err := engine.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	cleanup := func() {
		// Unregister from global projects map
		engine.Unregister(proj.Id)
	}

	return proj, cleanup
}

// serveChatMCP simulates the chat MCP handler dispatch for a given projectID,
// bypassing engine.Get() by directly invoking the registered handler.
func serveChatMCP(svc *Service, projectID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/chat/mcp/"+projectID, nil)
	rec := httptest.NewRecorder()

	handler := svc.getChatMCPHandler(projectID)
	if handler != nil {
		handler.ServeHTTP(rec, req)
	} else {
		http.Error(rec, "chat MCP not available", http.StatusNotFound)
	}

	return rec
}

func TestExtractProjectIDFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple project ID", "/chat/mcp/abc123", "abc123"},
		{"project ID with trailing path", "/chat/mcp/abc123/extra/path", "abc123"},
		{"project ID with query-like suffix", "/chat/mcp/proj-001?foo=bar", "proj-001"},
		{"empty after prefix", "/chat/mcp/", ""},
		{"just prefix", "/chat/mcp", ""},
		{"no prefix", "/other/path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractProjectIDFromPath(tt.path)
			if got != tt.expected {
				t.Errorf("extractProjectIDFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestChatMCPRoute_ProjectNotFound(t *testing.T) {
	svc := setupTestService()

	req := httptest.NewRequest(http.MethodPost, "/chat/mcp/nonexistent", nil)
	rec := httptest.NewRecorder()

	svc.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "project not found") {
		t.Errorf("expected 'project not found' in body, got: %s", string(body))
	}
}

func TestChatMCPRoute_ChatMCPNotRegistered(t *testing.T) {
	// We need a project to exist but no chat handler registered for it.
	svc := setupTestService()

	// No handler registered for "testproj"
	handler := svc.getChatMCPHandler("testproj")
	if handler != nil {
		t.Error("expected nil handler for unregistered project")
	}
}

func TestChatMCPRoute_HandlerRegistered(t *testing.T) {
	svc := setupTestService()

	mock := &mockHandler{response: "chat mcp ok"}
	svc.RegisterChatMCP("proj1", mock)

	// Verify lookup
	handler := svc.getChatMCPHandler("proj1")
	if handler == nil {
		t.Fatal("expected handler to be registered")
	}

	// Test via HTTP request (simulate without project lookup)
	req := httptest.NewRequest(http.MethodPost, "/chat/mcp/proj1", nil)
	rec := httptest.NewRecorder()

	// Manually invoke the handler to bypass engine.Get() which needs real project
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != "chat mcp ok" {
		t.Errorf("expected 'chat mcp ok', got: %s", string(body))
	}
}

func TestChatMCPRoute_ConcurrentRegistration(t *testing.T) {
	svc := setupTestService()

	var wg sync.WaitGroup
	numHandlers := 10

	for i := 0; i < numHandlers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			projectID := string(rune('A' + id))
			svc.RegisterChatMCP(projectID, &mockHandler{response: projectID})
		}(i)
	}

	wg.Wait()

	// Verify all handlers are registered
	for i := 0; i < numHandlers; i++ {
		projectID := string(rune('A' + i))
		handler := svc.getChatMCPHandler(projectID)
		if handler == nil {
			t.Errorf("expected handler for project %q after concurrent registration", projectID)
		}
	}
}

func TestExistingMCPHttpRoute_Unaffected(t *testing.T) {
	// Verify the existing /mcp/http/ route still works.
	// This tests that our changes didn't break the original handler.
	svc := setupTestService()

	req := httptest.NewRequest(http.MethodPost, "/mcp/http/", nil)
	rec := httptest.NewRecorder()

	svc.mux.ServeHTTP(rec, req)

	// Should NOT be 404 (route exists) — it will be 400 because of missing project_id
	if rec.Code == http.StatusNotFound {
		t.Error("expected /mcp/http/ route to still be accessible, but got 404")
	}

	// Should return 400 for missing project_id
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing project_id, got %d", rec.Code)
	}
}

// TestChatMCPRoute_FullFlowWithProject tests the full route dispatch when a project
// is registered with the engine and a chat handler is registered with the HTTP service.
func TestChatMCPRoute_FullFlowWithProject(t *testing.T) {
	proj, cleanup := newTestProject(t)
	defer cleanup()

	svc := setupTestService()

	mock := &mockHandler{response: "full flow ok"}
	svc.RegisterChatMCP(proj.Id, mock)

	// Test via the full mux (this exercises the route handler that calls engine.Get())
	req := httptest.NewRequest(http.MethodPost, "/chat/mcp/"+proj.Id, nil)
	rec := httptest.NewRecorder()

	svc.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != "full flow ok" {
		t.Errorf("expected 'full flow ok', got: %s", string(body))
	}
}
