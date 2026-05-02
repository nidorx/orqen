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

type InputWithJobId interface {
	SetJobId(jobId string)
}

var (
	DEBUG_STDIO = false
	dFile       *os.File
)

func StartStdio(orqenPort string, projectId string, jobId string) {

	if DEBUG_STDIO {
		if r := recover(); r != nil {
			debugAny("PANIC_RECOVER", r)
			panic(r)
		}

		defer func() {
			debugAny("END", time.Now())
		}()

		var openErr error
		dFile, openErr = os.OpenFile(fmt.Sprintf("./debug_mcp_stdio_%s.txt", jobId), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	if DEBUG_STDIO {
		debugAny("MCP_TOOLS_ADDED", time.Now())
	}

	// Run the server over stdin/stdout, until the client disconnects.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		panic(err)
	}
}

func addToolProxy[In InputWithJobId, Out any](s *mcp.Server, tool string, h ToolProjectHandler[In, Out], cs *mcp.ClientSession, jobId string) {
	mcp.AddTool(s, tools[tool], sseProxy(tool, projectHandler2MCP(nil, h), cs, jobId))
}

func sseProxy[In InputWithJobId, Out any](tool string, _ mcp.ToolHandlerFor[In, Out], cs *mcp.ClientSession, jobId string) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		input.SetJobId(jobId)

		if DEBUG_STDIO {
			debugAny("CALL_TOOL_REQUEST", req)
		}

		result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: input})
		if err != nil && DEBUG_STDIO {
			debugError(err)
		}

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
