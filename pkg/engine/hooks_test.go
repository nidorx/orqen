package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

// ============================================================================
// Wildcard Expansion Tests
// ============================================================================

func TestExpandWildcards(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		vars     map[string]string
		expected []string
	}{
		{
			name:     "single wildcard replaced",
			args:     []string{"script.sh", "$WI"},
			vars:     map[string]string{"WI": "04_prioritized/WI-0002-test"},
			expected: []string{"script.sh", "04_prioritized/WI-0002-test"},
		},
		{
			name:     "multiple wildcards replaced",
			args:     []string{"script.sh", "$MODULE", "$LANE", "$WI"},
			vars:     map[string]string{"MODULE": "task", "LANE": "ready", "WI": "06_ready/WI-0002"},
			expected: []string{"script.sh", "task", "ready", "06_ready/WI-0002"},
		},
		{
			name:     "unknown wildcard left as-is",
			args:     []string{"script.sh", "$UNKNOWN"},
			vars:     map[string]string{"WI": "04_prioritized/WI-0002"},
			expected: []string{"script.sh", "$UNKNOWN"},
		},
		{
			name:     "mixed known and unknown wildcards",
			args:     []string{"script.sh", "$WI", "$UNKNOWN", "$MODULE"},
			vars:     map[string]string{"WI": "04_prioritized/WI-0002", "MODULE": "task"},
			expected: []string{"script.sh", "04_prioritized/WI-0002", "$UNKNOWN", "task"},
		},
		{
			name:     "no wildcards",
			args:     []string{"script.sh", "--arg1"},
			vars:     map[string]string{"WI": "04_prioritized/WI-0002"},
			expected: []string{"script.sh", "--arg1"},
		},
		{
			name:     "wildcard in middle of argument",
			args:     []string{"script.sh", "--path=/tmp/$WI/output"},
			vars:     map[string]string{"WI": "04_prioritized/WI-0002"},
			expected: []string{"script.sh", "--path=/tmp/04_prioritized/WI-0002/output"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandWildcards(tt.args, tt.vars)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
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
// Hook Executor Tests
// ============================================================================

func TestHookExecutor_Execute_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	executor := NewHookExecutor(t.TempDir())
	hookDef := &HookDefinition{
		Command: []string{"echo", "hello"},
	}

	result := executor.Execute("test_hook", hookDef, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", result.Stdout)
	}
	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Duration == 0 {
		t.Error("expected duration > 0")
	}
}

func TestHookExecutor_Execute_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	executor := NewHookExecutor(t.TempDir())
	hookDef := &HookDefinition{
		Command: []string{"sh", "-c", "exit 42"},
	}

	result := executor.Execute("failing_hook", hookDef, nil)

	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
	if result.Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestHookExecutor_Execute_WildcardExpansion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	executor := NewHookExecutor(t.TempDir())
	hookDef := &HookDefinition{
		Command: []string{"echo", "$WI"},
	}

	vars := map[string]string{"WI": "04_prioritized/WI-0002-test"}
	result := executor.Execute("wildcard_hook", hookDef, vars)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "04_prioritized/WI-0002-test") {
		t.Errorf("expected stdout to contain expanded wildcard, got %q", result.Stdout)
	}
}

func TestHookExecutor_Execute_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	executor := NewHookExecutor(t.TempDir())
	hookDef := &HookDefinition{
		Command: []string{"sh", "-c", "sleep 10"},
		Timeout: 100 * time.Millisecond,
	}

	result := executor.Execute("timeout_hook", hookDef, nil)

	if result.ExitCode != -1 {
		t.Errorf("expected exit code -1 for timeout, got %d", result.ExitCode)
	}
	if result.Err == nil {
		t.Error("expected timeout error, got nil")
	}
	if _, ok := result.Err.(*HookTimeoutError); !ok {
		t.Errorf("expected HookTimeoutError, got %T", result.Err)
	}
}

func TestHookExecutor_Execute_NoCommand(t *testing.T) {
	executor := NewHookExecutor(t.TempDir())
	hookDef := &HookDefinition{
		Command: []string{},
	}

	result := executor.Execute("empty_hook", hookDef, nil)

	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
	if result.Err == nil {
		t.Error("expected error for empty command, got nil")
	}
}

// ============================================================================
// Hook Resolution Tests
// ============================================================================

func TestResolveHooks_ModuleOnly(t *testing.T) {
	namedHooks := NamedHooks{
		"hook1": {Command: []string{"script1.sh"}},
		"hook2": {Command: []string{"script2.sh"}},
	}

	moduleHooks := &HookBindings{
		Pre:  []HookBinding{{Name: "hook1"}, {Name: "hook2"}},
		Post: []HookBinding{{Name: "hook1"}},
	}

	preHooks, postHooks := ResolveHooks(moduleHooks, nil, namedHooks)

	if len(preHooks) != 2 {
		t.Errorf("expected 2 pre hooks, got %d", len(preHooks))
	}
	if len(postHooks) != 1 {
		t.Errorf("expected 1 post hook, got %d", len(postHooks))
	}
	if preHooks[0].Name != "hook1" || preHooks[1].Name != "hook2" {
		t.Errorf("pre hooks order unexpected: %v", preHooks)
	}
}

func TestResolveHooks_LaneExclusions(t *testing.T) {
	namedHooks := NamedHooks{
		"hook1": {Command: []string{"script1.sh"}},
		"hook2": {Command: []string{"script2.sh"}},
		"hook3": {Command: []string{"script3.sh"}},
	}

	moduleHooks := &HookBindings{
		Pre: []HookBinding{{Name: "hook1"}, {Name: "hook2"}},
	}

	laneHooks := &HookBindings{
		Pre: []HookBinding{
			{Name: "hook1", Negated: true}, // exclude hook1
			{Name: "hook3"},                // add hook3
		},
	}

	preHooks, _ := ResolveHooks(moduleHooks, laneHooks, namedHooks)

	// Should have hook2 (from module) and hook3 (from lane), but not hook1 (excluded)
	if len(preHooks) != 2 {
		t.Errorf("expected 2 pre hooks, got %d", len(preHooks))
	}

	hasHook2 := false
	hasHook3 := false
	for _, hook := range preHooks {
		if hook.Name == "hook2" {
			hasHook2 = true
		}
		if hook.Name == "hook3" {
			hasHook3 = true
		}
	}

	if !hasHook2 {
		t.Error("expected hook2 to be present")
	}
	if !hasHook3 {
		t.Error("expected hook3 to be present")
	}
}

func TestResolveHooks_Deduplication(t *testing.T) {
	namedHooks := NamedHooks{
		"hook1": {Command: []string{"script1.sh"}},
	}

	moduleHooks := &HookBindings{
		Pre: []HookBinding{{Name: "hook1"}},
	}

	laneHooks := &HookBindings{
		Pre: []HookBinding{{Name: "hook1"}}, // duplicate
	}

	preHooks, _ := ResolveHooks(moduleHooks, laneHooks, namedHooks)

	if len(preHooks) != 1 {
		t.Errorf("expected 1 pre hook (deduplicated), got %d", len(preHooks))
	}
}

func TestResolveHooks_UnknownHookIgnored(t *testing.T) {
	namedHooks := NamedHooks{
		"hook1": {Command: []string{"script1.sh"}},
	}

	moduleHooks := &HookBindings{
		Pre: []HookBinding{{Name: "hook1"}, {Name: "unknown_hook"}},
	}

	preHooks, _ := ResolveHooks(moduleHooks, nil, namedHooks)

	if len(preHooks) != 1 {
		t.Errorf("expected 1 pre hook (unknown ignored), got %d", len(preHooks))
	}
	if preHooks[0].Name != "hook1" {
		t.Errorf("expected hook1, got %v", preHooks[0])
	}
}

// ============================================================================
// HOOK-FAIL Artifact Tests
// ============================================================================

func TestCreateHookFailArtifact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tempDir := t.TempDir()
	lane := &Lane{
		DirAbs: tempDir,
		Dir:    "06_ready",
		Module: &Module{},
	}
	item := &WorkItem{
		Seq:  2,
		Name: "WI-0002-test",
		Lane: lane,
	}

	hook := &ResolvedHook{
		Name: "failing_hook",
	}
	result := HookResult{
		HookName: "failing_hook",
		ExitCode: 1,
		Stdout:   "standard output",
		Stderr:   "error message",
		Duration: 100 * time.Millisecond,
		Err:      nil,
	}

	err := CreateHookFailArtifact(item, hook, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	artifactPath := filepath.Join(tempDir, "WI-0002-test", "WI-0002-HOOK-FAIL.md")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Fatal("expected HOOK-FAIL artifact to be created")
	}

	// Verify content
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("failed to read artifact: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# HOOK-FAIL: failing_hook") {
		t.Errorf("expected title, got: %s", contentStr[:100])
	}
	if !strings.Contains(contentStr, "**Exit Code:** 1") {
		t.Errorf("expected exit code, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "standard output") {
		t.Errorf("expected stdout in artifact, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "error message") {
		t.Errorf("expected stderr in artifact, got: %s", contentStr)
	}
}

func TestCreateHookFailArtifact_SequenceNumbering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	tempDir := t.TempDir()
	lane := &Lane{
		DirAbs: tempDir,
		Dir:    "06_ready",
		Module: &Module{},
	}
	item := &WorkItem{
		Seq:  2,
		Name: "WI-0002-test",
		Lane: lane,
	}

	// Create first artifact manually
	artifactDir := filepath.Join(tempDir, "WI-0002-test")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	firstArtifact := filepath.Join(artifactDir, "WI-0002-HOOK-FAIL.md")
	if err := os.WriteFile(firstArtifact, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create second artifact
	err := CreateHookFailArtifact(item, &ResolvedHook{Name: "hook2"}, HookResult{ExitCode: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify second file exists with sequence number
	secondArtifact := filepath.Join(artifactDir, "WI-0002-HOOK-FAIL-02.md")
	if _, err := os.Stat(secondArtifact); os.IsNotExist(err) {
		t.Fatal("expected second HOOK-FAIL artifact with sequence number")
	}
}

func TestCreateHookFailArtifact_ZeroSeq(t *testing.T) {
	item := &WorkItem{
		Seq:  0,
		Name: "WI-0000-test",
		Lane: &Lane{},
	}

	err := CreateHookFailArtifact(item, &ResolvedHook{Name: "hook"}, HookResult{})
	if err == nil {
		t.Error("expected error for zero sequence number")
	}
}

// ============================================================================
// Hook Definition Timeout Parsing Tests
// ============================================================================

func TestNamedHooks_UnmarshalYAML_WithTimeout(t *testing.T) {
	input := `
hook_01:
  command: ["script.sh", "--arg1"]
  timeout: 2m
hook_02:
  command: ["other.sh"]
`

	var hooks NamedHooks
	err := hooks.UnmarshalYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h, ok := hooks["hook_01"]; !ok {
		t.Fatal("hook_01 not found")
	} else {
		if len(h.Command) != 2 || h.Command[0] != "script.sh" {
			t.Errorf("hook_01 command: expected [script.sh --arg1], got %v", h.Command)
		}
		if h.Timeout != 2*time.Minute {
			t.Errorf("hook_01 timeout: expected 2m, got %v", h.Timeout)
		}
	}

	if h, ok := hooks["hook_02"]; !ok {
		t.Fatal("hook_02 not found")
	} else {
		if h.Timeout != 0 {
			t.Errorf("hook_02 timeout: expected 0 (not set), got %v", h.Timeout)
		}
	}
}

func TestNamedHooks_UnmarshalYELLegacyFormat(t *testing.T) {
	// Test legacy format (map[string][]string)
	input := `
hook_01: ["script.sh", "--arg1"]
hook_02: ["other.sh"]
`

	var hooks NamedHooks
	err := hooks.UnmarshalYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(hooks))
	}

	if h, ok := hooks["hook_01"]; !ok {
		t.Fatal("hook_01 not found")
	} else {
		if len(h.Command) != 2 || h.Command[0] != "script.sh" {
			t.Errorf("hook_01 command: expected [script.sh --arg1], got %v", h.Command)
		}
	}
}
