package mcp

import (
	"testing"
)

// ============================================================================
// Test orqen_list_lanes
// ============================================================================

func TestListLanesHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("list lanes for task module", func(t *testing.T) {
		input := &ListLanesInput{Module: ptr("task")}
		out := callHandler(t, ListLanesHandler, input, proj)

		if out.Module != "task" {
			t.Errorf("module = %q, want 'task'", out.Module)
		}

		if len(out.Lanes) != 4 {
			t.Fatalf("expected 4 lanes, got %d", len(out.Lanes))
		}

		// Check first lane
		if out.Lanes[0].Name != "inbox" {
			t.Errorf("first lane = %q, want 'inbox'", out.Lanes[0].Name)
		}
	})

	t.Run("list lanes for adr module", func(t *testing.T) {
		input := &ListLanesInput{Module: ptr("adr")}
		out := callHandler(t, ListLanesHandler, input, proj)

		if out.Module != "adr" {
			t.Errorf("module = %q, want 'adr'", out.Module)
		}

		// load.go auto-creates an "inbox" lane if none exists
		if len(out.Lanes) < 2 {
			t.Fatalf("expected at least 2 lanes, got %d", len(out.Lanes))
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ListLanesInput{Module: ptr("nonexistent")}
		out := callHandler(t, ListLanesHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ListLanesInput{Module: ptr("task")}
		out := callHandler(t, ListLanesHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
