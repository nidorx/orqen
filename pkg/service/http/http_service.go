package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/nidorx/orqen/pkg/cli"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/mcp"
	"github.com/nidorx/orqen/pkg/project"
)

var messages = cli.Messages{
	"pt-BR": {
		"listening": "O serviço de integração HTTP está disponível em http://%s:%d",
	},
	"en": {
		"listening": "The HTTP integration service is available at http://%s:%d",
	},
}

type Service struct {
	server *http.Server
}

func (s *Service) Name() string {
	return "HttpService"
}

func (s *Service) OnStart() error {

	go func() { _ = s.server.ListenAndServe() }()

	cfg := conf.GetHttpServer()
	cli.Printf(messages, "listening", cfg.IP, cfg.Port)
	return nil
}

func (s *Service) OnStop() error {
	return s.server.Shutdown(context.Background())
}

func New() *Service {
	cfg := conf.GetHttpServer()
	addr := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)

	mux := http.NewServeMux()

	// prepared for multi projects (future)
	mux.Handle("/mcp/http/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := strings.TrimPrefix(r.URL.Path, "/mcp/http/"); path == "" {
			http.Error(w, "missing project_id", http.StatusBadRequest)
		} else {

			// projectID = dir hash
			projectID := strings.Split(path, "/")[0]

			if proj := project.Get(projectID); proj == nil {
				mcp.ServerHttp(proj)
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
