package mcp

import (
	"testing"
)

// ============================================================================
// Test orqen_list_items
// ============================================================================

func TestListItemsHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("list backlog items", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, ListItemsHandler, input, proj)

		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(out.Items))
		}
	})

	t.Run("list doing items", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("task"),
			Lane:   "doing",
		}
		out := callHandler(t, ListItemsHandler, input, proj)

		if len(out.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(out.Items))
		}
	})

	t.Run("empty lane", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("task"),
			Lane:   "inbox",
		}
		out := callHandler(t, ListItemsHandler, input, proj)

		if len(out.Items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(out.Items))
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("nonexistent"),
			Lane:   "backlog",
		}
		out := callHandler(t, ListItemsHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("lane not found", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("task"),
			Lane:   "nonexistent",
		}
		out := callHandler(t, ListItemsHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent lane")
		}
	})

	t.Run("missing lane", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("task"),
		}
		out := callHandler(t, ListItemsHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing lane")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ListItemsInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, ListItemsHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
