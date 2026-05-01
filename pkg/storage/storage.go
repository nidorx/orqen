// Package storage provides markdown file storage with front matter parsing
// and filter operators for module scanning.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// FrontMatterPattern matches YAML front matter bounded by --- lines.
var FrontMatterPattern = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

// Operator defines a filter operator for front matter fields.
type Operator string

const (
	OpEq       Operator = "eq"       // exact match (default when value is not an object)
	OpNe       Operator = "ne"       // not equal
	OpContains Operator = "contains" // string/array contains
	OpExists   Operator = "exists"   // field exists (bool)
	OpIn       Operator = "in"       // value in list
	OpPrefix   Operator = "prefix"   // string starts with
	OpSuffix   Operator = "suffix"   // string ends with
)

// FilterValue represents a filter condition for a front matter field.
type FilterValue struct {
	Op    Operator
	Value any
}

// ParseFilters parses a raw filter map into typed ParsedFilter entries.
// Supports both shorthand {"field": "value"} (eq) and explicit {"field": {"op": "value"}}.
func ParseFilters(raw map[string]any) ([]ParsedFilter, error) {
	var filters []ParsedFilter
	for field, val := range raw {
		if field == "" {
			continue
		}
		pf, err := parseFilterValue(field, val)
		if err != nil {
			return nil, err
		}
		filters = append(filters, pf)
	}
	return filters, nil
}

// ParsedFilter is a filter with its field resolved.
type ParsedFilter struct {
	FilterValue
	Field string
}

func parseFilterValue(field string, val any) (ParsedFilter, error) {
	if m, ok := val.(map[string]any); ok {
		if opRaw, hasOp := m["op"]; hasOp {
			op, err := parseOperator(opRaw)
			if err != nil {
				return ParsedFilter{}, fmt.Errorf("filter %q: invalid operator: %w", field, err)
			}
			return ParsedFilter{
				Field: field,
				FilterValue: FilterValue{
					Op:    op,
					Value: m["value"],
				},
			}, nil
		}
		// Check if it's a single-key map that looks like {"eq": "val"} or {"contains": "val"}
		if len(m) == 1 {
			for k, v := range m {
				op, err := parseOperator(k)
				if err == nil {
					return ParsedFilter{
						Field: field,
						FilterValue: FilterValue{
							Op:    op,
							Value: v,
						},
					}, nil
				}
			}
		}
		// Not an operator map, treat as eq with the whole map as value
		return ParsedFilter{
			Field: field,
			FilterValue: FilterValue{
				Op:    OpEq,
				Value: m,
			},
		}, nil
	}

	// Shorthand: {"field": "value"} means eq
	return ParsedFilter{
		Field: field,
		FilterValue: FilterValue{
			Op:    OpEq,
			Value: val,
		},
	}, nil
}

func parseOperator(raw any) (Operator, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("operator must be a string, got %T", raw)
	}
	switch s {
	case "eq", "==", "=":
		return OpEq, nil
	case "ne", "!=":
		return OpNe, nil
	case "contains":
		return OpContains, nil
	case "exists":
		return OpExists, nil
	case "in":
		return OpIn, nil
	case "prefix":
		return OpPrefix, nil
	case "suffix":
		return OpSuffix, nil
	default:
		return "", fmt.Errorf("unknown operator: %q", s)
	}
}

// MatchedFile represents a markdown file with its parsed front matter.
type MatchedFile struct {
	Path        string         `json:"path"`         // relative path from project root
	Name        string         `json:"name"`         // filename
	FrontMatter map[string]any `json:"front_matter"` // all front matter attributes
	Raw         string         `json:"raw"`          // full file content
	Body        string         `json:"body"`         // content after front matter
}

// SchemaField describes an observed front matter field across a module.
type SchemaField struct {
	Field  string   `json:"field"`  // field name
	Types  []string `json:"types"`  // observed Go/YAML types (string, bool, int, list, map)
	Values []any    `json:"values"` // unique observed values (up to schemaMaxValues)
}

const schemaMaxValues = 50 // max unique values per field in schema

// ScanModule scans a module directory and returns all markdown files
// with their parsed front matter, filtered by the given filters.
// On-demand scan: no caching, reads disk every time.
func ScanModule(dirAbs string, projectDir string, filters map[string]any) ([]MatchedFile, error) {
	parsedFilters, err := ParseFilters(filters)
	if err != nil {
		return nil, err
	}

	var results []MatchedFile

	err = filepath.WalkDir(dirAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext != ".md" && ext != ".txt" {
			return nil
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unread files
		}

		content := string(data)
		frontMatter := parseFrontMatter(content)
		if frontMatter == nil {
			frontMatter = map[string]any{}
		}

		// Apply filters
		if !matchesFilters(frontMatter, parsedFilters) {
			return nil
		}

		rel, err := filepath.Rel(projectDir, filepath.Clean(path))
		if err != nil {
			rel = path
		}

		body := content
		if m := FrontMatterPattern.FindStringSubmatch(content); len(m) > 0 {
			body = strings.TrimSpace(content[len(m[0]):])
		}

		results = append(results, MatchedFile{
			Path:        filepath.ToSlash(rel),
			Name:        d.Name(),
			FrontMatter: frontMatter,
			Raw:         content,
			Body:        body,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by path for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	return results, nil
}

// Schema scans a module directory and returns all observed front matter
// fields with their types and unique values (domains).
func Schema(dirAbs string) ([]SchemaField, error) {
	fieldData := make(map[string]*SchemaField)
	fieldValues := make(map[string]map[string]bool) // track unique values

	err := filepath.WalkDir(dirAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext != ".md" && ext != ".txt" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		frontMatter := parseFrontMatter(string(data))
		if frontMatter == nil {
			return nil
		}

		for field, val := range frontMatter {
			if _, exists := fieldData[field]; !exists {
				fieldData[field] = &SchemaField{Field: field}
				fieldValues[field] = map[string]bool{}
			}

			sd := fieldData[field]
			typeName := yamlTypeName(val)
			if !containsString(sd.Types, typeName) {
				sd.Types = append(sd.Types, typeName)
			}

			valKey := fmt.Sprintf("%v", val)
			if !fieldValues[field][valKey] && len(sd.Values) < schemaMaxValues {
				fieldValues[field][valKey] = true
				sd.Values = append(sd.Values, val)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	var fields []SchemaField
	for _, f := range fieldData {
		fields = append(fields, *f)
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Field < fields[j].Field
	})

	return fields, nil
}

// parseFrontMatter extracts and parses YAML front matter from markdown content.
// Returns nil if no front matter is found.
func parseFrontMatter(content string) map[string]any {
	m := FrontMatterPattern.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(m[1]), &fm); err != nil {
		return nil
	}
	return fm
}

// matchesFilters checks if the front matter matches all filter conditions.
// Filters are AND-ed together.
func matchesFilters(frontMatter map[string]any, filters []ParsedFilter) bool {
	if len(filters) == 0 {
		return true
	}

	for _, f := range filters {
		if !matchField(frontMatter, f.Field, f.FilterValue) {
			return false
		}
	}
	return true
}

func matchField(frontMatter map[string]any, field string, fv FilterValue) bool {
	val, exists := frontMatter[field]

	if fv.Op == OpExists {
		if boolVal, ok := fv.Value.(bool); ok {
			return exists == boolVal
		}
		return exists
	}

	if !exists {
		return false
	}

	switch fv.Op {
	case OpEq:
		return valuesEqual(val, fv.Value)
	case OpNe:
		return !valuesEqual(val, fv.Value)
	case OpContains:
		return valueContains(val, fv.Value)
	case OpIn:
		return valueInList(val, fv.Value)
	case OpPrefix:
		return valuePrefix(val, fv.Value)
	case OpSuffix:
		return valueSuffix(val, fv.Value)
	}

	return false
}

// valuesEqual checks if two values are equal (handles type coercion for numbers).
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// valueContains checks if a string or array contains the given value.
func valueContains(haystack, needle any) bool {
	switch h := haystack.(type) {
	case string:
		return strings.Contains(h, fmt.Sprintf("%v", needle))
	case []any:
		for _, item := range h {
			if valuesEqual(item, needle) {
				return true
			}
		}
	case []string:
		for _, item := range h {
			if valuesEqual(item, needle) {
				return true
			}
		}
	}
	return false
}

// valueInList checks if a value is in a list.
func valueInList(val, list any) bool {
	switch l := list.(type) {
	case []any:
		for _, item := range l {
			if valuesEqual(val, item) {
				return true
			}
		}
	case []string:
		for _, item := range l {
			if valuesEqual(val, item) {
				return true
			}
		}
	}
	return false
}

// valuePrefix checks if a string starts with the given prefix.
func valuePrefix(val, prefix any) bool {
	s, ok := val.(string)
	if !ok {
		s = fmt.Sprintf("%v", val)
	}
	p, ok := prefix.(string)
	if !ok {
		p = fmt.Sprintf("%v", prefix)
	}
	return strings.HasPrefix(s, p)
}

// valueSuffix checks if a string ends with the given suffix.
func valueSuffix(val, suffix any) bool {
	s, ok := val.(string)
	if !ok {
		s = fmt.Sprintf("%v", val)
	}
	p, ok := suffix.(string)
	if !ok {
		p = fmt.Sprintf("%v", suffix)
	}
	return strings.HasSuffix(s, p)
}

// yamlTypeName returns the YAML type name for a value.
func yamlTypeName(val any) string {
	switch val.(type) {
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int"
	case float32, float64:
		return "float"
	case string:
		return "string"
	case []any:
		return "list"
	case map[string]any:
		return "map"
	default:
		return "unknown"
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
