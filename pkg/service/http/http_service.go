package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/nidorx/orqen/pkg/chat"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
	project "github.com/nidorx/orqen/pkg/engine"
	"github.com/nidorx/orqen/pkg/mcp"
)

type Service struct {
	server *http.Server
	mux    *http.ServeMux
	port   int
}

// Port returns the HTTP server port.
func (s *Service) Port() int { return s.port }

// RegisterRoute registers an additional HTTP handler on the server's mux.
func (s *Service) RegisterRoute(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *Service) Name() string {
	return "HttpService"
}

func (s *Service) OnStart() error {

	go func() { _ = s.server.ListenAndServe() }()

	return nil
}

func (s *Service) OnStop() error {
	return s.server.Shutdown(context.Background())
}

func New() *Service {

	cfg := conf.GetHttpServer()
	addr := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
	mux := http.NewServeMux()

	svc := &Service{
		server: &http.Server{
			Addr:           addr,
			Handler:        mux,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: 1 << 20,
		},
		mux:  mux,
		port: cfg.Port,
	}

	var (
		orqenMcpServersMu sync.Mutex
		orqenMcpServers   = map[*engine.Project]http.Handler{}
	)

	// prepared for multi projects (future)
	mux.Handle("/mcp/http/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.TrimPrefix(r.URL.Path, "/mcp/http/"); path == "" {
			http.Error(w, "missing project_id", http.StatusBadRequest)
		} else {

			// projectID = dir hash
			projectID := strings.Split(path, "/")[0]

			if proj := project.Get(projectID); proj != nil {

				orqenMcpServersMu.Lock()
				server, exists := orqenMcpServers[proj]
				if !exists {
					if r := recover(); r != nil {
						orqenMcpServersMu.Unlock()
						http.Error(w, "internal server error", http.StatusInternalServerError)
					}
					server = mcp.ServerHttp(proj)
					orqenMcpServers[proj] = server
				}
				orqenMcpServersMu.Unlock()

				server.ServeHTTP(w, r)
			} else {
				http.Error(w, "project not found", http.StatusNotFound)
			}
		}
	}))

	var (
		chatMcpServersMu sync.Mutex
		chatMcpServers   = map[string]http.Handler{}
	)

	// Chat MCP route: /chat/mcp/{projectID}/...
	mux.Handle("/chat/mcp/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.TrimPrefix(r.URL.Path, "/chat/mcp/"); path == "" {
			http.Error(w, "missing project_id", http.StatusBadRequest)
		} else {
			// projectID = dir hash
			projectID := strings.Split(path, "/")[0]

			if chatsrv := chat.Get(projectID); chatsrv != nil {

				// .GetMCPHandler().ServeHTTP(w, r)

				chatMcpServersMu.Lock()
				handler, exists := chatMcpServers[projectID]
				if !exists {
					if r := recover(); r != nil {
						chatMcpServersMu.Unlock()
						http.Error(w, "internal server error", http.StatusInternalServerError)
					}
					handler = chatsrv.GetMCPHandler()
					chatMcpServers[projectID] = handler
				}
				chatMcpServersMu.Unlock()

				handler.ServeHTTP(w, r)
			} else {
				http.Error(w, "project not found", http.StatusNotFound)
			}
		}
	}))

	// Register chat MCP route with HttpService after all services started
	// if _chatService != nil && _chatService.GetMCPHandler() != nil {
	// 	projectID := _chatService.GetProjectID()
	// 	_httpService.RegisterRoute("/chat/mcp/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 		if path := strings.TrimPrefix(r.URL.Path, "/chat/mcp/"); path == "" {
	// 			http.Error(w, "missing project_id", http.StatusBadRequest)
	// 		} else {
	// 			reqProjectID := strings.Split(path, "/")[0]
	// 			if reqProjectID == projectID {
	// 				_chatService.GetMCPHandler().ServeHTTP(w, r)
	// 			} else {
	// 				http.Error(w, "project not found", http.StatusNotFound)
	// 			}
	// 		}
	// 	}))

	// 	log.Printf("[service] Chat MCP available at http://127.0.0.1:%d/chat/mcp/%s", _httpService.Port(), projectID)
	// }

	return svc
}

// extractProjectIDFromPath extracts the project ID from paths like /chat/mcp/{projectID}/...
func extractProjectIDFromPath(path string) string {
	// Strip query string if present
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}
	parts := strings.Split(strings.TrimPrefix(path, "/chat/mcp/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
