package chat

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
)

// NewChatMCPServer creates an HTTP handler that exposes all chat MCP tools
// at /chat/mcp/{projectID}/... via Streamable HTTP transport.
//
// The returned handler reuses the same server instance for all requests,
// following the same pattern as pkg/mcp/server_http.go.
func NewChatMCPServer(proj *engine.Project, chatStore *ChatStore, sessionMgr *SessionManager) http.Handler {
	server := createChatServer(proj, chatStore, sessionMgr)

	return mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, nil)
}

// createChatServer builds an mcp.Server with all chat tools registered.
func createChatServer(proj *engine.Project, chatStore *ChatStore, sessionMgr *SessionManager) *mcp.Server {
	info := &mcp.Implementation{
		Name:    "orqen-chat",
		Title:   "Orqen Chat MCP Server",
		Version: "0.1.0",
	}

	if ci := conf.GetInfo(); ci != nil {
		info.Version = ci.Version
	}

	server := mcp.NewServer(info, nil)

	RegisterAllTools(server, proj, chatStore, sessionMgr)

	return server
}
