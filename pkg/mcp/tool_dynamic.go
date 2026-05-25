package mcp

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

const defaultTimeoutSeconds = 30

// dynamicToolContext holds the metadata needed to execute a dynamic tool.
type dynamicToolContext struct {
	toolName       string
	inputProps     map[string]string // param_name -> description
	requiredArgs   []string
	timeoutSeconds int
	proj           *engine.Project
}

// DynamicInput is a flexible input type for dynamic tools.
type DynamicInput struct {
	Args map[string]string `json:",inline"`
}

// RegisterDynamicTools registers all user-defined tools from orqen.yaml configuration.
// Dynamic tools are registered after built-in tools to ensure they appear later in the tool list.
func RegisterDynamicTools(server *mcp.Server, proj *engine.Project) {
	if proj == nil || len(proj.Tools) == 0 {
		return
	}

	// Check for conflicts with built-in tools
	for toolName := range proj.Tools {
		if _, exists := tools[toolName]; exists {
			slog.Warn("ignoring user-defined tool: name conflicts with built-in tool", "tool", toolName)
			continue
		}

		toolDef := proj.Tools[toolName]
		if err := validateToolDef(toolName, toolDef); err != nil {
			slog.Warn("skipping invalid tool definition", "tool", toolName, "error", err)
			continue
		}

		timeout := toolDef.Timeout
		if timeout <= 0 {
			timeout = defaultTimeoutSeconds
		}

		// Build input properties map
		inputProps := make(map[string]string)
		requiredArgs := make([]string, 0, len(toolDef.Args))
		for paramName, paramDesc := range toolDef.Args {
			inputProps[paramName] = paramDesc
			requiredArgs = append(requiredArgs, paramName)
		}

		mcpTool := &mcp.Tool{
			Name:        toolName,
			Description: toolDef.Description,
		}

		tools[toolName] = mcpTool

		dtCtx := &dynamicToolContext{
			toolName:       toolName,
			inputProps:     inputProps,
			requiredArgs:   requiredArgs,
			timeoutSeconds: timeout,
			proj:           proj,
		}

		// Use a raw message handler since we can't define dynamic struct types at runtime
		// The SDK will pass the raw JSON which we parse ourselves
		registerDynamicToolWithRawHandler(server, mcpTool, dtCtx)
	}
}

// validateToolDef validates a tool definition for required fields and valid OS keys.
func validateToolDef(toolName string, def engine.ToolDef) error {
	if len(def.Command) == 0 {
		// Check if there's at least one OS-specific command
		hasOSCommand := false
		for osKey := range def.OSCommands {
			if engine.ValidOSKeys[osKey] && len(def.OSCommands[osKey]) > 0 {
				hasOSCommand = true
				break
			}
		}
		if !hasOSCommand {
			return fmt.Errorf("tool has no command and no valid OS-specific commands (windows/darwin/linux)")
		}
	}

	// Validate OS keys
	for osKey := range def.OSCommands {
		if !engine.ValidOSKeys[osKey] {
			return fmt.Errorf("invalid OS key %q: must be one of windows, darwin, linux", osKey)
		}
	}

	return nil
}

// registerDynamicToolWithRawHandler registers a dynamic tool using a raw JSON handler.
func registerDynamicToolWithRawHandler(server *mcp.Server, mcpTool *mcp.Tool, dtCtx *dynamicToolContext) {
	// We need to use the lower-level SDK API to handle raw JSON input
	// The mcp.AddTool function expects a typed handler, so we need a different approach

	// For dynamic tools, we'll use a map[string]string as input type
	// The SDK will unmarshal the JSON arguments into this map
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input map[string]string) (*mcp.CallToolResult, map[string]interface{}, error) {
		return handleDynamicTool(ctx, req, input, dtCtx)
	}

	mcp.AddTool(server, mcpTool, handler)
}

// handleDynamicTool is the generic handler for all dynamic tools.
func handleDynamicTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input map[string]string,
	dtCtx *dynamicToolContext,
) (*mcp.CallToolResult, map[string]interface{}, error) {
	// Validate required arguments
	for _, argName := range dtCtx.requiredArgs {
		if _, ok := input[argName]; !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("missing required parameter: %s", argName)},
				},
				IsError: true,
			}, nil, nil
		}
	}

	// Get tool definition
	toolDef, ok := dtCtx.proj.Tools[dtCtx.toolName]
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("tool %q not found in configuration", dtCtx.toolName)},
			},
			IsError: true,
		}, nil, nil
	}

	// Resolve command for current OS
	command, err := toolDef.GetCommandForOS(runtime.GOOS)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("command resolution failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Substitute wildcards in command
	resolvedCommand := make([]string, len(command))
	for i, arg := range command {
		resolvedCommand[i] = substituteWildcards(arg, input)
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(dtCtx.timeoutSeconds)*time.Second)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(timeoutCtx, resolvedCommand[0], resolvedCommand[1:]...)
	cmd.Dir = dtCtx.proj.DirAbs

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Build output
	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n--- stderr ---\n")
		}
		output.WriteString(stderr.String())
	}

	outMap := map[string]interface{}{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		if ctxErr := timeoutCtx.Err(); ctxErr != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("command timed out after %d seconds", dtCtx.timeoutSeconds)},
				},
				IsError: true,
			}, outMap, nil
		}

		errorMsg := fmt.Sprintf("command failed: %v", err)
		if output.Len() > 0 {
			errorMsg += "\n" + output.String()
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: errorMsg},
			},
			IsError: true,
		}, outMap, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, outMap, nil
}

// substituteWildcards replaces $param_name tokens in a string with actual values from args.
// Only exact $param_name tokens are replaced (not partial matches like prefix_$param_name).
func substituteWildcards(arg string, args map[string]string) string {
	// Exact token match: $param_name
	if strings.HasPrefix(arg, "$") {
		paramName := arg[1:]
		if val, ok := args[paramName]; ok {
			return val
		}
	}
	return arg
}
