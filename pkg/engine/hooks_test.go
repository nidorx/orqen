package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Hook Definition Tests
// ============================================================================

func TestHookDefinition_GetCommandForOS(t *testing.T) {
	tests := []struct {
		name     string
		hook     HookDefinition
		goos     string
		expected []string
	}{
		{
			name: "windows with variant",
			hook: HookDefinition{
				Command: []string{"script.sh", "arg1"},
				Windows: []string{"script.cmd", "/arg1"},
			},
			goos:     "windows",
			expected: []string{"script.cmd", "/arg1"},
		},
		{
			name: "windows without variant falls back to base",
			hook: HookDefinition{
				Command: []string{"script.sh", "arg1"},
			},
			goos:     "windows",
			expected: []string{"script.sh", "arg1"},
		},
		{
			name: "darwin with variant",
			hook: HookDefinition{
				Command: []string{"script.sh", "arg1"},
				Darwin:  []string{"script-macos.sh", "arg1"},
			},
			goos:     "darwin",
			expected: []string{"script-macos.sh", "arg1"},
		},
		{
			name: "linux with variant",
			hook: HookDefinition{
				Command: []string{"script.sh", "arg1"},
				Linux:   []string{"script-linux.sh", "arg1"},
			},
			goos:     "linux",
			expected: []string{"script-linux.sh", "arg1"},
		},
		{
			name: "unknown OS falls back to base",
			hook: HookDefinition{
				Command: []string{"script.sh", "arg1"},
			},
			goos:     "freebsd",
			expected: []string{"script.sh", "arg1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.hook.GetCommandForOS(tt.goos)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// ============================================================================
// HookBinding YAML Parsing Tests
// ============================================================================

func TestHookBinding_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected HookBinding
		wantErr  bool
	}{
		{
			name:  "normal binding",
			input: `named_hook_01`,
			expected: HookBinding{
				Name:    "named_hook_01",
				Negated: false,
			},
		},
		{
			name:  "negated binding with !",
			input: `"!named_hook_02"`,
			expected: HookBinding{
				Name:    "named_hook_02",
				Negated: true,
			},
		},
		{
			name:  "negated binding with single quotes",
			input: `'!named_hook_03'`,
			expected: HookBinding{
				Name:    "named_hook_03",
				Negated: true,
			},
		},
		{
			name:  "quoted normal binding",
			input: `"named_hook_04"`,
			expected: HookBinding{
				Name:    "named_hook_04",
				Negated: false,
			},
		},
		{
			name:    "empty binding fails",
			input:   `""`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hb HookBinding
			err := hb.UnmarshalYAML([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if hb.Name != tt.expected.Name {
				t.Errorf("name: expected %q, got %q", tt.expected.Name, hb.Name)
			}
			if hb.Negated != tt.expected.Negated {
				t.Errorf("negated: expected %v, got %v", tt.expected.Negated, hb.Negated)
			}
		})
	}
}

// ============================================================================
// NamedHooks YAML Parsing Tests
// ============================================================================

func TestNamedHooks_UnmarshalYAML(t *testing.T) {
	input := `
hook_01: ["script.sh", "--arg1", "$WI"]
hook_01.windows: ["script.cmd", "/arg1", "%WI%"]
hook_02: ["other.sh", "--arg2"]
hook_02.darwin: ["other-macos.sh", "--arg2"]
hook_02.linux: ["other-linux.sh", "--arg2"]
`

	var hooks NamedHooks
	err := hooks.UnmarshalYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check hook_01
	if h, ok := hooks["hook_01"]; !ok {
		t.Fatal("hook_01 not found")
	} else {
		if len(h.Command) != 3 || h.Command[0] != "script.sh" {
			t.Errorf("hook_01 command: expected [script.sh --arg1 $WI], got %v", h.Command)
		}
		if len(h.Windows) != 3 || h.Windows[0] != "script.cmd" {
			t.Errorf("hook_01.windows: expected [script.cmd /arg1 %%WI%%], got %v", h.Windows)
		}
	}

	// Check hook_02
	if h, ok := hooks["hook_02"]; !ok {
		t.Fatal("hook_02 not found")
	} else {
		if len(h.Command) != 2 || h.Command[0] != "other.sh" {
			t.Errorf("hook_02 command: expected [other.sh --arg2], got %v", h.Command)
		}
		if len(h.Darwin) != 2 || h.Darwin[0] != "other-macos.sh" {
			t.Errorf("hook_02.darwin: expected [other-macos.sh --arg2], got %v", h.Darwin)
		}
		if len(h.Linux) != 2 || h.Linux[0] != "other-linux.sh" {
			t.Errorf("hook_02.linux: expected [other-linux.sh --arg2], got %v", h.Linux)
		}
	}
}

func TestNamedHooks_UnmarshalYAML_InvalidSuffix(t *testing.T) {
	// Test that unknown suffixes create a separate hook entry with the full key as name
	input := `
hook_01: ["base.sh"]
hook_01.unknown: ["separate-entry.sh"]
`

	var hooks NamedHooks
	err := hooks.UnmarshalYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// hook_01 should have only the base command
	if h, ok := hooks["hook_01"]; !ok {
		t.Fatal("hook_01 not found")
	} else {
		if len(h.Command) != 1 || h.Command[0] != "base.sh" {
			t.Errorf("hook_01 command: expected [base.sh], got %v", h.Command)
		}
	}

	// hook_01.unknown should be a separate entry (full key as name)
	if h, ok := hooks["hook_01.unknown"]; !ok {
		t.Fatal("hook_01.unknown not found")
	} else {
		if len(h.Command) != 1 || h.Command[0] != "separate-entry.sh" {
			t.Errorf("hook_01.unknown command: expected [separate-entry.sh], got %v", h.Command)
		}
	}
}

// ============================================================================
// Hook Validation Tests
// ============================================================================

func TestValidate_HookDefinitions(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create module directories
	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(filepath.Join(taskDir, "01_inbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Test: empty command array fails
	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 1

hooks:
  hook_01: []
  hook_02: ["script.sh", "arg1"]

modules:
  - name: task
    dir: "tasks"
    lanes:
      - name: "inbox"
        purpose: "User ideas"
`

	configPath := filepath.Join(orqenDir, "orqen.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Fatal("expected validation error for empty hook command, got nil")
	}
	// Check error mentions the hook
	// (error message format may vary, just check it's not nil)
}

func TestValidate_HookBindings_ReferenceUnknownHook(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(filepath.Join(taskDir, "01_inbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Test: binding references non-existent hook
	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 1

hooks:
  existing_hook: ["script.sh"]

modules:
  - name: task
    dir: "tasks"
    hooks:
      pre:
        - existing_hook
        - nonexistent_hook
    lanes:
      - name: "inbox"
        purpose: "User ideas"
`

	configPath := filepath.Join(orqenDir, "orqen.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tempDir)
	if err == nil {
		t.Fatal("expected validation error for unknown hook reference, got nil")
	}
}

func TestValidate_ValidHooks(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(filepath.Join(taskDir, "01_inbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Test: valid hooks configuration
	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 1

hooks:
  hook_01: ["script.sh", "--arg1"]
  hook_01.windows: ["script.cmd", "/arg1"]
  hook_02: ["other.sh"]

modules:
  - name: task
    dir: "tasks"
    hooks:
      pre:
        - hook_01
      post:
        - hook_02
    lanes:
      - name: "inbox"
        purpose: "User ideas"
        hooks:
          post:
            - "!hook_02"
            - hook_01
`

	configPath := filepath.Join(orqenDir, "orqen.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := Load(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hooks were parsed correctly
	if proj.NamedHooks == nil {
		t.Fatal("NamedHooks is nil")
	}
	if len(proj.NamedHooks) != 2 {
		t.Errorf("expected 2 named hooks, got %d", len(proj.NamedHooks))
	}

	// Verify module hooks
	mod := proj.Modules[0]
	if mod.Hooks == nil {
		t.Fatal("module Hooks is nil")
	}
	if len(mod.Hooks.Pre) != 1 || mod.Hooks.Pre[0].Name != "hook_01" {
		t.Errorf("module pre hooks: expected [hook_01], got %v", mod.Hooks.Pre)
	}
	if len(mod.Hooks.Post) != 1 || mod.Hooks.Post[0].Name != "hook_02" {
		t.Errorf("module post hooks: expected [hook_02], got %v", mod.Hooks.Post)
	}

	// Verify lane hooks
	lane := mod.Lanes[0]
	if lane.Hooks == nil {
		t.Fatal("lane Hooks is nil")
	}
	if len(lane.Hooks.Post) != 2 {
		t.Errorf("lane post hooks: expected 2 bindings, got %d", len(lane.Hooks.Post))
	}
	if !lane.Hooks.Post[0].Negated || lane.Hooks.Post[0].Name != "hook_02" {
		t.Errorf("first lane post hook: expected !hook_02, got %v", lane.Hooks.Post[0])
	}
	if lane.Hooks.Post[1].Negated || lane.Hooks.Post[1].Name != "hook_01" {
		t.Errorf("second lane post hook: expected hook_01, got %v", lane.Hooks.Post[1])
	}
}

// ============================================================================
// Integration Test: Full Config with Hooks
// ============================================================================

func TestLoad_FullConfigWithHooks(t *testing.T) {
	tempDir := t.TempDir()
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	taskDir := filepath.Join(tempDir, "tasks")
	if err := os.MkdirAll(filepath.Join(taskDir, "01_inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(taskDir, "02_doing"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 1

hooks:
  notify: ["notify.sh", "$WI"]
  notify.windows: ["notify.cmd", "%WI%"]
  validate: ["validate.sh"]
  cleanup: ["cleanup.sh"]

modules:
  - name: task
    dir: "tasks"
    order: ["doing", "inbox"]
    hooks:
      pre:
        - validate
        - notify
      post:
        - notify
        - cleanup
    lanes:
      - name: "inbox"
        purpose: "User ideas"
      - name: "doing"
        purpose: "Task being implemented"
        hooks:
          pre:
            - "!validate"
            - notify
`

	configPath := filepath.Join(orqenDir, "orqen.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := Load(tempDir)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}

	// Verify hooks
	if len(proj.NamedHooks) != 3 {
		t.Errorf("expected 3 named hooks, got %d", len(proj.NamedHooks))
	}

	// Check notify has windows variant
	notify := proj.NamedHooks["notify"]
	if notify == nil {
		t.Fatal("notify hook not found")
	}
	if len(notify.Windows) == 0 {
		t.Error("notify hook should have windows variant")
	}

	// Check module has pre/post hooks
	mod := proj.Modules[0]
	if mod.Hooks == nil {
		t.Fatal("module hooks is nil")
	}
	if len(mod.Hooks.Pre) != 2 {
		t.Errorf("module should have 2 pre hooks, got %d", len(mod.Hooks.Pre))
	}
	if len(mod.Hooks.Post) != 2 {
		t.Errorf("module should have 2 post hooks, got %d", len(mod.Hooks.Post))
	}

	// Check doing lane has negated hook
	doingLane := mod.GetLane("doing")
	if doingLane == nil {
		t.Fatal("doing lane not found")
	}
	if doingLane.Hooks == nil {
		t.Fatal("doing lane hooks is nil")
	}
	if len(doingLane.Hooks.Pre) != 2 {
		t.Errorf("doing lane should have 2 pre bindings, got %d", len(doingLane.Hooks.Pre))
	}
	if !doingLane.Hooks.Pre[0].Negated || doingLane.Hooks.Pre[0].Name != "validate" {
		t.Errorf("first doing pre hook should be !validate, got %v", doingLane.Hooks.Pre[0])
	}
}