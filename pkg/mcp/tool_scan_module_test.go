package mcp

import (
	"testing"
)

// ============================================================================
// Test orqen_scan_module
// ============================================================================

func TestScanModuleHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("scan all adr files", func(t *testing.T) {
		input := &ScanModuleInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, ScanModuleHandler, input, proj)

		if out.Module != "adr" {
			t.Errorf("module = %q, want 'adr'", out.Module)
		}

		// We have 2 ADR files, plus HEADER.md and lane prompt files created by load.go
		if out.Count < 2 {
			t.Fatalf("expected at least 2 files, got %d", out.Count)
		}
	})

	t.Run("scan with filter", func(t *testing.T) {
		input := &ScanModuleInput{
			Module: ptr("adr"),
			Filters: map[string]any{
				"status": "accepted",
			},
		}
		out := callHandler(t, ScanModuleHandler, input, proj)

		if out.Count != 1 {
			t.Fatalf("expected 1 file, got %d", out.Count)
		}

		if out.Files[0].Name != "ADR-0002.md" {
			t.Errorf("file name = %q, want 'ADR-0002.md'", out.Files[0].Name)
		}
	})

	t.Run("module not found", func(t *testing.T) {
		input := &ScanModuleInput{
			Module: ptr("nonexistent"),
		}
		out := callHandler(t, ScanModuleHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent module")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ScanModuleInput{
			Module: ptr("adr"),
		}
		out := callHandler(t, ScanModuleHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}
