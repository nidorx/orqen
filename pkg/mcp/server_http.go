package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
)

func ServerHttp(proj *engine.Project) http.Handler {

	server := createServer()

	// Register real handlers with project reference
	addTool(server, tnItem, ItemHandler, proj)
	addTool(server, tnItemMove, ItemMoveHandler, proj)
	addTool(server, tnItemCreate, ItemCreateHandler, proj)
	addTool(server, tnItemSearch, ItemSearchHandler, proj)
	addTool(server, tnItemAttrsSet, ItemAttrsSetHandler, proj)
	addTool(server, tnItemAttrsDel, ItemAttrsDelHandler, proj)
	addTool(server, tnItemAttrsSchema, ItemAttrSchemaHandler, proj)
	addTool(server, tnItemDependencies, ItemDependenciesHandler, proj)
	addTool(server, tnLaneList, LaneListHandler, proj)
	addTool(server, tnProjectInfo, ProjectInfoHandler, proj)

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
