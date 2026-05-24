package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type InputWithWorkItemID interface {
	SetWorkItemID(workItemID string)
}

var (
	DEBUG_STDIO = false
	dFile       *os.File
)

func StartStdio(orqenPort string, projectId string) {

	if DEBUG_STDIO {
		if r := recover(); r != nil {
			debugAny("PANIC_RECOVER", r)
			panic(r)
		}

		defer func() {
			debugAny("END", time.Now())
		}()

		var openErr error
		dFile, openErr = os.OpenFile("./debug_mcp_stdio.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if openErr != nil {
			panic(openErr)
		}
		defer dFile.Close()

		debugAny("START", time.Now())
	}

	server := createServer()
	if DEBUG_STDIO {
		debugAny("MCP_SERVER_CREATED", time.Now())
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:       "orqen-client",
		Title:      impl.Title,
		Version:    impl.Version,
		WebsiteURL: impl.WebsiteURL,
		Icons:      impl.Icons,
	}, nil)
	if DEBUG_STDIO {
		debugAny("MCP_CLIENT_CREATED", time.Now())
	}

	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: fmt.Sprintf("http://127.0.0.1:%s/mcp/http/%s", orqenPort, projectId),
	}, nil)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	if DEBUG_STDIO {
		debugAny("MCP_SESSION_CONNECTED", time.Now())
	}

	// Tools that need workItemID (auto-injected)
	addToolProxy(server, tnWorkitem, WorkitemHandler, session)
	addToolProxy(server, tnWorkitemMove, WorkitemMoveHandler, session)
	addToolProxy(server, tnWorkitemCreate, WorkitemCreateHandler, session)
	addToolProxy(server, tnWorkitemSearch, WorkitemSearchHandler, session)
	addToolProxy(server, tnWorkitemAttrsSet, WorkitemAttrsSetHandler, session)
	addToolProxy(server, tnWorkitemAttrsDel, WorkitemAttrsDelHandler, session)
	addToolProxy(server, tnWorkitemAttrsSchema, WorkitemAttrSchemaHandler, session)
	addToolProxy(server, tnWorkitemDependencies, WorkitemDependenciesHandler, session)
	addToolProxy(server, tnLaneList, LaneListHandler, session)
	addToolProxy(server, tnProjectInfo, ProjectInfoHandler, session)

	// Filesystem tools (implement SetWorkItemID as no-op)
	addToolProxy(server, tnFsMove, FsMoveHandler, session)
	addToolProxy(server, tnFsCopy, FsCopyHandler, session)
	addToolProxy(server, tnFsList, FsListHandler, session)
	addToolProxy(server, tnFsTree, FsTreeHandler, session)
	addToolProxy(server, tnFsFind, FsFindHandler, session)
	addToolProxy(server, tnFsGrep, FsGrepHandler, session)
	addToolProxy(server, tnFsDiff, FsDiffHandler, session)
	if DEBUG_STDIO {
		debugAny("MCP_TOOLS_ADDED", time.Now())
	}

	// Run the server over stdin/stdout, until the client disconnects.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		panic(err)
	}
}

func addToolProxy[In any, Out any](s *mcp.Server, tool string, h ToolProjectHandler[In, Out], cs *mcp.ClientSession) {
	mcp.AddTool(s, tools[tool], sseProxy(tool, projectHandler2MCP(nil, h), cs))
}

func sseProxy[In any, Out any](tool string, _ mcp.ToolHandlerFor[In, Out], cs *mcp.ClientSession) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		// input.SetWorkItemID(workItemID)

		if DEBUG_STDIO {
			debugAny("CALL_TOOL_REQUEST", req)
		}

		result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: input})

		if DEBUG_STDIO {
			debugAny("CALL_TOOL_RESULT", result)

			if err != nil {
				debugError(err)
			}
		}

		return
	}
}

func debugError(err error) {
	stackBuf := make([]byte, 4096)
	n := runtime.Stack(stackBuf, false)
	stack := string(stackBuf[:n])

	fmt.Fprintf(dFile, "ERROR: %v\n\nSTACK:\n%s\n\n", err, stack)
}

func debugAny(key any, value any) {
	if value != nil {
		data, jsonErr := json.MarshalIndent(value, "", "  ")
		if jsonErr == nil {
			fmt.Fprintf(dFile, "%s:\n%s\n\n", key, string(data))
		} else {
			fmt.Fprintf(dFile, "%s (error marshaling): %v\n\n", key, jsonErr)
		}
	}
}
