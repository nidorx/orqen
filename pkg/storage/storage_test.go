package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestModule creates a temporary directory with test markdown files.
func createTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// ============================================================================
// Test parseFrontMatter
// ============================================================================

func TestParseFrontMatter(t *testing.T) {
	t.Run("valid front matter", func(t *testing.T) {
		content := `---
title: Test ADR
status: accepted
author: nidorx
---

# Test ADR

Some body content.
`
		fm := parseFrontMatter(content)
		if fm == nil {
			t.Fatal("expected front matter to be parsed")
		}

		if fm["title"] != "Test ADR" {
			t.Errorf("title = %v, want 'Test ADR'", fm["title"])
		}
		if fm["status"] != "accepted" {
			t.Errorf("status = %v, want 'accepted'", fm["status"])
		}
		if fm["author"] != "nidorx" {
			t.Errorf("author = %v, want 'nidorx'", fm["author"])
		}
	})

	t.Run("no front matter", func(t *testing.T) {
		content := `# Just a markdown file

No front matter here.
`
		fm := parseFrontMatter(content)
		if fm != nil {
			t.Errorf("expected nil, got %v", fm)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		content := `---
title: Test
  bad_indent: broken
---
body
`
		fm := parseFrontMatter(content)
		// Should either be nil or have partial parse — both acceptable
		_ = fm
	})

	t.Run("front matter with list", func(t *testing.T) {
		content := `---
title: Multi-tag
tags:
  - go
  - backend
  - api
---

Body
`
		fm := parseFrontMatter(content)
		if fm == nil {
			t.Fatal("expected front matter")
		}

		tags, ok := fm["tags"].([]any)
		if !ok {
			t.Fatalf("tags is not []any, got %T", fm["tags"])
		}
		if len(tags) != 3 {
			t.Errorf("tags length = %d, want 3", len(tags))
		}
	})

	t.Run("front matter with bool", func(t *testing.T) {
		content := `---
title: Test
reviewed: true
priority: 1
---

Body
`
		fm := parseFrontMatter(content)
		if fm == nil {
			t.Fatal("expected front matter")
		}

		if fm["reviewed"] != true {
			t.Errorf("reviewed = %v, want true", fm["reviewed"])
		}
	})
}

// ============================================================================
// Test ParseFilters
// ============================================================================

func TestParseFilters(t *testing.T) {
	t.Run("shorthand eq", func(t *testing.T) {
		raw := map[string]any{
			"status": "accepted",
			"author": "nidorx",
		}

		filters, err := ParseFilters(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(filters) != 2 {
			t.Fatalf("expected 2 filters, got %d", len(filters))
		}

		for _, f := range filters {
			if f.Op != OpEq {
				t.Errorf("filter %q: op = %q, want %q", f.Field, f.Op, OpEq)
			}
		}
	})

	t.Run("explicit operator", func(t *testing.T) {
		raw := map[string]any{
			"status": map[string]any{"op": "eq", "value": "accepted"},
			"tags":   map[string]any{"op": "contains", "value": "go"},
		}

		filters, err := ParseFilters(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, f := range filters {
			switch f.Field {
			case "status":
				if f.Op != OpEq {
					t.Errorf("status op = %q, want %q", f.Op, OpEq)
				}
				if f.Value != "accepted" {
					t.Errorf("status value = %v, want 'accepted'", f.Value)
				}
			case "tags":
				if f.Op != OpContains {
					t.Errorf("tags op = %q, want %q", f.Op, OpContains)
				}
			}
		}
	})

	t.Run("single-key shorthand operator", func(t *testing.T) {
		raw := map[string]any{
			"title":  map[string]any{"prefix": "ADR"},
			"status": map[string]any{"exists": true},
		}

		filters, err := ParseFilters(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, f := range filters {
			switch f.Field {
			case "title":
				if f.Op != OpPrefix {
					t.Errorf("title op = %q, want %q", f.Op, OpPrefix)
				}
			case "status":
				if f.Op != OpExists {
					t.Errorf("status op = %q, want %q", f.Op, OpExists)
				}
			}
		}
	})

	t.Run("unknown operator", func(t *testing.T) {
		raw := map[string]any{
			"field": map[string]any{"op": "unknown_op", "value": "x"},
		}

		_, err := ParseFilters(raw)
		if err == nil {
			t.Fatal("expected error for unknown operator")
		}
	})

	t.Run("in operator", func(t *testing.T) {
		raw := map[string]any{
			"tags": map[string]any{"op": "in", "value": []any{"go", "backend"}},
		}

		filters, err := ParseFilters(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if filters[0].Op != OpIn {
			t.Errorf("op = %q, want %q", filters[0].Op, OpIn)
		}
	})
}

// ============================================================================
// Test matchesFilters
// ============================================================================

func TestMatchesFilters(t *testing.T) {
	fm := map[string]any{
		"title":    "ADR-0001",
		"status":   "accepted",
		"author":   "nidorx",
		"tags":     []any{"go", "backend", "api"},
		"priority": 1,
		"reviewed": true,
	}

	tests := []struct {
		name    string
		filters []ParsedFilter
		want    bool
	}{
		{
			name:    "no filters",
			filters: nil,
			want:    true,
		},
		{
			name:    "eq match",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpEq, Value: "accepted"}}},
			want:    true,
		},
		{
			name:    "eq no match",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpEq, Value: "draft"}}},
			want:    false,
		},
		{
			name:    "ne match",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpNe, Value: "draft"}}},
			want:    true,
		},
		{
			name:    "ne no match",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpNe, Value: "accepted"}}},
			want:    false,
		},
		{
			name:    "contains in string",
			filters: []ParsedFilter{{Field: "title", FilterValue: FilterValue{Op: OpContains, Value: "ADR"}}},
			want:    true,
		},
		{
			name:    "contains in array",
			filters: []ParsedFilter{{Field: "tags", FilterValue: FilterValue{Op: OpContains, Value: "go"}}},
			want:    true,
		},
		{
			name:    "contains no match",
			filters: []ParsedFilter{{Field: "tags", FilterValue: FilterValue{Op: OpContains, Value: "python"}}},
			want:    false,
		},
		{
			name:    "exists true",
			filters: []ParsedFilter{{Field: "reviewed", FilterValue: FilterValue{Op: OpExists, Value: true}}},
			want:    true,
		},
		{
			name:    "exists false",
			filters: []ParsedFilter{{Field: "missing_field", FilterValue: FilterValue{Op: OpExists, Value: false}}},
			want:    true,
		},
		{
			name:    "exists false but exists",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpExists, Value: false}}},
			want:    false,
		},
		{
			name:    "in match",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpIn, Value: []any{"draft", "accepted", "rejected"}}}},
			want:    true,
		},
		{
			name:    "in no match",
			filters: []ParsedFilter{{Field: "status", FilterValue: FilterValue{Op: OpIn, Value: []any{"draft", "rejected"}}}},
			want:    false,
		},
		{
			name:    "prefix match",
			filters: []ParsedFilter{{Field: "title", FilterValue: FilterValue{Op: OpPrefix, Value: "ADR"}}},
			want:    true,
		},
		{
			name:    "prefix no match",
			filters: []ParsedFilter{{Field: "title", FilterValue: FilterValue{Op: OpPrefix, Value: "TASK"}}},
			want:    false,
		},
		{
			name:    "suffix match",
			filters: []ParsedFilter{{Field: "title", FilterValue: FilterValue{Op: OpSuffix, Value: "0001"}}},
			want:    true,
		},
		{
			name:    "suffix no match",
			filters: []ParsedFilter{{Field: "title", FilterValue: FilterValue{Op: OpSuffix, Value: "9999"}}},
			want:    false,
		},
		{
			name: "multiple filters AND-ed",
			filters: []ParsedFilter{
				{Field: "status", FilterValue: FilterValue{Op: OpEq, Value: "accepted"}},
				{Field: "author", FilterValue: FilterValue{Op: OpEq, Value: "nidorx"}},
			},
			want: true,
		},
		{
			name: "multiple filters AND-ed one fails",
			filters: []ParsedFilter{
				{Field: "status", FilterValue: FilterValue{Op: OpEq, Value: "accepted"}},
				{Field: "author", FilterValue: FilterValue{Op: OpEq, Value: "other"}},
			},
			want: false,
		},
		{
			name:    "field not present",
			filters: []ParsedFilter{{Field: "nonexistent", FilterValue: FilterValue{Op: OpEq, Value: "x"}}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilters(fm, tt.filters)
			if got != tt.want {
				t.Errorf("matchesFilters = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Test ScanModule
// ============================================================================

func TestScanModule(t *testing.T) {
	t.Run("scan all files", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"subdir/file1.md": `---
title: File 1
status: accepted
---

Content 1
`,
			"subdir/file2.md": `---
title: File 2
status: draft
---

Content 2
`,
			"subdir/ignored.txt": `---
title: Should not appear
---

Txt files are also scanned.
`,
			"subdir/image.png": "binary", // should be ignored
		})

		results, err := ScanModule(dir, dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// .md and .txt should be scanned
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}

		// Check sorting
		if results[0].Name != "file1.md" {
			t.Errorf("first file = %q, want 'file1.md'", results[0].Name)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"adr/ADR-0001-use-go.md": `---
title: Use Go
status: accepted
---

Content
`,
			"adr/ADR-0002-use-rust.md": `---
title: Use Rust
status: draft
---

Content
`,
			"adr/ADR-0003-use-python.md": `---
title: Use Python
status: accepted
---

Content
`,
		})

		results, err := ScanModule(dir, dir, map[string]any{
			"status": "accepted",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d: %v", len(results), results)
		}

		for _, r := range results {
			if r.FrontMatter["status"] != "accepted" {
				t.Errorf("result %q: status = %v, want 'accepted'", r.Name, r.FrontMatter["status"])
			}
		}
	})

	t.Run("filter with contains operator", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"file1.md": `---
title: Database Migration
status: accepted
---
`,
			"file2.md": `---
title: API Endpoint
status: accepted
---
`,
			"file3.md": `---
title: Database Schema
status: draft
---
`,
		})

		results, err := ScanModule(dir, dir, map[string]any{
			"title": map[string]any{"op": "contains", "value": "Database"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("filter with prefix operator", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"file1.md": `---
title: ADR-0001
---
`,
			"file2.md": `---
title: WI-0001
---
`,
		})

		results, err := ScanModule(dir, dir, map[string]any{
			"title": map[string]any{"op": "prefix", "value": "ADR"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "file1.md" {
			t.Errorf("result name = %q, want 'file1.md'", results[0].Name)
		}
	})

	t.Run("filter with exists operator", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"file1.md": `---
title: Has Author
author: nidorx
---
`,
			"file2.md": `---
title: No Author
---
`,
		})

		results, err := ScanModule(dir, dir, map[string]any{
			"author": map[string]any{"op": "exists", "value": true},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("body extraction", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"file.md": `---
title: Test
---

# Heading

Body paragraph.
`,
		})

		results, err := ScanModule(dir, dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		body := results[0].Body
		if body != "# Heading\n\nBody paragraph." {
			t.Errorf("body = %q, want '# Heading\\n\\nBody paragraph.'", body)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		results, err := ScanModule(dir, dir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

// ============================================================================
// Test Schema
// ============================================================================

func TestSchema(t *testing.T) {
	t.Run("collects all fields and values", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"file1.md": `---
title: First
status: accepted
author: nidorx
---
`,
			"file2.md": `---
title: Second
status: draft
priority: 1
---
`,
			"file3.md": `---
title: Third
status: accepted
reviewed: true
---
`,
		})

		fields, err := Schema(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(fields) == 0 {
			t.Fatal("expected at least one field")
		}

		// Check we got expected fields
		fieldNames := make(map[string]bool)
		for _, f := range fields {
			fieldNames[f.Field] = true
		}

		for _, expected := range []string{"title", "status", "author", "priority", "reviewed"} {
			if !fieldNames[expected] {
				t.Errorf("missing field %q", expected)
			}
		}

		// Check status has multiple values
		for _, f := range fields {
			if f.Field == "status" {
				if len(f.Values) < 2 {
					t.Errorf("status values = %d, want >= 2", len(f.Values))
				}
				// Check types
				found := false
				for _, ty := range f.Types {
					if ty == "string" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("status types = %v, want to include 'string'", f.Types)
				}
			}
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		fields, err := Schema(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fields) != 0 {
			t.Errorf("expected 0 fields, got %d", len(fields))
		}
	})

	t.Run("files without front matter", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"plain.md": "# No front matter\n\nJust content.\n",
		})

		fields, err := Schema(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fields) != 0 {
			t.Errorf("expected 0 fields, got %d", len(fields))
		}
	})

	t.Run("detects different types", func(t *testing.T) {
		dir := createTestModule(t, map[string]string{
			"file1.md": `---
value: hello
---
`,
			"file2.md": `---
value: true
---
`,
			"file3.md": `---
value: 42
---
`,
		})

		fields, err := Schema(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(fields) != 1 {
			t.Fatalf("expected 1 field, got %d", len(fields))
		}

		f := fields[0]
		if f.Field != "value" {
			t.Errorf("field = %q, want 'value'", f.Field)
		}

		if len(f.Types) < 3 {
			t.Errorf("types = %v, want at least 3 types (string, bool, int)", f.Types)
		}
	})
}

// ============================================================================
// Test yamlTypeName
// ============================================================================

func TestYamlTypeName(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"bool", true, "bool"},
		{"int", 42, "int"},
		{"float", 3.14, "float"},
		{"string", "hello", "string"},
		{"list", []any{1, 2}, "list"},
		{"map", map[string]any{"a": 1}, "map"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := yamlTypeName(tt.input)
			if got != tt.want {
				t.Errorf("yamlTypeName(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Test valuesEqual
// ============================================================================

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{"nil nil", nil, nil, true},
		{"nil val", nil, "x", false},
		{"string eq", "hello", "hello", true},
		{"string ne", "hello", "world", false},
		{"int via fmt", "42", 42, true},
		{"bool via fmt", "true", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valuesEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
