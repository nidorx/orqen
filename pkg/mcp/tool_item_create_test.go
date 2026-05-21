package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Test orqen_item_create
// ============================================================================

func TestCreateItemHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("create task in backlog", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "implement-auth",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Item ID should be 4 (we have 0001, 0002, 0003)
		if out.WorkItem.Seq != 4 {
			t.Errorf("item_id = %d, want 4", out.WorkItem.Seq)
		}

		// Directory name
		if out.WorkItem.Name != "TASK-0004-implement-auth" {
			t.Errorf("item_name = %q, want 'TASK-0004-implement-auth'", out.WorkItem.Name)
		}

		// Verify directory exists
		fullDir := filepath.Join(out.WorkItem.Lane, out.WorkItem.Name)
		if _, err := os.Stat(fullDir); os.IsNotExist(err) {
			t.Errorf("directory does not exist: %s", fullDir)
		}

		// Verify file exists
		fullFile := filepath.Join(fullDir, "TASK-0004.yaml")
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
		input := &ItemCreateInput{
			Module:     ptr("adr"),
			Lane:       "draft",
			SimpleName: "use-redis-cache",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		if out.WorkItem.Name != "ADR-0003-use-redis-cache" {
			t.Errorf("item_name = %q, want 'ADR-0003-use-redis-cache'", out.WorkItem.Name)
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("nonexistent"),
			Lane:       "backlog",
			SimpleName: "test",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("lane not found", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("task"),
			Lane:       "nonexistent",
			SimpleName: "test",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent lane")
		}
	})

	t.Run("invalid kebab-case name", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "Invalid Name With Spaces",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for invalid kebab-case name")
		}
	})

	t.Run("uppercase name should be lowercased", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "MY-FEATURE",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// The name should be lowercased (seq depends on existing items, just check the pattern)
		if out.WorkItem.Seq <= 0 {
			t.Errorf("item_seq = %d, want positive number", out.WorkItem.Seq)
		}
		if !strings.Contains(out.WorkItem.Name, "my-feature") {
			t.Errorf("item_name = %q, should contain 'my-feature'", out.WorkItem.Name)
		}
		if !strings.HasPrefix(out.WorkItem.Name, "TASK-") {
			t.Errorf("item_name = %q, should start with 'TASK-'", out.WorkItem.Name)
		}
	})

	t.Run("missing module", func(t *testing.T) {
		input := &ItemCreateInput{
			Lane:       "backlog",
			SimpleName: "test",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		// Should resolve from single module or fail if ambiguous
		// With 2 modules, should fail without explicit module
		if out.Error == "" && len(proj.Modules) > 1 {
			t.Error("expected error for ambiguous module resolution")
		}
	})

	t.Run("missing lane", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("task"),
			SimpleName: "test",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing lane")
		}
	})

	t.Run("missing simple name", func(t *testing.T) {
		input := &ItemCreateInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, ItemCreateHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing simple_name")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ItemCreateInput{
			Module:     ptr("task"),
			Lane:       "backlog",
			SimpleName: "test",
		}
		out := callHandler(t, ItemCreateHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
