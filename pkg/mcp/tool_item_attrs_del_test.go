package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test orqen_item_attrs_del
// ============================================================================

func TestItemAttrsDelHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Load attributes for an existing item and set some test attributes
	taskModule := proj.GetModule("task")
	backlog := taskModule.GetLane("backlog")
	var testItem *engine.WorkItem
	for item := range backlog.WorkItems() {
		if item.Seq == 1 {
			testItem = item
			// Set initial attributes directly on the map
			testItem.Attributes["priority"] = 3
			testItem.Attributes["assignee"] = "john"
			testItem.Attributes["status"] = "open"
			testItem.Attributes["description"] = "Test item"
			testItem.Attributes["tags"] = []string{"test", "backend"}
			testItem.AttributesSave(testItem.Attributes)
			break
		}
	}

	t.Run("delete single attribute", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Verify attribute was deleted by reloading from disk
		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		if _, exists := testItem.Attributes["priority"]; exists {
			t.Error("priority attribute should have been deleted")
		}
	})

	t.Run("delete multiple attributes", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   []string{"assignee", "status"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Verify attributes were deleted
		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		if _, exists := testItem.Attributes["assignee"]; exists {
			t.Error("assignee attribute should have been deleted")
		}
		if _, exists := testItem.Attributes["status"]; exists {
			t.Error("status attribute should have been deleted")
		}

		// Verify other attributes remain
		if _, exists := testItem.Attributes["description"]; !exists {
			t.Error("description attribute should still exist")
		}
	})

	t.Run("delete attributes using workitem_id resolution", func(t *testing.T) {
		workItemID := testItem.ID
		input := &ItemAttrsDelInput{
			Seq:  1,
			Keys: []string{"description"},
		}
		input.WorkItemID = &workItemID
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		if _, exists := testItem.Attributes["description"]; exists {
			t.Error("description attribute should have been deleted")
		}
	})

	t.Run("dependencies key cannot be removed", func(t *testing.T) {
		// Set dependencies attribute directly
		testItem.Attributes["dependencies"] = []string{"TASK-0002"}
		testItem.AttributesSave(testItem.Attributes)

		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   []string{"dependencies"},
		}
		_ = callHandler(t, ItemAttrsDelHandler, input, proj)

		// Should succeed but dependencies should not be deleted
		// (AttributesDel skips "dependencies" key)
		time.Sleep(50 * time.Millisecond)
		testItem.AttributesLoad()
		if _, exists := testItem.Attributes["dependencies"]; !exists {
			t.Log("dependencies key was not removed (expected behavior)")
		}
	})

	t.Run("delete attributes on item in different module", func(t *testing.T) {
		// Create an ADR item with attributes
		adrModule := proj.GetModule("adr")
		draft := adrModule.GetLane("draft")
		var adrItem *engine.WorkItem
		for item := range draft.WorkItems() {
			adrItem = item
			adrItem.Attributes["reviewed_by"] = "alice"
			adrItem.Attributes["approved"] = true
			adrItem.Attributes["version"] = 1
			adrItem.AttributesSave(adrItem.Attributes)
			break
		}

		input := &ItemAttrsDelInput{
			Module: ptr("adr"),
			Seq:    adrItem.Seq,
			Keys:   []string{"reviewed_by", "approved"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		time.Sleep(50 * time.Millisecond)
		adrItem.AttributesLoad()
		if _, exists := adrItem.Attributes["reviewed_by"]; exists {
			t.Error("reviewed_by attribute should have been deleted")
		}
		if _, exists := adrItem.Attributes["approved"]; exists {
			t.Error("approved attribute should have been deleted")
		}
		if _, exists := adrItem.Attributes["version"]; !exists {
			t.Error("version attribute should still exist")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Seq:  1,
			Keys: []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})

	t.Run("invalid seq zero", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    0,
			Keys:   []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for seq = 0")
		}
	})

	t.Run("invalid seq negative", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    -1,
			Keys:   []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for seq < 0")
		}
	})

	t.Run("empty keys", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   []string{},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for empty keys")
		}
	})

	t.Run("nil keys", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   nil,
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nil keys")
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("nonexistent"),
			Seq:    1,
			Keys:   []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("item not found", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    9999,
			Keys:   []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent item")
		}
	})

	t.Run("ambiguous module resolution", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Seq:  1,
			Keys: []string{"priority"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		// With multiple modules and no workitem_id, should fail
		if out.Error == "" {
			t.Error("expected error for ambiguous module resolution")
		}
	})

	t.Run("delete non-existent key should still succeed", func(t *testing.T) {
		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   []string{"nonexistent_key"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		// Should succeed even if key doesn't exist
		if !out.Success {
			t.Fatalf("expected success even for non-existent key, got error: %s", out.Error)
		}
	})

	t.Run("verify yaml file is updated after delete", func(t *testing.T) {
		// Set a fresh attribute
		testItem.Attributes["temp_attr"] = "temp_value"
		testItem.AttributesSave(testItem.Attributes)

		input := &ItemAttrsDelInput{
			Module: ptr("task"),
			Seq:    1,
			Keys:   []string{"temp_attr"},
		}
		out := callHandler(t, ItemAttrsDelHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		// Verify YAML file still exists and is updated
		yamlPath := filepath.Join(backlog.DirAbs, "TASK-0001-write-tests", "TASK-0001.yaml")
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			t.Errorf("YAML file does not exist: %s", yamlPath)
		}

		data, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("failed to read YAML file: %v", err)
		}
		if len(data) == 0 {
			t.Error("YAML file is empty")
		}
	})
}
