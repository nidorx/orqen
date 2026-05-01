package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

func ServerHttp(proj *project.Project) http.Handler {

	server := createServer()

	// Register real handlers with project reference
	addTool(server, tnStatus, StatusHandler, proj)
	addTool(server, tnListItems, ListItemsHandler, proj)
	addTool(server, tnScanModule, ScanModuleHandler, proj)
	addTool(server, tnSchema, SchemaHandler, proj)
	addTool(server, tnMoveItem, MoveItemHandler, proj)
	addTool(server, tnDependencies, DependenciesHandler, proj)
	addTool(server, tnProjectInfo, ProjectInfoHandler, proj)
	addTool(server, tnCreateItem, CreateItemHandler, proj)
	addTool(server, tnNextSequence, NextSequenceHandler, proj)
	addTool(server, tnListLanes, ListLanesHandler, proj)

	return mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		return server
	}, nil)

	// return func(ctx *chain.Context) {
	// 	w := ctx.Writer.(*chain.ResponseWriterSpy).ResponseWriter
	// 	req := ctx.Request
	// 	mcpHandler.ServeHTTP(w, req)
	// }
}
