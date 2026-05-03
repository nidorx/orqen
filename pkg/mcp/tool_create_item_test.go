package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Test orqen_create_item
// ============================================================================

func TestCreateItemHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("create task in backlog", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "implement-auth",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		if out.ModuleType != "TASK" {
			t.Errorf("module_type = %q, want 'TASK'", out.ModuleType)
		}

		// Item ID should be 4 (we have 0001, 0002, 0003)
		if out.ItemSeq != 4 {
			t.Errorf("item_id = %d, want 4", out.ItemSeq)
		}

		// Directory name
		if out.ItemName != "TASK-0004-implement-auth" {
			t.Errorf("item_name = %q, want 'TASK-0004-implement-auth'", out.ItemName)
		}

		// Verify directory exists
		fullDir := filepath.Join(proj.DirAbs, out.DirPath)
		if _, err := os.Stat(fullDir); os.IsNotExist(err) {
			t.Errorf("directory does not exist: %s", fullDir)
		}

		// Verify file exists
		fullFile := filepath.Join(proj.DirAbs, out.FilePath)
		if _, err := os.Stat(fullFile); os.IsNotExist(err) {
			t.Errorf("file does not exist: %s", fullFile)
		}

		// File should be empty
		data, _ := os.ReadFile(fullFile)
		if len(data) != 0 {
			t.Errorf("file should be empty, got %d bytes", len(data))
		}
	})

	t.Run("create adr in draft", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("adr"),
			Lane:       "draft",
			SimpleName: "use-redis-cache",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		if out.ItemName != "ADR-0003-use-redis-cache" {
			t.Errorf("item_name = %q, want 'ADR-0003-use-redis-cache'", out.ItemName)
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("nonexistent"),
			Lane:       "backlog",
			SimpleName: "test",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("lane not found", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("task"),
			Lane:       "nonexistent",
			SimpleName: "test",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent lane")
		}
	})

	t.Run("invalid kebab-case name", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "Invalid Name With Spaces",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for invalid kebab-case name")
		}
	})

	t.Run("uppercase name should be lowercased", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "MY-FEATURE",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// The name should be lowercased
		if out.ItemName != "TASK-0005-my-feature" {
			t.Errorf("item_name = %q, want 'TASK-0005-my-feature'", out.ItemName)
		}
	})

	t.Run("missing module", func(t *testing.T) {
		input := &CreateItemInput{
			Lane:       "backlog",
			SimpleName: "test",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		// Should resolve from single module or fail if ambiguous
		// With 2 modules, should fail without explicit module
		if out.Error == "" && len(proj.Modules) > 1 {
			t.Error("expected error for ambiguous module resolution")
		}
	})

	t.Run("missing lane", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("task"),
			SimpleName: "test",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing lane")
		}
	})

	t.Run("missing simple name", func(t *testing.T) {
		input := &CreateItemInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, CreateItemHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing simple_name")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &CreateItemInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "test",
		}
		out := callHandler(t, CreateItemHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
