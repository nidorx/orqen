package mcp

import (
	"testing"
)

// ============================================================================
// Test orqen_status
// ============================================================================

func TestStatusHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Set WorkItemID for TASK-0001
	taskModule := proj.GetModule("task")
	for _, lane := range taskModule.Lanes {
		for item := range lane.WorkItems() {
			if item.Seq == 1 {
				item.ID = "test-job-status"
			}
		}
	}

	t.Run("get status", func(t *testing.T) {
		workItemID := "test-job-status"
		input := &StatusInput{WorkItemID: &workItemID}
		out := callHandler(t, StatusHandler, input, proj)

		if !out.Found {
			t.Fatal("expected item to be found")
		}

		if out.ItemID != 1 {
			t.Errorf("item_id = %d, want 1", out.ItemID)
		}

		if out.CurrentLane.Name != "backlog" {
			t.Errorf("current_lane.name = %q, want 'backlog'", out.CurrentLane.Name)
		}

		if out.CurrentLane.Module != "task" {
			t.Errorf("current_lane.module = %q, want 'task'", out.CurrentLane.Module)
		}
	})

	t.Run("unknown job id", func(t *testing.T) {
		workItemID := "nonexistent"
		input := &StatusInput{WorkItemID: &workItemID}
		out := callHandler(t, StatusHandler, input, proj)

		if out.Found {
			t.Error("expected not found")
		}
		if out.Error == "" {
			t.Error("expected error message")
		}
	})

	t.Run("no job id", func(t *testing.T) {
		input := &StatusInput{}
		out := callHandler(t, StatusHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing job id")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		workItemID := "test-job-status"
		input := &StatusInput{WorkItemID: &workItemID}
		out := callHandler(t, StatusHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
