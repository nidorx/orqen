package mcp

import (
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestItemSearchHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Set attributes on test items for filtering
	taskModule := proj.GetModule("task")
	backlog := taskModule.GetLane("backlog")
	var item1 *engine.WorkItem
	_ = item1
	for item := range backlog.WorkItems() {
		if item.Seq == 1 {
			item1 = item
			item.AttributesLoad()
			item.AttributesSave(engine.Attributes{
				"priority": 2,
				"status":   "open",
			})
		}
		if item.Seq == 2 {
			item.AttributesLoad()
			item.AttributesSave(engine.Attributes{
				"priority": 5,
				"status":   "open",
			})
		}
	}

	doing := taskModule.GetLane("doing")
	for item := range doing.WorkItems() {
		if item.Seq == 3 {
			item.AttributesLoad()
			item.AttributesSave(engine.Attributes{
				"priority": 3,
				"status":   "in-progress",
			})
		}
	}

	// Set attributes on ADR items
	adrModule := proj.GetModule("adr")
	draft := adrModule.GetLane("draft")
	for item := range draft.WorkItems() {
		item.AttributesLoad()
		item.AttributesSave(engine.Attributes{
			"status": "draft",
			"type":   "architecture",
		})
	}

	accepted := adrModule.GetLane("accepted")
	for item := range accepted.WorkItems() {
		item.AttributesLoad()
		item.AttributesSave(engine.Attributes{
			"status": "accepted",
			"type":   "architecture",
		})
	}

	t.Run("search all items in module", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Should have 3 task items (1, 2 in backlog, 3 in doing)
		if len(out.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(out.Items))
		}
	})

	t.Run("search items in specific lane", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(out.Items))
		}
	})

	t.Run("search items in empty lane", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
			Lane:   "inbox",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if len(out.Items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(out.Items))
		}
	})

	t.Run("search with condition - filter by priority", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Lane:      "backlog",
			Condition: "priority > 3",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Only item2 has priority > 5
		if len(out.Items) != 1 {
			t.Fatalf("expected 1 item with priority > 3, got %d", len(out.Items))
		}
		if out.Items[0].Seq != 2 {
			t.Errorf("expected item seq 2, got %d", out.Items[0].Seq)
		}
	})

	t.Run("search with condition across all lanes", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Condition: "status == 'open'",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Items 1 and 2 have status=open
		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items with status=open, got %d", len(out.Items))
		}
	})

	t.Run("search with condition in specific lane", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Lane:      "doing",
			Condition: "priority > 2",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Item 3 has priority=3 which is > 2
		if len(out.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(out.Items))
		}
		if out.Items[0].Seq != 3 {
			t.Errorf("expected item seq 3, got %d", out.Items[0].Seq)
		}
	})

	t.Run("search using workitem_id resolution", func(t *testing.T) {
		input := &ItemSearchInput{
			Lane: "backlog",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(out.Items))
		}
	})

	t.Run("search in ADR module", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Should have 2 ADR items (1 in draft, 1 in accepted)
		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(out.Items))
		}
	})

	t.Run("search with condition - type filter", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("adr"),
			Condition: "type == 'architecture'",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Both ADRs have type=architecture
		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(out.Items))
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
		}
		out := callHandler(t, WorkitemSearchHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("nonexistent"),
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("lane not found", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
			Lane:   "nonexistent",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent lane")
		}
	})

	t.Run("invalid condition syntax", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Condition: "invalid syntax here @#$",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Should return an error for invalid condition
		if out.Error == "" {
			t.Error("expected error for invalid condition syntax")
		}
	})

	t.Run("ambiguous module resolution", func(t *testing.T) {
		input := &ItemSearchInput{}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// With multiple modules and no workitem_id, should fail
		if out.Error == "" {
			t.Error("expected error for ambiguous module resolution")
		}
	})

	t.Run("search returns full workitem objects", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if len(out.Items) == 0 {
			t.Fatal("expected at least 1 item")
		}

		// Verify work item has expected fields
		item := out.Items[0]
		if item.ID == "" {
			t.Error("item ID should not be empty")
		}
		if item.Name == "" {
			t.Error("item Name should not be empty")
		}
		if item.Lane == "" {
			t.Error("item Lane should not be nil")
		}
		if item.Attributes == nil {
			t.Error("item Attributes should not be nil")
		}
	})

	t.Run("search with condition that matches nothing", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Condition: "priority > 100",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if len(out.Items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(out.Items))
		}
	})

	t.Run("search with empty condition string uses all items", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Lane:      "backlog",
			Condition: "",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Should return all items in backlog
		if len(out.Items) != 2 {
			t.Fatalf("expected 2 items with empty condition, got %d", len(out.Items))
		}
	})

	t.Run("verify file paths are relative", func(t *testing.T) {
		input := &ItemSearchInput{
			Module: ptr("task"),
			Lane:   "backlog",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		if len(out.Items) == 0 {
			t.Fatal("expected at least 1 item")
		}

		for _, item := range out.Items {
			for _, file := range item.Files {
				// Files should be relative paths
				if len(file) > 0 && file[0] == '/' {
					t.Errorf("file path should be relative, got absolute: %s", file)
				}
			}
		}
	})

	t.Run("search across all lanes with complex condition", func(t *testing.T) {
		input := &ItemSearchInput{
			Module:    ptr("task"),
			Condition: "status == 'in-progress'",
		}
		out := callHandler(t, WorkitemSearchHandler, input, proj)

		// Only item 3 has status=in-progress
		if len(out.Items) != 1 {
			t.Fatalf("expected 1 item with status=in-progress, got %d", len(out.Items))
		}
		if out.Items[0].Seq != 3 {
			t.Errorf("expected item seq 3, got %d", out.Items[0].Seq)
		}
	})
}
