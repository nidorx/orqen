package condition

import (
	"regexp"
	"strconv"
	"strings"
)

// evalComparison evaluates a ComparisonExpr against attributes.
func evalComparison(c *ComparisonExpr, attrs map[string]any) bool {
	if attrs == nil {
		attrs = make(map[string]any)
	}
	val, exists := attrs[c.Field]

	switch c.Op {
	case OpExists:
		return exists

	case OpIsNull:
		return !exists || val == nil

	case OpIsNotNull:
		return exists && val != nil
	}

	// For all other ops, field must exist
	if !exists {
		return false
	}

	switch c.Op {
	case OpEq:
		return compareEqual(val, c.Value)
	case OpNe:
		return !compareEqual(val, c.Value)
	case OpGt:
		return compareNumeric(val, c.Value, ">")
	case OpGte:
		return compareNumeric(val, c.Value, ">=")
	case OpLt:
		return compareNumeric(val, c.Value, "<")
	case OpLte:
		return compareNumeric(val, c.Value, "<=")
	case OpLike:
		pattern, ok := c.Value.(string)
		if !ok {
			pattern = toString(c.Value)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false // invalid regex = no match
		}
		return re.MatchString(toString(val))
	case OpIn:
		values, ok := c.Value.([]any)
		if !ok {
			return false
		}
		for _, v := range values {
			if compareEqual(val, v) {
				return true
			}
		}
		return false
	case OpNotIn:
		values, ok := c.Value.([]any)
		if !ok {
			return false
		}
		for _, v := range values {
			if compareEqual(val, v) {
				return false
			}
		}
		return true
	case OpContains:
		return valueContains(val, c.Value)
	case OpPrefix:
		return valuePrefix(val, c.Value)
	case OpSuffix:
		return valueSuffix(val, c.Value)
	case OpBetween:
		bounds, ok := c.Value.([]any)
		if !ok || len(bounds) != 2 {
			return false
		}
		return compareNumeric(val, bounds[0], ">=") && compareNumeric(val, bounds[1], "<=")
	case OpAnyOf:
		// field value (array) has ANY of the specified values
		fieldArr := toSlice(val)
		if len(fieldArr) == 0 {
			return false
		}
		searchVals, ok := c.Value.([]any)
		if !ok {
			return false
		}
		for _, sv := range searchVals {
			for _, fv := range fieldArr {
				if compareEqual(fv, sv) {
					return true
				}
			}
		}
		return false
	case OpAllOf:
		// field value (array) has ALL of the specified values
		fieldArr := toSlice(val)
		if len(fieldArr) == 0 {
			return false
		}
		searchVals, ok := c.Value.([]any)
		if !ok {
			return false
		}
		for _, sv := range searchVals {
			found := false
			for _, fv := range fieldArr {
				if compareEqual(fv, sv) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case OpHasLen:
		fieldArr := toSlice(val)
		expectedLen, ok := c.Value.(int64)
		if !ok {
			return false
		}
		return int64(len(fieldArr)) == expectedLen
	default:
		return false
	}
}

// evalLogical evaluates a LogicalExpr against attributes.
func evalLogical(l *LogicalExpr, attrs map[string]any) bool {
	if attrs == nil {
		attrs = make(map[string]any)
	}

	switch l.Op {
	case OpAnd:
		for _, child := range l.Children {
			if !Evaluate(child, attrs) {
				return false
			}
		}
		return true
	case OpOr:
		for _, child := range l.Children {
			if Evaluate(child, attrs) {
				return true
			}
		}
		return false
	case OpNot:
		if len(l.Children) == 1 {
			return !Evaluate(l.Children[0], attrs)
		}
		return false
	default:
		return false
	}
}

// ---- Comparison helpers ----

// compareEqual checks if two values are equal, with type coercion.
func compareEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Direct comparison for same types
	if a == b {
		return true
	}
	// String comparison
	return toString(a) == toString(b)
}

// compareNumeric compares two numeric values with the given operator.
func compareNumeric(a, b any, op string) bool {
	af := toFloat64(a)
	bf := toFloat64(b)
	switch op {
	case ">":
		return af > bf
	case ">=":
		return af >= bf
	case "<":
		return af < bf
	case "<=":
		return af <= bf
	default:
		return false
	}
}

// toFloat64 converts a value to float64.
func toFloat64(v any) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int8:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint8:
		return float64(val)
	case uint16:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	case float32:
		return float64(val)
	case float64:
		return val
	default:
		return 0
	}
}

// toString converts a value to its string representation.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(formatAny(v))
}

// valueContains checks if a value contains another.
func valueContains(haystack, needle any) bool {
	switch h := haystack.(type) {
	case string:
		return strings.Contains(h, toString(needle))
	case []any:
		for _, item := range h {
			if compareEqual(item, needle) {
				return true
			}
		}
	case []string:
		for _, item := range h {
			if item == toString(needle) {
				return true
			}
		}
	}
	return false
}

// valuePrefix checks if a string starts with a prefix.
func valuePrefix(val, prefix any) bool {
	return strings.HasPrefix(toString(val), toString(prefix))
}

// valueSuffix checks if a string ends with a suffix.
func valueSuffix(val, suffix any) bool {
	return strings.HasSuffix(toString(val), toString(suffix))
}

// toSlice converts a value to []any.
func toSlice(v any) []any {
	switch val := v.(type) {
	case []any:
		return val
	case []string:
		result := make([]any, len(val))
		for i, s := range val {
			result[i] = s
		}
		return result
	default:
		return nil
	}
}

// formatAny formats a value as string.
func formatAny(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return "<unknown>"
	}
}
