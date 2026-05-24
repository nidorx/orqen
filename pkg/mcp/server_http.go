package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

func ServerHttp(proj *engine.Project) http.Handler {

	server := createServer()

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

	// return func(ctx *chain.Context) {
	// 	w := ctx.Writer.(*chain.ResponseWriterSpy).ResponseWriter
	// 	req := ctx.Request
	// 	mcpHandler.ServeHTTP(w, req)
	// }
}

func addTool[In, Out any](s *mcp.Server, tool string, h ToolProjectHandler[In, Out], proj *engine.Project) {
	mcp.AddTool(s, tools[tool], projectHandler2MCP(proj, h))
}
