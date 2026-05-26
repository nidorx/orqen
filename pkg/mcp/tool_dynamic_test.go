package mcp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

func TestSubstituteWildcards(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		args     map[string]string
		expected string
	}{
		{
			name:     "exact wildcard substitution",
			arg:      "$param_a",
			args:     map[string]string{"param_a": "value_a"},
			expected: "value_a",
		},
		{
			name:     "no substitution for partial match",
			arg:      "prefix_$param_a",
			args:     map[string]string{"param_a": "value_a"},
			expected: "prefix_$param_a",
		},
		{
			name:     "no substitution when param missing",
			arg:      "$param_a",
			args:     map[string]string{"param_b": "value_b"},
			expected: "$param_a",
		},
		{
			name:     "non-wildcard arg unchanged",
			arg:      "--flag",
			args:     map[string]string{"param_a": "value_a"},
			expected: "--flag",
		},
		{
			name:     "multiple wildcards in different args",
			arg:      "$param_b",
			args:     map[string]string{"param_a": "value_a", "param_b": "value_b"},
			expected: "value_b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteWildcards(tt.arg, tt.args)
			if result != tt.expected {
				t.Errorf("substituteWildcards(%q, %v) = %q, want %q", tt.arg, tt.args, result, tt.expected)
			}
		})
	}
}

func TestValidateToolDef(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		def         engine.Tool
		expectError bool
	}{
		{
			name:     "valid tool with command",
			toolName: "my_tool",
			def: engine.Tool{
				Command: []string{"echo", "hello"},
				Args:    map[string]string{"msg": "message"},
			},
			expectError: false,
		},
		{
			name:     "valid tool with os-specific command",
			toolName: "my_tool",
			def: engine.Tool{
				Windows: []string{"echo.bat"},
				Linux:   []string{"echo.sh"},
				Args:    map[string]string{"msg": "message"},
			},
			expectError: false,
		},
		{
			name:     "invalid: no command and no os commands",
			toolName: "my_tool",
			def: engine.Tool{
				Args: map[string]string{"msg": "message"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestToolDefGetCommandForOS(t *testing.T) {
	tests := []struct {
		name        string
		def         engine.Tool
		goos        string
		expectCmd   []string
		expectError bool
	}{
		{
			name: "default command when no os override",
			def: engine.Tool{
				Command: []string{"default.sh", "--arg"},
			},
			goos:      "linux",
			expectCmd: []string{"default.sh", "--arg"},
		},
		{
			name: "os override takes precedence",
			def: engine.Tool{
				Command: []string{"default.sh"},
				Windows: []string{"windows.bat"},
			},
			goos:      "windows",
			expectCmd: []string{"windows.bat"},
		},
		{
			name: "no command for os and no default",
			def: engine.Tool{
				Linux: []string{"linux.sh"},
			},
			goos:        "windows",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.def.GetCommandForOS(tt.goos)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cmd) != len(tt.expectCmd) {
				t.Errorf("command length mismatch: got %d, want %d", len(cmd), len(tt.expectCmd))
			}
			for i := range cmd {
				if cmd[i] != tt.expectCmd[i] {
					t.Errorf("command[%d] = %q, want %q", i, cmd[i], tt.expectCmd[i])
				}
			}
		})
	}
}

func TestRegisterDynamicTools_EmptyProject(t *testing.T) {
	proj := &engine.Project{}
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// Should not panic with empty tools
	RegisterDynamicTools(server, proj)
}

func TestRegisterDynamicTools_NilProject(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// Should not panic with nil project
	RegisterDynamicTools(server, nil)
}

func TestDynamicToolHandler_MissingRequiredArg(t *testing.T) {
	tempDir := t.TempDir()
	proj := &engine.Project{
		DirAbs: tempDir,
		Tools: map[string]engine.Tool{
			"test_tool": {
				Command: []string{"echo"},
				Args:    map[string]string{"param_a": "first param", "param_b": "second param"},
			},
		},
	}

	dtCtx := &dynamicToolContext{
		toolName:       "test_tool",
		inputProps:     proj.Tools["test_tool"].Args,
		requiredArgs:   []string{"param_a", "param_b"},
		timeoutSeconds: 30,
		project:        proj,
	}

	// Missing param_b
	input := map[string]string{"param_a": "value_a"}
	result, _, err := handleDynamicTool(context.Background(), nil, input, dtCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.IsError {
		t.Error("expected error result for missing required arg")
	}
}

func TestDynamicToolHandler_CommandExecution(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple test script
	scriptPath := filepath.Join(tempDir, "test.sh")
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(tempDir, "test.bat")
	}

	if runtime.GOOS == "windows" {
		if err := os.WriteFile(scriptPath, []byte("@echo off\necho Hello from script"), 0755); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho Hello from script"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	proj := &engine.Project{
		DirAbs: tempDir,
		Tools: map[string]engine.Tool{
			"greet": {
				Command: []string{scriptPath},
				Args:    map[string]string{},
			},
		},
	}

	dtCtx := &dynamicToolContext{
		toolName:       "greet",
		inputProps:     proj.Tools["greet"].Args,
		requiredArgs:   []string{},
		timeoutSeconds: 30,
		project:        proj,
	}

	input := map[string]string{}
	result, _, err := handleDynamicTool(context.Background(), nil, input, dtCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.IsError {
		t.Errorf("unexpected error in result: %v", result.Content)
	}
}

func TestDynamicToolHandler_WildcardSubstitution(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test script that echoes its arguments
	scriptPath := filepath.Join(tempDir, "echo_args.sh")
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(tempDir, "echo_args.bat")
	}

	if runtime.GOOS == "windows" {
		if err := os.WriteFile(scriptPath, []byte("@echo off\necho %1 %2"), 0755); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho $1 $2"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	proj := &engine.Project{
		DirAbs: tempDir,
		Tools: map[string]engine.Tool{
			"echo_tool": {
				Command: []string{scriptPath, "$arg1", "$arg2"},
				Args: map[string]string{
					"arg1": "first argument",
					"arg2": "second argument",
				},
			},
		},
	}

	dtCtx := &dynamicToolContext{
		toolName:       "echo_tool",
		inputProps:     proj.Tools["echo_tool"].Args,
		requiredArgs:   []string{"arg1", "arg2"},
		timeoutSeconds: 30,
		project:        proj,
	}

	input := map[string]string{
		"arg1": "hello",
		"arg2": "world",
	}

	result, _, err := handleDynamicTool(context.Background(), nil, input, dtCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
}

func TestDynamicToolHandler_NonExistentCommand(t *testing.T) {
	tempDir := t.TempDir()

	proj := &engine.Project{
		DirAbs: tempDir,
		Tools: map[string]engine.Tool{
			"bad_tool": {
				Command: []string{"nonexistent-command-that-does-not-exist"},
				Args:    map[string]string{},
			},
		},
	}

	dtCtx := &dynamicToolContext{
		toolName:       "bad_tool",
		inputProps:     proj.Tools["bad_tool"].Args,
		requiredArgs:   []string{},
		timeoutSeconds: 5,
		project:        proj,
	}

	input := map[string]string{}
	result, _, err := handleDynamicTool(context.Background(), nil, input, dtCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.IsError {
		t.Error("expected error for non-existent command")
	}
}

func TestDynamicToolHandler_OSCommandResolution(t *testing.T) {
	tempDir := t.TempDir()

	proj := &engine.Project{
		DirAbs: tempDir,
		Tools: map[string]engine.Tool{
			"os_tool": {
				Linux:   []string{"echo", "linux"},
				Darwin:  []string{"echo", "darwin"},
				Windows: []string{"echo", "windows"},
				Args:    map[string]string{},
			},
		},
	}

	dtCtx := &dynamicToolContext{
		toolName:       "os_tool",
		inputProps:     proj.Tools["os_tool"].Args,
		requiredArgs:   []string{},
		timeoutSeconds: 30,
		project:        proj,
	}

	input := map[string]string{}
	result, _, err := handleDynamicTool(context.Background(), nil, input, dtCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// Should succeed with OS-specific command for current runtime
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
}
