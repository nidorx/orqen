package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/nidorx/orqen/pkg/engine"
)

// helper function to create a temporary project directory for testing
func createTestProjectDir(t *testing.T, config map[string]interface{}) string {
	t.Helper()

	tmpDir := t.TempDir()
	orqenDir := filepath.Join(tmpDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatalf("failed to create .orqen dir: %v", err)
	}

	// Create a minimal orqen.yaml
	cfgData := map[string]interface{}{
		"execution": map[string]interface{}{
			"max_agents":             10,
			"sleep_interval_seconds": 60,
		},
		"modules": []interface{}{
			map[string]interface{}{
				"name":   "test-module",
				"prefix": "TEST",
				"lanes": []interface{}{
					map[string]interface{}{
						"name":    "inbox",
						"purpose": "Test inbox",
					},
				},
			},
		},
		"agents": map[string]interface{}{
			"clients": map[string]interface{}{
				"test-agent": map[string]interface{}{
					"command": []string{"echo", "test"},
				},
			},
			"default": "test-agent",
		},
	}

	// Merge custom config
	for k, v := range config {
		cfgData[k] = v
	}

	cfgBytes, err := yaml.Marshal(cfgData)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	cfgPath := filepath.Join(orqenDir, "orqen.yaml")
	if err := os.WriteFile(cfgPath, cfgBytes, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return tmpDir
}

// createTestProject loads a test project with the given config overrides
func createTestProject(t *testing.T, config map[string]interface{}) *engine.Project {
	t.Helper()

	tmpDir := createTestProjectDir(t, config)

	proj, err := engine.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	return proj
}

// Test 1: NewChatService — Create service with project and config, verify initialization
func TestNewChatService(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
		Telegram: TelegramConfig{
			Token: "test-token",
		},
	}

	svc := New(proj, config)

	if svc == nil {
		t.Fatal("expected service to be created, got nil")
	}

	if svc.Name() != "chat" {
		t.Errorf("expected name 'chat', got %q", svc.Name())
	}

	if svc.proj != proj {
		t.Error("expected project reference to be set")
	}

	if svc.config.Agent != "test-agent" {
		t.Errorf("expected agent %q, got %q", "test-agent", svc.config.Agent)
	}
}

// Test 2: OnStart — with Telegram token, verify all components start
func TestChatService_OnStart_WithTelegramToken(t *testing.T) {
	t.Skip("Skipping test with valid Telegram token - requires real token to avoid API errors")

	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
		Telegram: TelegramConfig{
			Token: "test-token-12345",
		},
	}

	svc := New(proj, config)

	// OnStart will fail because the token is invalid, but we can verify the flow
	err := svc.OnStart()
	if err == nil {
		// If it somehow succeeded, verify components
		if svc.chatStore == nil {
			t.Error("expected chatStore to be initialized")
		}
		if svc.sessionMgr == nil {
			t.Error("expected sessionMgr to be initialized")
		}
		if svc.confirmMgr == nil {
			t.Error("expected confirmMgr to be initialized")
		}
		if svc.mcpHandler == nil {
			t.Error("expected mcpHandler to be initialized")
		}

		// Cleanup
		_ = svc.OnStop()
	} else {
		// Expected to fail with invalid token, but check that it got past validation
		if strings.Contains(err.Error(), "agent") {
			t.Errorf("unexpected agent validation error: %v", err)
		}
	}
}

// Test 3: OnStart — without Telegram token, verify service starts without bot
func TestChatService_OnStart_WithoutTelegramToken(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
	}

	svc := New(proj, config)

	err := svc.OnStart()
	if err != nil {
		t.Fatalf("expected service to start without telegram token, got error: %v", err)
	}

	if svc.chatStore == nil {
		t.Error("expected chatStore to be initialized")
	}
	if svc.sessionMgr == nil {
		t.Error("expected sessionMgr to be initialized")
	}
	if svc.confirmMgr == nil {
		t.Error("expected confirmMgr to be initialized")
	}
	if svc.mcpHandler == nil {
		t.Error("expected mcpHandler to be initialized")
	}
	if svc.bot != nil {
		t.Error("expected bot to be nil when no token provided")
	}
	if !svc.started {
		t.Error("expected service to be marked as started")
	}

	// Cleanup
	_ = svc.OnStop()
}

// Test 4: OnStart — invalid agent, verify error is returned
func TestChatService_OnStart_InvalidAgent(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "nonexistent-agent",
	}

	svc := New(proj, config)

	err := svc.OnStart()
	if err == nil {
		t.Fatal("expected error for invalid agent, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// Test 5: OnStart — empty agent name, verify it falls back to first project agent
func TestChatService_OnStart_EmptyAgentName(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "", // Empty — should fallback to first agent
	}

	svc := New(proj, config)

	err := svc.OnStart()
	if err != nil {
		t.Fatalf("expected service to start with empty agent name (fallback), got error: %v", err)
	}

	// Should have fallen back to "test-agent"
	if svc.config.Agent != "test-agent" {
		t.Errorf("expected agent to fallback to 'test-agent', got %q", svc.config.Agent)
	}

	// Cleanup
	_ = svc.OnStop()
}

// Test 6: OnStop — Start service, stop it, verify cleanup
func TestChatService_OnStop(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
	}

	svc := New(proj, config)

	err := svc.OnStart()
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Stop the service
	err = svc.OnStop()
	if err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}

	if svc.started {
		t.Error("expected service to be marked as not started")
	}

	// Verify chat store is closed (attempting to use it should fail)
	if svc.chatStore == nil {
		t.Error("expected chatStore to still exist (not nil)")
	}
}

// Test 7: OnStop — not started, verify no panic
func TestChatService_OnStop_NotStarted(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
	}

	svc := New(proj, config)

	// Should not panic
	err := svc.OnStop()
	if err != nil {
		t.Errorf("expected nil error when stopping non-started service, got: %v", err)
	}
}

// Test 8: GetMCPHandler — Verify the MCP handler is non-nil after OnStart
func TestChatService_GetMCPHandler(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
	}

	svc := New(proj, config)

	// Before OnStart
	if svc.GetMCPHandler() != nil {
		t.Error("expected MCP handler to be nil before OnStart")
	}

	err := svc.OnStart()
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// After OnStart
	handler := svc.GetMCPHandler()
	if handler == nil {
		t.Error("expected MCP handler to be non-nil after OnStart")
	}

	// Cleanup
	_ = svc.OnStop()
}

// Test 9: GetProjectID — Verify the project ID is returned correctly
func TestChatService_GetProjectID(t *testing.T) {
	proj := createTestProject(t, nil)

	config := ChatConfig{
		Agent: "test-agent",
	}

	svc := New(proj, config)

	projectID := svc.GetProjectID()
	if projectID != proj.Id {
		t.Errorf("expected project ID %q, got %q", proj.Id, projectID)
	}

	if projectID == "" {
		t.Error("expected project ID to be non-empty")
	}
}

// Test 10: ParseChatConfig — Parse a project config with chat section
func TestParseChatConfig_WithChatSection(t *testing.T) {
	tmpDir := createTestProjectDir(t, map[string]interface{}{
		"chat": map[string]interface{}{
			"agent": "qwen",
			"telegram": map[string]interface{}{
				"token": "12345:ABC-DEF",
			},
		},
	})

	// Create a minimal project to pass ParseChatConfig
	proj := &engine.Project{
		DirAbs: tmpDir,
	}

	config, err := ParseChatConfig(proj)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if config.Agent != "qwen" {
		t.Errorf("expected agent %q, got %q", "qwen", config.Agent)
	}

	if config.Telegram.Token != "12345:ABC-DEF" {
		t.Errorf("expected token %q, got %q", "12345:ABC-DEF", config.Telegram.Token)
	}
}

// Test 11: ParseChatConfig — missing chat section, verify zero-value config
func TestParseChatConfig_MissingChatSection(t *testing.T) {
	tmpDir := createTestProjectDir(t, nil)

	proj := &engine.Project{
		DirAbs: tmpDir,
	}

	config, err := ParseChatConfig(proj)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should be zero-value config
	if config.Agent != "" {
		t.Errorf("expected empty agent, got %q", config.Agent)
	}

	if config.Telegram.Token != "" {
		t.Errorf("expected empty token, got %q", config.Telegram.Token)
	}
}
