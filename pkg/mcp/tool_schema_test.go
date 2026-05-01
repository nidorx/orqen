package mcp

import (
	"testing"
)

// ============================================================================
// Test orqen_schema
// ============================================================================

func TestSchemaHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("schema for adr module", func(t *testing.T) {
		input := &SchemaInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, SchemaHandler, input, proj)

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
		out := callHandler(t, SchemaHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &SchemaInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, SchemaHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
