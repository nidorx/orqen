package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test orqen_item_attrs_set
// ============================================================================

func TestItemAttrsSetHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Load attributes for an existing item to ensure they're populated
	taskModule := proj.GetModule("task")
	backlog := taskModule.GetLane("backlog")
	var testItem *engine.WorkItem
	for item := range backlog.WorkItems() {
		if item.Seq == 1 {
			testItem = item
			break
		}
	}

	t.Run("set new attribute on work item", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    1,
			Attributes: engine.Attributes{
				"priority":    2,
				"assignee":    "john",
				"description": "Implement unit tests",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Verify attributes were saved - wait briefly for file watcher to settle
		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		priority := testItem.Attributes["priority"]
		if priority != uint64(2) && priority != 2 && priority != float64(2) {
			t.Errorf("priority attribute not set correctly, got: %v (type: %T)", priority, priority)
		}
		if testItem.Attributes["assignee"] != "john" {
			t.Errorf("assignee attribute not set correctly, got: %v", testItem.Attributes["assignee"])
		}
	})

	t.Run("update existing attribute", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    1,
			Attributes: engine.Attributes{
				"priority": 5,
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Verify attribute was updated
		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		priority := testItem.Attributes["priority"]
		if priority != uint64(5) && priority != 5 && priority != float64(5) {
			t.Errorf("priority attribute not updated, got: %v (type: %T)", priority, priority)
		}
	})

	t.Run("set attributes using workitem_id resolution", func(t *testing.T) {
		// Set WorkItemID to resolve module
		workItemID := testItem.ID
		input := &ItemAttrsSetInput{
			Seq: 1,
			Attributes: engine.Attributes{
				"status": "in-progress",
			},
		}
		input.WorkItemID = &workItemID
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		if testItem.Attributes["status"] != "in-progress" {
			t.Errorf("status attribute not set, got: %v", testItem.Attributes["status"])
		}
	})

	t.Run("set attributes on item in different module", func(t *testing.T) {
		// Create an ADR item with attributes
		adrModule := proj.GetModule("adr")
		draft := adrModule.GetLane("draft")
		var adrItem *engine.WorkItem
		for item := range draft.WorkItems() {
			adrItem = item
			break
		}

		input := &ItemAttrsSetInput{
			Module: ptr("adr"),
			Seq:    adrItem.Seq,
			Attributes: engine.Attributes{
				"reviewed_by": "alice",
				"approved":    true,
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		time.Sleep(50 * time.Millisecond)
		adrItem.AttributesLoad()
		if adrItem.Attributes["reviewed_by"] != "alice" {
			t.Errorf("reviewed_by attribute not set, got: %v", adrItem.Attributes["reviewed_by"])
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Seq: 1,
			Attributes: engine.Attributes{
				"key": "value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})

	t.Run("invalid seq zero", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    0,
			Attributes: engine.Attributes{
				"key": "value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for seq = 0")
		}
	})

	t.Run("invalid seq negative", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    -1,
			Attributes: engine.Attributes{
				"key": "value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for seq < 0")
		}
	})

	t.Run("empty attributes", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module:     ptr("task"),
			Seq:        1,
			Attributes: engine.Attributes{},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for empty attributes")
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("nonexistent"),
			Seq:    1,
			Attributes: engine.Attributes{
				"key": "value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("item not found", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    9999,
			Attributes: engine.Attributes{
				"key": "value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent item")
		}
	})

	t.Run("ambiguous module resolution", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Seq: 1,
			Attributes: engine.Attributes{
				"key": "value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		// With multiple modules and no workitem_id, should fail
		if out.Error == "" {
			t.Error("expected error for ambiguous module resolution")
		}
	})

	t.Run("set complex attributes with nested structures", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    1,
			Attributes: engine.Attributes{
				"tags":    []string{"backend", "auth", "security"},
				"metrics": map[string]any{"complexity": "high", "lines": 150},
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		if _, ok := testItem.Attributes["tags"]; !ok {
			t.Error("tags attribute not set")
		}
		if _, ok := testItem.Attributes["metrics"]; !ok {
			t.Error("metrics attribute not set")
		}
	})

	t.Run("verify yaml file is created", func(t *testing.T) {
		input := &ItemAttrsSetInput{
			Module: ptr("task"),
			Seq:    1,
			Attributes: engine.Attributes{
				"test_key": "test_value",
			},
		}
		out := callHandler(t, ItemAttrsSetHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Verify YAML file exists
		yamlPath := filepath.Join(backlog.DirAbs, "TASK-0001-write-tests", "TASK-0001.yaml")
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			t.Errorf("YAML file does not exist: %s", yamlPath)
		}

		// Read and verify content
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("failed to read YAML file: %v", err)
		}
		if len(data) == 0 {
			t.Error("YAML file is empty")
		}
	})
}
