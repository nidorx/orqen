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

func StartStdio(orqenPort string, projectId string, workItemID string) {

	if DEBUG_STDIO {
		if r := recover(); r != nil {
			debugAny("PANIC_RECOVER", r)
			panic(r)
		}

		defer func() {
			debugAny("END", time.Now())
		}()

		var openErr error
		dFile, openErr = os.OpenFile(fmt.Sprintf("./debug_mcp_stdio_%s.txt", workItemID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	addToolProxy(server, tnWorkitem, WorkitemHandler, session, workItemID)
	addToolProxy(server, tnWorkitemMove, WorkitemMoveHandler, session, workItemID)
	addToolProxy(server, tnWorkitemCreate, WorkitemCreateHandler, session, workItemID)
	addToolProxy(server, tnWorkitemSearch, WorkitemSearchHandler, session, workItemID)
	addToolProxy(server, tnWorkitemAttrsSet, WorkitemAttrsSetHandler, session, workItemID)
	addToolProxy(server, tnWorkitemAttrsDel, WorkitemAttrsDelHandler, session, workItemID)
	addToolProxy(server, tnWorkitemAttrsSchema, WorkitemAttrSchemaHandler, session, workItemID)
	addToolProxy(server, tnWorkitemDependencies, WorkitemDependenciesHandler, session, workItemID)
	addToolProxy(server, tnLaneList, LaneListHandler, session, workItemID)
	addToolProxy(server, tnProjectInfo, ProjectInfoHandler, session, workItemID)

	// Filesystem tools (implement SetWorkItemID as no-op)
	addToolProxy(server, tnFsMove, FsMoveHandler, session, workItemID)
	addToolProxy(server, tnFsCopy, FsCopyHandler, session, workItemID)
	addToolProxy(server, tnFsList, FsListHandler, session, workItemID)
	addToolProxy(server, tnFsTree, FsTreeHandler, session, workItemID)
	addToolProxy(server, tnFsFind, FsFindHandler, session, workItemID)
	addToolProxy(server, tnFsGrep, FsGrepHandler, session, workItemID)
	addToolProxy(server, tnFsDiff, FsDiffHandler, session, workItemID)
	if DEBUG_STDIO {
		debugAny("MCP_TOOLS_ADDED", time.Now())
	}

	// Run the server over stdin/stdout, until the client disconnects.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		panic(err)
	}
}

func addToolProxy[In InputWithWorkItemID, Out any](
	s *mcp.Server, tool string, h ToolProjectHandler[In, Out], cs *mcp.ClientSession, workItemID string,
) {
	mcp.AddTool(s, tools[tool], sseProxy(tool, projectHandler2MCP(nil, h), cs, workItemID))
}

func sseProxy[In InputWithWorkItemID, Out any](
	tool string, _ mcp.ToolHandlerFor[In, Out], cs *mcp.ClientSession, workItemID string,
) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		input.SetWorkItemID(workItemID)

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
