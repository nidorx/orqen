package mcp

import (
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test orqen_status
// ============================================================================

func TestStatusHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Get the actual work item from backlog lane (seq 1)
	taskModule := proj.GetModule("task")
	backlog := taskModule.GetLane("backlog")
	var item1 *engine.WorkItem
	for item := range backlog.WorkItems() {
		if item.Seq == 1 {
			item1 = item
			break
		}
	}

	t.Run("get status", func(t *testing.T) {
		if item1 == nil {
			t.Fatal("item1 not found")
		}

		workItemID := item1.ID
		input := &ItemStatusInput{WorkItemID: &workItemID}
		out := callHandler(t, ItemStatusHandler, input, proj)

		if !out.Found {
			t.Fatal("expected item to be found")
		}

		if out.Item.Seq != 1 {
			t.Errorf("item_id = %d, want 1", out.Item.Seq)
		}

		if out.Item.Lane != "backlog" {
			t.Errorf("current_lane.name = %q, want 'backlog'", out.Item.Lane)
		}

		if out.Item.Module != "task" {
			t.Errorf("current_lane.module = %q, want 'task'", out.Item.Module)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		// Use a hash that doesn't exist
		workItemID := "nonexistent-id-12345"
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
		workItemID := "some-id"
		input := &ItemStatusInput{WorkItemID: &workItemID}
		out := callHandler(t, ItemStatusHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
