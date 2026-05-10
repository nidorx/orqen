package condition

import (
	"testing"
)

// ---- Validate Tests ----

func TestValidate_KnownFields(t *testing.T) {
	node, _ := Parse("name == 'hello' AND age > 30")
	known := map[string]bool{"name": true, "age": true}

	errs := Validate(node, known)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_UnknownFields(t *testing.T) {
	node, _ := Parse("name == 'hello' AND unknown > 30")
	known := map[string]bool{"name": true}

	errs := Validate(node, known)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0] == nil {
		t.Error("expected non-nil error")
	}
}

func TestValidate_MultipleUnknown(t *testing.T) {
	node, _ := Parse("a > 1 AND b < 2 AND c == 'x'")
	known := map[string]bool{"a": true}

	errs := Validate(node, known)
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_EmptyKnown(t *testing.T) {
	node, _ := Parse("name == 'hello'")
	known := map[string]bool{}

	errs := Validate(node, known)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for empty known fields")
	}
}

func TestValidate_NoErrors(t *testing.T) {
	node, _ := Parse("(a > 1 OR b == 'x') AND c EXISTS")
	known := map[string]bool{"a": true, "b": true, "c": true}

	errs := Validate(node, known)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

// ---- ValidateAttrs Tests ----

func TestValidateAttrs_AllPresent(t *testing.T) {
	node, _ := Parse("name == 'hello' AND age > 30")
	attrs := map[string]any{"name": "hello", "age": 35}

	errs := ValidateAttrs(node, attrs)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateAttrs_MissingField(t *testing.T) {
	node, _ := Parse("name == 'hello' AND missing > 30")
	attrs := map[string]any{"name": "hello"}

	errs := ValidateAttrs(node, attrs)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateAttrs_ExistsOpIgnoresMissing(t *testing.T) {
	node, _ := Parse("email EXISTS")
	attrs := map[string]any{"name": "test"}

	errs := ValidateAttrs(node, attrs)
	if len(errs) != 0 {
		t.Errorf("EXISTS should not error on missing field, got %v", errs)
	}
}

func TestValidateAttrs_IsNullIgnoresMissing(t *testing.T) {
	node, _ := Parse("deleted_at IS NULL")
	attrs := map[string]any{"name": "test"}

	errs := ValidateAttrs(node, attrs)
	if len(errs) != 0 {
		t.Errorf("IS NULL should not error on missing field, got %v", errs)
	}
}

func TestValidateAttrs_IsNotNullIgnoresMissing(t *testing.T) {
	node, _ := Parse("deleted_at IS NOT NULL")
	attrs := map[string]any{"name": "test"}

	errs := ValidateAttrs(node, attrs)
	if len(errs) != 0 {
		t.Errorf("IS NOT NULL should not error on missing field, got %v", errs)
	}
}

// ---- FieldRefs Tests ----

func TestFieldRefs_Simple(t *testing.T) {
	node, _ := Parse("name == 'hello' AND age > 30")
	refs := FieldRefs(node)
	expected := []string{"age", "name"} // sorted

	if len(refs) != len(expected) {
		t.Fatalf("expected %d refs, got %d", len(expected), len(refs))
	}
	for i, r := range expected {
		if refs[i] != r {
			t.Errorf("ref[%d] = %q, want %q", i, refs[i], r)
		}
	}
}

func TestFieldRefs_Duplicates(t *testing.T) {
	node, _ := Parse("name == 'a' OR name == 'b'")
	refs := FieldRefs(node)
	if len(refs) != 1 || refs[0] != "name" {
		t.Errorf("expected unique [name], got %v", refs)
	}
}

func TestFieldRefs_Complex(t *testing.T) {
	node, _ := Parse("(a > 1 OR b < 2) AND c == 'x' AND a EXISTS")
	refs := FieldRefs(node)
	expected := []string{"a", "b", "c"}

	if len(refs) != len(expected) {
		t.Fatalf("expected %d refs, got %d", len(expected), len(refs))
	}
	for i, r := range expected {
		if refs[i] != r {
			t.Errorf("ref[%d] = %q, want %q", i, refs[i], r)
		}
	}
}

// ---- String() Canonical Serialization Tests ----

func TestString_ComparisonEq(t *testing.T) {
	node, _ := Parse("name == 'hello'")
	s := node.String()
	expected := "name == 'hello'"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_ComparisonGt(t *testing.T) {
	node, _ := Parse("age > 30")
	s := node.String()
	expected := "age > 30"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_InList(t *testing.T) {
	node, _ := Parse("type IN ('bug', 'feature')")
	s := node.String()
	expected := "type IN ('bug', 'feature')"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_NotIn(t *testing.T) {
	node, _ := Parse("status NOT IN ('done', 'archived')")
	s := node.String()
	expected := "status NOT IN ('done', 'archived')"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Like(t *testing.T) {
	node, _ := Parse("name LIKE '^task-[0-9]+'")
	s := node.String()
	expected := "name LIKE '^task-[0-9]+'"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Between(t *testing.T) {
	node, _ := Parse("age BETWEEN 18 AND 65")
	s := node.String()
	expected := "age BETWEEN 18 AND 65"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Exists(t *testing.T) {
	node, _ := Parse("email EXISTS")
	s := node.String()
	expected := "email EXISTS"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_IsNull(t *testing.T) {
	node, _ := Parse("deleted_at IS NULL")
	s := node.String()
	expected := "deleted_at IS NULL"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_IsNotNull(t *testing.T) {
	node, _ := Parse("deleted_at IS NOT NULL")
	s := node.String()
	expected := "deleted_at IS NOT NULL"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_AndChain(t *testing.T) {
	node, _ := Parse("a == 1 AND b > 2 AND c < 3")
	s := node.String()
	expected := "(a == 1 AND b > 2 AND c < 3)"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_OrChain(t *testing.T) {
	node, _ := Parse("a == 1 OR b == 2")
	s := node.String()
	expected := "(a == 1 OR b == 2)"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Not(t *testing.T) {
	node, _ := Parse("NOT age > 30")
	s := node.String()
	expected := "NOT age > 30"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_NestedNot(t *testing.T) {
	node, _ := Parse("NOT NOT age > 30")
	s := node.String()
	expected := "NOT NOT age > 30"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Prefix(t *testing.T) {
	node, _ := Parse("name PREFIX 'task-'")
	s := node.String()
	expected := "name PREFIX 'task-'"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Suffix(t *testing.T) {
	node, _ := Parse("name SUFFIX '-v2'")
	s := node.String()
	expected := "name SUFFIX '-v2'"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Contains(t *testing.T) {
	node, _ := Parse("tags CONTAINS 'urgent'")
	s := node.String()
	expected := "tags CONTAINS 'urgent'"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_AnyOf(t *testing.T) {
	node, _ := Parse("tags ANY_OF ('urgent', 'critical')")
	s := node.String()
	expected := "tags ANY_OF ('urgent', 'critical')"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_AllOf(t *testing.T) {
	node, _ := Parse("tags ALL_OF ('a', 'b')")
	s := node.String()
	expected := "tags ALL_OF ('a', 'b')"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_HasLength(t *testing.T) {
	node, _ := Parse("tags HAS_LENGTH 3")
	s := node.String()
	expected := "tags HAS_LENGTH 3"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Complex(t *testing.T) {
	input := "name LIKE '^task-' AND (priority > 3 OR urgent == true)"
	node, _ := Parse(input)
	s := node.String()
	// After canonical formatting
	expected := "(name LIKE '^task-' AND (priority > 3 OR urgent == true))"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestString_Ne(t *testing.T) {
	node, _ := Parse("status != 'done'")
	s := node.String()
	expected := "status != 'done'"
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}
