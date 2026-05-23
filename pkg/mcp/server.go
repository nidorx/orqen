package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
)

var (
	tools = map[string]*mcp.Tool{}
	impl  = &mcp.Implementation{
		Name:       "orqen",
		Title:      "Orqen MCP Server",
		WebsiteURL: "https://orqen.ai.br",
	}
)

func createServer() *mcp.Server {
	impl.Version = conf.GetInfo().Version
	impl.WebsiteURL = conf.GetInfo().Website
	server := mcp.NewServer(impl, nil)

	for name, tool := range tools {
		tool.Name = name
	}

	return server
}

type ToolProjectHandler[In, Out any] func(_ context.Context, request *mcp.CallToolRequest, input In, proj *engine.Project) (result *mcp.CallToolResult, output Out, _ error)

func projectHandler2MCP[In, Out any](proj *engine.Project, handler ToolProjectHandler[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		return handler(ctx, req, input, proj)
	}
}
