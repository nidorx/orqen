package mcp

import (
	"testing"
)

// ============================================================================
// Test orqen_status
// ============================================================================

func TestStatusHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Set WorkItemID for WI-0001
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
		input := &ItemStatusInput{WorkItemID: &workItemID}
		out := callHandler(t, ItemStatusHandler, input, proj)

		if !out.Found {
			t.Fatal("expected item to be found")
		}

		if out.Item.Seq != 1 {
			t.Errorf("item_id = %d, want 1", out.Item.Seq)
		}

		if out.Item.Lane.Name != "backlog" {
			t.Errorf("current_lane.name = %q, want 'backlog'", out.Item.Lane.Name)
		}

		if out.Item.Lane.Module.Name != "task" {
			t.Errorf("current_lane.module = %q, want 'task'", out.Item.Lane.Module.Name)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		workItemID := "nonexistent"
		input := &ItemStatusInput{WorkItemID: &workItemID}
		out := callHandler(t, ItemStatusHandler, input, proj)

		if out.Found {
			t.Error("expected not found")
		}
		if out.Error == "" {
			t.Error("expected error message")
		}
	})

	t.Run("no id", func(t *testing.T) {
		input := &ItemStatusInput{}
		out := callHandler(t, ItemStatusHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing id")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		workItemID := "test-job-status"
		input := &ItemStatusInput{WorkItemID: &workItemID}
		out := callHandler(t, ItemStatusHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
