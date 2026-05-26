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

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

// dynamicToolContext holds the metadata needed to execute a dynamic tool.
type dynamicToolContext struct {
	toolName       string
	inputProps     map[string]string // param_name -> description
	requiredArgs   []string
	timeoutSeconds int
	project        *engine.Project
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
		if err := toolDef.Validate(); err != nil {
			slog.Warn("skipping invalid tool definition", "tool", toolName, "error", err)
			continue
		}

		timeout := toolDef.Timeout
		if timeout <= 0 {
			timeout = 0
		}

		// Build input properties map
		inputProps := make(map[string]string)
		requiredArgs := make([]string, 0, len(toolDef.Args))
		for paramName, paramDesc := range toolDef.Args {
			inputProps[paramName] = paramDesc
			requiredArgs = append(requiredArgs, paramName)
		}

		// Build JSON Schema for input: {type: "object", properties: {param: {type: "string", description: ...}}, required: [...]}
		inputSchema := &jsonschema.Schema{
			Type:       "object",
			Properties: make(map[string]*jsonschema.Schema, len(inputProps)),
			Required:   append([]string(nil), requiredArgs...),
		}
		for paramName, paramDesc := range inputProps {
			inputSchema.Properties[paramName] = &jsonschema.Schema{
				Type:        "string",
				Description: paramDesc,
			}
		}

		// Build JSON Schema for output: {type: "object", properties: {stdout: {type: "string"}, stderr: {type: "string"}}}
		outputSchema := &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"stdout": {Type: "string", Description: "Standard output from command execution"},
				"stderr": {Type: "string", Description: "Standard error output from command execution"},
			},
		}

		mcpTool := &mcp.Tool{
			Name:         toolName,
			Description:  toolDef.Description,
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
		}

		tools[toolName] = mcpTool

		dtCtx := &dynamicToolContext{
			toolName:       toolName,
			inputProps:     inputProps,
			requiredArgs:   requiredArgs,
			timeoutSeconds: timeout,
			project:        proj,
		}

		// Use a raw message handler since we can't define dynamic struct types at runtime
		// The SDK will pass the raw JSON which we parse ourselves
		registerDynamicToolWithRawHandler(server, mcpTool, dtCtx)
	}
}

// registerDynamicToolWithRawHandler registers a dynamic tool using a raw JSON handler.
func registerDynamicToolWithRawHandler(server *mcp.Server, mcpTool *mcp.Tool, dtCtx *dynamicToolContext) {
	// We need to use the lower-level SDK API to handle raw JSON input
	// The mcp.AddTool function expects a typed handler, so we need a different approach

	// For dynamic tools, we'll use a map[string]string as input type
	// The SDK will unmarshal the JSON arguments into this map
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input map[string]string) (*mcp.CallToolResult, map[string]any, error) {
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
) (*mcp.CallToolResult, map[string]any, error) {
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
	tool, ok := dtCtx.project.Tools[dtCtx.toolName]
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("tool %q not found in configuration", dtCtx.toolName)},
			},
			IsError: true,
		}, nil, nil
	}

	// Resolve command for current OS
	command, err := tool.GetCommandForOS(runtime.GOOS)
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
	if dtCtx.timeoutSeconds > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, time.Duration(dtCtx.timeoutSeconds)*time.Second)
		defer cancel()
	}

	// Execute command
	cmd := exec.CommandContext(ctx, resolvedCommand[0], resolvedCommand[1:]...)
	cmd.Dir = dtCtx.project.DirAbs

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

	outMap := map[string]any{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		if dtCtx.timeoutSeconds > 0 {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: fmt.Sprintf("command timed out after %d seconds", dtCtx.timeoutSeconds)},
					},
					IsError: true,
				}, outMap, nil
			}
		}

		errorMsg := fmt.Sprintf("command failed: %v", err)
		if output.Len() > 0 {
			errorMsg += "\n" + output.String()
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: errorMsg}},
			IsError: true,
		}, outMap, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output.String()}},
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
