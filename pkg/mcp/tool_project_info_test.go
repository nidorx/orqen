package mcp

import (
	"testing"
)

// ============================================================================
// Test project_info
// ============================================================================

func TestProjectInfoHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("basic project info", func(t *testing.T) {
		input := &ProjectInfoInput{}
		out := callHandler(t, ProjectInfoHandler, input, proj)

		if len(out.Modules) != 2 {
			t.Fatalf("expected 2 modules, got %d", len(out.Modules))
		}

		// Check task module
		var taskMod ModuleSummary
		for _, m := range out.Modules {
			if m.Name == "task" {
				taskMod = m
				break
			}
		}

		if taskMod.Name != "task" {
			t.Errorf("task module not found")
		}

		if len(taskMod.Lanes) != 4 {
			t.Errorf("task module lanes = %d, want 4", len(taskMod.Lanes))
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ProjectInfoInput{}
		out := callHandler(t, ProjectInfoHandler, input, nil)

		if len(out.Modules) != 0 {
			t.Errorf("expected 0 modules for nil project, got %d", len(out.Modules))
		}
	})
}
