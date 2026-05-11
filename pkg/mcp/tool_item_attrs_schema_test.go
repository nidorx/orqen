package mcp

import (
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test orqen_item_attrs_schema
// ============================================================================

func TestItemAttrSchemaHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Set attributes on test items so schema has fields to detect
	taskModule := proj.GetModule("task")
	backlog := taskModule.GetLane("backlog")
	for item := range backlog.WorkItems() {
		item.AttributesLoad()
		if item.Seq == 1 {
			item.AttributesSave(engine.Attributes{
				"title":    "Write Tests",
				"priority": 2,
			})
		}
		if item.Seq == 2 {
			item.AttributesSave(engine.Attributes{
				"title":    "Add Feature",
				"priority": 5,
			})
		}
	}

	doing := taskModule.GetLane("doing")
	for item := range doing.WorkItems() {
		item.AttributesLoad()
		item.AttributesSave(engine.Attributes{
			"title":    "Refactor",
			"priority": 3,
		})
	}

	// Set attributes on ADR items
	adrModule := proj.GetModule("adr")
	draft := adrModule.GetLane("draft")
	for item := range draft.WorkItems() {
		item.AttributesLoad()
		item.AttributesSave(engine.Attributes{
			"title":  "Use Go",
			"status": "draft",
		})
	}

	accepted := adrModule.GetLane("accepted")
	for item := range accepted.WorkItems() {
		item.AttributesLoad()
		item.AttributesSave(engine.Attributes{
			"title":  "Use PostgreSQL",
			"status": "accepted",
			"author": "nidorx",
		})
	}

	t.Run("schema for task module", func(t *testing.T) {
		input := &SchemaInput{Module: ptr("task")}
		out := callHandler(t, ItemAttrSchemaHandler, input, proj)

		if out.Module != "task" {
			t.Errorf("module = %q, want 'task'", out.Module)
		}

		// Schema should have at least one field (title from created items)
		if len(out.Fields) == 0 {
			t.Error("expected at least one schema field for task module")
		}

		// Check that 'title' field exists
		foundTitle := false
		for _, field := range out.Fields {
			if field.Field == "title" {
				foundTitle = true
				break
			}
		}
		if !foundTitle {
			t.Error("expected 'title' field in schema")
		}
	})

	t.Run("schema for adr module", func(t *testing.T) {
		input := &SchemaInput{Module: ptr("adr")}
		out := callHandler(t, ItemAttrSchemaHandler, input, proj)

		if out.Module != "adr" {
			t.Errorf("module = %q, want 'adr'", out.Module)
		}

		if len(out.Fields) == 0 {
			t.Error("expected at least one schema field for adr module")
		}

		// ADR items should have title, status, and author fields
		foundStatus := false
		foundAuthor := false
		for _, field := range out.Fields {
			if field.Field == "status" {
				foundStatus = true
			}
			if field.Field == "author" {
				foundAuthor = true
			}
		}
		if !foundStatus {
			t.Error("expected 'status' field in ADR schema")
		}
		if !foundAuthor {
			t.Error("expected 'author' field in ADR schema")
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &SchemaInput{Module: ptr("nonexistent")}
		out := callHandler(t, ItemAttrSchemaHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &SchemaInput{Module: ptr("task")}
		out := callHandler(t, ItemAttrSchemaHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})

	t.Run("empty module parameter", func(t *testing.T) {
		// When module is nil/empty, it should try to resolve from workitem_id
		// Since we don't have a workitem_id set, it should fail
		input := &SchemaInput{}
		out := callHandler(t, ItemAttrSchemaHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error when module cannot be resolved")
		}
	})
}
