package server

import (
	"context"
	"fmt"
	"net/http"
)

type Config struct {
	Addr string
	Handler http.Handler
}

type Server struct {
	cfg Config
	http *http.Server
}

func New(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("addr required")
	}
	h := cfg.Handler
	if h == nil {
		h = http.NewServeMux()
	}
	srv := &http.Server{Addr: cfg.Addr, Handler: h}
	return &Server{cfg: cfg, http: srv}, nil
}

func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
