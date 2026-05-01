package mcp

import (
	"os"
	"testing"
)

// ============================================================================
// Test orqen_next_sequence
// ============================================================================

func TestNextSequenceHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("next sequence for task module", func(t *testing.T) {
		input := &NextSequenceInput{Module: ptr("task")}
		out := callHandler(t, NextSequenceHandler, input, proj)

		if out.Module != "task" {
			t.Errorf("module = %q, want 'task'", out.Module)
		}

		// We have TASK-0001, TASK-0002, TASK-0003
		if out.Current != 3 {
			t.Errorf("current_max = %d, want 3", out.Current)
		}
		if out.Next != 4 {
			t.Errorf("next = %d, want 4", out.Next)
		}
	})

	t.Run("next sequence for adr module", func(t *testing.T) {
		input := &NextSequenceInput{Module: ptr("adr")}
		out := callHandler(t, NextSequenceHandler, input, proj)

		if out.Module != "adr" {
			t.Errorf("module = %q, want 'adr'", out.Module)
		}

		// We have ADR-0001, ADR-0002
		if out.Current != 2 {
			t.Errorf("current_max = %d, want 2", out.Current)
		}
		if out.Next != 3 {
			t.Errorf("next = %d, want 3", out.Next)
		}
	})

	t.Run("empty module", func(t *testing.T) {
		// Load a fresh project with no items
		proj2, _ := setupTestProject(t)
		// Remove all items
		for _, mod := range proj2.Modules {
			for _, lane := range mod.Lanes {
				if lane.DirAbs != "" {
					os.RemoveAll(lane.DirAbs)
				}
			}
		}

		input := &NextSequenceInput{Module: ptr("task")}
		out := callHandler(t, NextSequenceHandler, input, proj2)

		if out.Current != 0 {
			t.Errorf("current_max = %d, want 0", out.Current)
		}
		if out.Next != 1 {
			t.Errorf("next = %d, want 1", out.Next)
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &NextSequenceInput{Module: ptr("nonexistent")}
		out := callHandler(t, NextSequenceHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &NextSequenceInput{Module: ptr("task")}
		out := callHandler(t, NextSequenceHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
