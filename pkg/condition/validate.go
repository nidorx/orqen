package condition

import (
	"fmt"
	"sort"
)

// Validate checks that every field name referenced in the condition AST
// exists in the provided schema (a set of known field names).
//
// Returns a list of errors for each unknown field. An empty slice means valid.
func Validate(node ExprNode, knownFields map[string]bool) []error {
	var errs []error
	collectFieldErrors(node, knownFields, &errs)
	return errs
}

// ValidateAttrs checks that every field name referenced in the condition AST
// actually exists in the given attributes map (non-nil check only).
// This is useful for detecting conditions that reference fields that will
// never match (e.g., typo in a field name).
func ValidateAttrs(node ExprNode, attrs map[string]any) []error {
	var errs []error
	collectMissingFields(node, attrs, &errs)
	return errs
}

func collectFieldErrors(node ExprNode, knownFields map[string]bool, errs *[]error) {
	switch n := node.(type) {
	case *ComparisonExpr:
		if _, ok := knownFields[n.Field]; !ok {
			*errs = append(*errs, fmt.Errorf("unknown field %q in expression: %s", n.Field, n.String()))
		}
	case *LogicalExpr:
		for _, child := range n.Children {
			collectFieldErrors(child, knownFields, errs)
		}
	}
}

func collectMissingFields(node ExprNode, attrs map[string]any, errs *[]error) {
	switch n := node.(type) {
	case *ComparisonExpr:
		// EXISTS and IS NULL / IS NOT NULL are valid even if field doesn't exist
		if n.Op == OpExists || n.Op == OpIsNull || n.Op == OpIsNotNull {
			return
		}
		if _, ok := attrs[n.Field]; !ok {
			*errs = append(*errs, fmt.Errorf("field %q not found in attributes", n.Field))
		}
	case *LogicalExpr:
		for _, child := range n.Children {
			collectMissingFields(child, attrs, errs)
		}
	}
}

// FieldRefs returns all unique field names referenced in the condition AST,
// sorted alphabetically.
func FieldRefs(node ExprNode) []string {
	refs := make(map[string]bool)
	collectFields(node, refs)

	result := make([]string, 0, len(refs))
	for k := range refs {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func collectFields(node ExprNode, refs map[string]bool) {
	switch n := node.(type) {
	case *ComparisonExpr:
		refs[n.Field] = true
	case *LogicalExpr:
		for _, child := range n.Children {
			collectFields(child, refs)
		}
	}
}
