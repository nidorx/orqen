package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/project"
)

var (
	tools = map[string]*mcp.Tool{}
	impl  = &mcp.Implementation{
		Name:       "orqen",
		Title:      "Execution layer for AI workflows",
		WebsiteURL: "https://orqen.ai.br",
	}
)

func createServer() *mcp.Server {
	impl.Version = conf.GetVersion().Value
	server := mcp.NewServer(impl, nil)

	for name, tool := range tools {
		tool.Name = name
	}

	return server
}

type ToolProjectHandler[In, Out any] func(_ context.Context, request *mcp.CallToolRequest, input In, proj *project.Project) (result *mcp.CallToolResult, output Out, _ error)

func projectHandler2MCP[In, Out any](proj *project.Project, handler ToolProjectHandler[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		return handler(ctx, req, input, proj)
	}
}

func addTool[In, Out any](s *mcp.Server, tool string, h ToolProjectHandler[In, Out], proj *project.Project) {
	mcp.AddTool(s, tools[tool], projectHandler2MCP(proj, h))
}

func addToolProxy[In InputWithJobId, Out any](
	s *mcp.Server,
	tool string,
	h ToolProjectHandler[In, Out],
	cs *mcp.ClientSession,
	jobId string,
) {
	mcp.AddTool(s, tools[tool], sseProxy(tool, projectHandler2MCP(nil, h), cs, jobId))
}

func sseProxy[In InputWithJobId, Out any](tool string, _ mcp.ToolHandlerFor[In, Out], cs *mcp.ClientSession, jobId string) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		input.SetJobId(jobId)
		result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: input})
		if err != nil {
			logError(err, result)
		}
		return
	}
}

type InputWithJobId interface {
	SetJobId(jobId string)
}

func logError(err error, result *mcp.CallToolResult) {
	stackBuf := make([]byte, 4096)
	n := runtime.Stack(stackBuf, false)
	stack := string(stackBuf[:n])

	f, openErr := os.OpenFile("./ignore/debug/mcp_error.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "ERROR: %v\n\nSTACK:\n%s\n\n", err, stack)

	if result != nil {
		data, jsonErr := json.MarshalIndent(result, "", "  ")
		if jsonErr == nil {
			fmt.Fprintf(f, "RESULT:\n%s\n\n", string(data))
		} else {
			fmt.Fprintf(f, "RESULT (error marshaling): %v\n\n", jsonErr)
		}
	}

	fmt.Fprint(f, "\n")
}
