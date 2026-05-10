package engine

import (
	"fmt"
	"os"
	"sort"

	"github.com/goccy/go-yaml"
)

// Attributes represents a set of key-value pairs attached to a WorkItem.
// It is stored as YAML on disk and used for querying and filtering work items.
type Attributes map[string]any

// Get retrieves a value by key, returning the value and whether it exists.
func (attrs Attributes) Get(key string) (any, bool) {
	v, ok := attrs[key]
	return v, ok
}

// Set sets a key-value pair in the attributes.
func (attrs Attributes) Set(key string, value any) {
	attrs[key] = value
}

// Has returns true if the key exists in the attributes.
func (attrs Attributes) Has(key string) bool {
	_, ok := attrs[key]
	return ok
}

// Delete removes a key from the attributes.
func (attrs Attributes) Delete(key string) {
	delete(attrs, key)
}

// Keys returns a sorted slice of all keys in the attributes.
func (attrs Attributes) Keys() []string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Merge merges another set of attributes into this one.
// Existing keys are overwritten by the other attributes.
func (attrs Attributes) Merge(other Attributes) {
	for k, v := range other {
		if v == nil || v == "" {
			continue
		}
		attrs[k] = v
	}
}

// String retrieves a value as a string. Returns empty string if the key
// does not exist or the value is not a string.
func (attrs Attributes) String(key string) string {
	if v, exists := attrs[key]; exists {
		if s, ok := v.(string); ok {
			return s
		}
		// Fallback: convert to string representation
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// Int retrieves a value as an int64. Returns 0 if the key does not exist
// or the value cannot be converted to an integer.
func (attrs Attributes) Int(key string) int64 {
	if v, exists := attrs[key]; exists {
		switch val := v.(type) {
		case int:
			return int64(val)
		case int8:
			return int64(val)
		case int16:
			return int64(val)
		case int32:
			return int64(val)
		case int64:
			return val
		case uint:
			return int64(val)
		case uint8:
			return int64(val)
		case uint16:
			return int64(val)
		case uint32:
			return int64(val)
		case uint64:
			return int64(val)
		case float32:
			return int64(val)
		case float64:
			return int64(val)
		}
	}
	return 0
}

// Float retrieves a value as a float64. Returns 0 if the key does not exist
// or the value cannot be converted to a float.
func (attrs Attributes) Float(key string) float64 {
	if v, exists := attrs[key]; exists {
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
		}
	}
	return 0
}

// Bool retrieves a value as a bool. Returns false if the key does not exist
// or the value cannot be converted to a bool.
func (attrs Attributes) Bool(key string) bool {
	if v, exists := attrs[key]; exists {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// StringArray retrieves a value as a []string. Returns nil if the key does
// not exist or the value cannot be converted to a string slice.
func (attrs Attributes) StringArray(key string) []string {
	if v, exists := attrs[key]; exists {
		switch val := v.(type) {
		case []string:
			return val
		case []any:
			result := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					result = append(result, s)
				} else {
					result = append(result, fmt.Sprintf("%v", item))
				}
			}
			return result
		}
	}
	return nil
}

// LoadFromYAML reads a YAML file from disk and populates the attributes.
// If the file does not exist or cannot be read, an error is returned.
// Existing attributes are overwritten.
func (attrs Attributes) LoadFromYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read yaml file %q: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse yaml file %q: %w", path, err)
	}

	// Clear existing attributes
	for k := range attrs {
		delete(attrs, k)
	}

	for k, v := range raw {
		attrs[k] = v
	}

	return nil
}

// SaveToYAML writes the attributes to a YAML file on disk.
// The file is created or overwritten.
func (attrs Attributes) SaveToYAML(path string) error {
	data, err := yaml.Marshal(map[string]any(attrs))
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write yaml file %q: %w", path, err)
	}

	return nil
}
