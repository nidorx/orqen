package mcp

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
)

var (
	tools = map[string]*mcp.Tool{}
	impl  = &mcp.Implementation{
		Name:  "orqen",
		Title: "Orqen MCP Server",
	}
)

type ToolProjectHandler[In, Out any] func(
	_ context.Context, request *mcp.CallToolRequest, input In, proj *engine.Project,
) (result *mcp.CallToolResult, output Out, _ error)

func ServerHttp(proj *engine.Project) http.Handler {

	impl.Version = conf.GetInfo().Version
	impl.WebsiteURL = conf.GetInfo().Website
	server := mcp.NewServer(impl, nil)

	for name, tool := range tools {
		tool.Name = name
	}

	// Register real handlers with project reference
	addTool(server, tnWorkitem, WorkitemHandler, proj)
	addTool(server, tnWorkitemMove, WorkitemMoveHandler, proj)
	addTool(server, tnWorkitemCreate, WorkitemCreateHandler, proj)
	addTool(server, tnWorkitemSearch, WorkitemSearchHandler, proj)
	addTool(server, tnWorkitemAttrsSet, WorkitemAttrsSetHandler, proj)
	addTool(server, tnWorkitemAttrsDel, WorkitemAttrsDelHandler, proj)
	addTool(server, tnWorkitemAttrsSchema, WorkitemAttrSchemaHandler, proj)
	addTool(server, tnWorkitemDependencies, WorkitemDependenciesHandler, proj)
	addTool(server, tnLaneList, LaneListHandler, proj)
	addTool(server, tnProjectInfo, ProjectInfoHandler, proj)

	// Filesystem tools
	addTool(server, tnFsMove, FsMoveHandler, proj)
	addTool(server, tnFsCopy, FsCopyHandler, proj)
	addTool(server, tnFsList, FsListHandler, proj)
	addTool(server, tnFsTree, FsTreeHandler, proj)
	addTool(server, tnFsFind, FsFindHandler, proj)
	addTool(server, tnFsGrep, FsGrepHandler, proj)
	addTool(server, tnFsDiff, FsDiffHandler, proj)

	return mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		return server
	}, nil)
}

func addTool[In, Out any](s *mcp.Server, tool string, handler ToolProjectHandler[In, Out], proj *engine.Project) {
	mcp.AddTool(s, tools[tool], func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		return handler(ctx, req, input, proj)
	})
}
