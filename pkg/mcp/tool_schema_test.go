package mcp

import (
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test orqen_schema
// ============================================================================

func TestSchemaHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	// Set attributes on ADR items so schema has fields to detect
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

	t.Run("schema for adr module", func(t *testing.T) {
		input := &SchemaInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, ItemAttrSchemaHandler, input, proj)

		if out.Module != "adr" {
			t.Errorf("module = %q, want 'adr'", out.Module)
		}

		// Should have title, status, author fields
		fieldNames := make(map[string]bool)
		for _, f := range out.Fields {
			fieldNames[f.Field] = true
		}

		for _, expected := range []string{"title", "status", "author"} {
			if !fieldNames[expected] {
				t.Errorf("missing field %q", expected)
			}
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &SchemaInput{
			Module: ptr("nonexistent"),
		}
		out := callHandler(t, ItemAttrSchemaHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &SchemaInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, ItemAttrSchemaHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
