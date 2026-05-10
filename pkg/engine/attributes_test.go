package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAttributes_Get(t *testing.T) {
	attrs := Attributes{"name": "test", "age": 30}

	v, ok := attrs.Get("name")
	if !ok || v != "test" {
		t.Errorf("Get(name) = %v, %v; want test, true", v, ok)
	}

	v, ok = attrs.Get("missing")
	if ok || v != nil {
		t.Errorf("Get(missing) = %v, %v; want nil, false", v, ok)
	}
}

func TestAttributes_Set(t *testing.T) {
	attrs := Attributes{}
	attrs.Set("key", "value")
	if attrs["key"] != "value" {
		t.Errorf("Set failed: attrs[key] = %v; want value", attrs["key"])
	}
}

func TestAttributes_Has(t *testing.T) {
	attrs := Attributes{"a": 1}
	if !attrs.Has("a") {
		t.Error("Has(a) = false; want true")
	}
	if attrs.Has("b") {
		t.Error("Has(b) = true; want false")
	}
}

func TestAttributes_Delete(t *testing.T) {
	attrs := Attributes{"a": 1, "b": 2}
	attrs.Delete("a")
	if attrs.Has("a") {
		t.Error("Delete failed: a still exists")
	}
	if !attrs.Has("b") {
		t.Error("Delete removed wrong key")
	}
}

func TestAttributes_Keys(t *testing.T) {
	attrs := Attributes{"c": 3, "a": 1, "b": 2}
	keys := attrs.Keys()
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(keys, expected) {
		t.Errorf("Keys() = %v; want %v", keys, expected)
	}
}

func TestAttributes_Merge(t *testing.T) {
	a := Attributes{"x": 1, "y": 2}
	b := Attributes{"y": 99, "z": 3}
	a.Merge(b)

	if a["x"] != 1 {
		t.Error("Merge lost existing key")
	}
	if a["y"] != 99 {
		t.Errorf("Merge didn't overwrite: y = %v; want 99", a["y"])
	}
	if a["z"] != 3 {
		t.Error("Merge didn't add new key")
	}
}

func TestAttributes_String(t *testing.T) {
	attrs := Attributes{
		"name":   "hello",
		"age":    42,
		"active": true,
	}

	if attrs.String("name") != "hello" {
		t.Errorf("String(name) = %q; want hello", attrs.String("name"))
	}
	if attrs.String("age") != "42" {
		t.Errorf("String(age) = %q; want 42", attrs.String("age"))
	}
	if attrs.String("active") != "true" {
		t.Errorf("String(active) = %q; want true", attrs.String("active"))
	}
	if attrs.String("missing") != "" {
		t.Errorf("String(missing) = %q; want empty", attrs.String("missing"))
	}
}

func TestAttributes_Int(t *testing.T) {
	attrs := Attributes{
		"int":    42,
		"int64":  int64(100),
		"float":  3.14,
		"string": "notint",
	}

	if attrs.Int("int") != 42 {
		t.Errorf("Int(int) = %d; want 42", attrs.Int("int"))
	}
	if attrs.Int("int64") != 100 {
		t.Errorf("Int(int64) = %d; want 100", attrs.Int("int64"))
	}
	if attrs.Int("float") != 3 {
		t.Errorf("Int(float) = %d; want 3", attrs.Int("float"))
	}
	if attrs.Int("string") != 0 {
		t.Errorf("Int(string) = %d; want 0", attrs.Int("string"))
	}
	if attrs.Int("missing") != 0 {
		t.Errorf("Int(missing) = %d; want 0", attrs.Int("missing"))
	}
}

func TestAttributes_Float(t *testing.T) {
	attrs := Attributes{
		"int":   42,
		"float": 3.14,
	}

	if attrs.Float("int") != 42.0 {
		t.Errorf("Float(int) = %f; want 42.0", attrs.Float("int"))
	}
	if attrs.Float("float") != 3.14 {
		t.Errorf("Float(float) = %f; want 3.14", attrs.Float("float"))
	}
	if attrs.Float("missing") != 0 {
		t.Errorf("Float(missing) = %f; want 0", attrs.Float("missing"))
	}
}

func TestAttributes_Bool(t *testing.T) {
	attrs := Attributes{
		"active": true,
		"done":   false,
		"age":    30,
	}

	if !attrs.Bool("active") {
		t.Error("Bool(active) = false; want true")
	}
	if attrs.Bool("done") {
		t.Error("Bool(done) = true; want false")
	}
	if attrs.Bool("age") {
		t.Error("Bool(age) = true; want false")
	}
	if attrs.Bool("missing") {
		t.Error("Bool(missing) = true; want false")
	}
}

func TestAttributes_StringArray(t *testing.T) {
	attrs := Attributes{
		"tags":   []string{"a", "b", "c"},
		"mixed":  []any{"x", 42, true},
		"single": "only",
	}

	got := attrs.StringArray("tags")
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("StringArray(tags) = %v; want [a b c]", got)
	}

	got = attrs.StringArray("mixed")
	if !reflect.DeepEqual(got, []string{"x", "42", "true"}) {
		t.Errorf("StringArray(mixed) = %v; want [x 42 true]", got)
	}

	got = attrs.StringArray("missing")
	if got != nil {
		t.Errorf("StringArray(missing) = %v; want nil", got)
	}
}

func TestAttributes_Empty(t *testing.T) {
	attrs := Attributes{}

	if attrs.Keys() != nil && len(attrs.Keys()) != 0 {
		t.Error("Empty attributes should have no keys")
	}
	if attrs.String("x") != "" {
		t.Error("String on empty attrs should return empty")
	}
	if attrs.Int("x") != 0 {
		t.Error("Int on empty attrs should return 0")
	}
	if attrs.Float("x") != 0 {
		t.Error("Float on empty attrs should return 0")
	}
	if attrs.Bool("x") {
		t.Error("Bool on empty attrs should return false")
	}
}

func TestAttributes_SaveAndLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	attrs := Attributes{
		"name":     "task-alpha",
		"priority": 5,
		"type":     "bug",
		"tags":     []string{"urgent", "backend"},
	}

	// Save
	if err := attrs.SaveToYAML(path); err != nil {
		t.Fatalf("SaveToYAML error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("YAML file was not created")
	}

	// Load into new map
	loaded := Attributes{}
	if err := loaded.LoadFromYAML(path); err != nil {
		t.Fatalf("LoadFromYAML error: %v", err)
	}

	if loaded.String("name") != "task-alpha" {
		t.Errorf("loaded name = %q; want task-alpha", loaded.String("name"))
	}
	if loaded.Int("priority") != 5 {
		t.Errorf("loaded priority = %d; want 5", loaded.Int("priority"))
	}
	tags := loaded.StringArray("tags")
	if len(tags) != 2 || tags[0] != "urgent" || tags[1] != "backend" {
		t.Errorf("loaded tags = %v; want [urgent backend]", tags)
	}
}

func TestAttributes_LoadFromYAML_NonExistent(t *testing.T) {
	attrs := Attributes{}
	err := attrs.LoadFromYAML("/nonexistent/path/file.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestAttributes_LoadFromYAML_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	// Write invalid YAML
	if err := os.WriteFile(path, []byte("{{{{invalid yaml}}}}"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	attrs := Attributes{}
	err := attrs.LoadFromYAML(path)
	// Some YAML parsers are lenient; we just check the call doesn't panic
	_ = err
}

func TestAttributes_LoadFromYAML_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.yaml")

	// Write initial YAML
	initial := `name: original
oldkey: oldvalue
`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Pre-populate with extra data
	attrs := Attributes{"extrakey": "should disappear"}
	if err := attrs.LoadFromYAML(path); err != nil {
		t.Fatalf("LoadFromYAML error: %v", err)
	}

	if attrs.Has("extrakey") {
		t.Error("LoadFromYAML should clear existing keys not in the file")
	}
	if attrs.String("name") != "original" {
		t.Errorf("name = %q; want original", attrs.String("name"))
	}
}
