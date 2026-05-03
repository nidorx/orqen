package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Test orqen_dependencies
// ============================================================================

func TestDependenciesHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Add a dependency file to TASK-0001
	taskModule := proj.GetModule("task")
	backlog := taskModule.GetLane("backlog")
	if backlog != nil {
		itemDir := filepath.Join(backlog.DirAbs, "TASK-0001-write-tests")
		os.WriteFile(filepath.Join(itemDir, "DEP_0003"), []byte("depends on TASK-0003"), 0644)
	}

	// Set WorkItemID for TASK-0001
	for _, lane := range taskModule.Lanes {
		for item := range lane.WorkItems() {
			if item.Seq == 1 {
				item.ID = "test-job-001"
			}
		}
	}

	t.Run("check dependencies", func(t *testing.T) {
		workItemID := "test-job-001"
		input := &DependenciesInput{WorkItemID: &workItemID}
		out := callHandler(t, DependenciesHandler, input, proj)

		if out.ItemSeq != 1 {
			t.Errorf("item_id = %d, want 1", out.ItemSeq)
		}

		if len(out.Dependencies) != 1 {
			t.Fatalf("expected 1 dependency, got %d", len(out.Dependencies))
		}

		dep := out.Dependencies[0]
		if dep.DepID != 3 {
			t.Errorf("dep_id = %d, want 3", dep.DepID)
		}

		if dep.ItemName != "TASK-0003-refactor" {
			t.Errorf("item_name = %q, want 'TASK-0003-refactor'", dep.ItemName)
		}

		if dep.Lane != "doing" {
			t.Errorf("lane = %q, want 'doing'", dep.Lane)
		}

		// TASK-0003 is in "doing" lane, so status should be "in_progress"
		if dep.Status != "in_progress" {
			t.Errorf("status = %q, want 'in_progress'", dep.Status)
		}
	})

	t.Run("no job id", func(t *testing.T) {
		input := &DependenciesInput{}
		out := callHandler(t, DependenciesHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing job id")
		}
	})

	t.Run("unknown job id", func(t *testing.T) {
		workItemID := "nonexistent"
		input := &DependenciesInput{WorkItemID: &workItemID}
		out := callHandler(t, DependenciesHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for unknown job id")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		workItemID := "test-job-001"
		input := &DependenciesInput{WorkItemID: &workItemID}
		out := callHandler(t, DependenciesHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
