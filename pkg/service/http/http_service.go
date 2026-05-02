package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

type Service struct {
	server *http.Server
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

	var (
		cfg          = conf.GetHttpServer()
		addr         = fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		mux          = http.NewServeMux()
		mcpServersMu sync.Mutex
		mcpServers   = map[*project.Project]http.Handler{}
	)
	// prepared for multi projects (future)
	mux.Handle("/mcp/http/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.TrimPrefix(r.URL.Path, "/mcp/http/"); path == "" {
			http.Error(w, "missing project_id", http.StatusBadRequest)
		} else {

			// projectID = dir hash
			projectID := strings.Split(path, "/")[0]

			if proj := project.Get(projectID); proj != nil {

				mcpServersMu.Lock()
				server, exists := mcpServers[proj]
				if !exists {
					if r := recover(); r != nil {
						mcpServersMu.Unlock()
						http.Error(w, "internal server error", http.StatusInternalServerError)
					}
					server := mcp.ServerHttp(proj)
					mcpServers[proj] = server
				}
				mcpServersMu.Unlock()

				server.ServeHTTP(w, r)
			} else {
				http.Error(w, "project not found", http.StatusNotFound)
			}
		}
	}))

	return &Service{
		server: &http.Server{
			Addr:           addr,
			Handler:        mux,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: 1 << 20,
		},
	}
}
