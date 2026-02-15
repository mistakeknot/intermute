package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
)

type Config struct {
	Addr       string
	SocketPath string
	Handler    http.Handler
}

type Server struct {
	cfg    Config
	http   *http.Server
	unix   *http.Server
	unixLn net.Listener
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
	s := &Server{cfg: cfg, http: srv}

	if cfg.SocketPath != "" {
		// Remove stale socket file from previous run
		if err := os.Remove(cfg.SocketPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
		ln, err := net.Listen("unix", cfg.SocketPath)
		if err != nil {
			return nil, fmt.Errorf("unix listen: %w", err)
		}
		if err := os.Chmod(cfg.SocketPath, 0660); err != nil {
			ln.Close()
			return nil, fmt.Errorf("chmod socket: %w", err)
		}
		s.unixLn = ln
		s.unix = &http.Server{Handler: h}
	}

	return s, nil
}

func (s *Server) Start() error {
	if s.unixLn != nil {
		go s.unix.Serve(s.unixLn)
	}
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	var firstErr error

	if s.unix != nil {
		if err := s.unix.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.cfg.SocketPath != "" {
		os.Remove(s.cfg.SocketPath)
	}

	if err := s.http.Shutdown(ctx); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// SocketPath returns the configured socket path, or empty if not configured.
func (s *Server) SocketPath() string {
	return s.cfg.SocketPath
}
