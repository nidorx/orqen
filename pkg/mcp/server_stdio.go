package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func StartStdio(orqenPort string, projectId string, jobId string) {

	server := createServer()

	client := mcp.NewClient(&mcp.Implementation{
		Name:       "orqen-client",
		Title:      impl.Title,
		Version:    impl.Version,
		WebsiteURL: impl.WebsiteURL,
		Icons:      impl.Icons,
	}, nil)

	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: fmt.Sprintf("http://127.0.0.1:%s/mcp/http/%s", orqenPort, projectId),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	// Tools that need jobId (auto-injected)
	addToolProxy(server, tnStatus, StatusHandler, session, jobId)
	addToolProxy(server, tnListItems, ListItemsHandler, session, jobId)
	addToolProxy(server, tnScanModule, ScanModuleHandler, session, jobId)
	addToolProxy(server, tnSchema, SchemaHandler, session, jobId)
	addToolProxy(server, tnMoveItem, MoveItemHandler, session, jobId)
	addToolProxy(server, tnDependencies, DependenciesHandler, session, jobId)
	addToolProxy(server, tnProjectInfo, ProjectInfoHandler, session, jobId)
	addToolProxy(server, tnCreateItem, CreateItemHandler, session, jobId)
	addToolProxy(server, tnNextSequence, NextSequenceHandler, session, jobId)
	addToolProxy(server, tnListLanes, ListLanesHandler, session, jobId)

	// Run the server over stdin/stdout, until the client disconnects.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
