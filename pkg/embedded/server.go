// Package embedded provides an embeddable Intermute server for in-process use.
package embedded

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	httpapi "github.com/mistakeknot/intermute/internal/http"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
	"github.com/mistakeknot/intermute/internal/ws"
)

// Config configures the embedded server
type Config struct {
	// DBPath is the path to the SQLite database file.
	// If empty, defaults to ~/.autarch/data.db
	DBPath string

	// Port is the HTTP port to listen on.
	// If 0, defaults to 7338.
	Port int

	// Host is the host to bind to.
	// If empty, defaults to localhost (127.0.0.1).
	Host string
}

// Server is an embedded Intermute server
type Server struct {
	cfg     Config
	store   *sqlite.Store
	hub     *ws.Hub
	http    *http.Server
	started bool
	mu      sync.Mutex
}

// New creates a new embedded Intermute server
func New(cfg Config) (*Server, error) {
	// Apply defaults
	if cfg.DBPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		cfg.DBPath = filepath.Join(home, ".autarch", "data.db")
	}
	if cfg.Port == 0 {
		cfg.Port = 7338
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	// Initialize store
	store, err := sqlite.New(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	// Create WebSocket hub
	hub := ws.NewHub()

	// Create domain service (supports both messaging and domain APIs)
	svc := httpapi.NewDomainService(store).WithBroadcaster(hub)

	// Create router - no auth for embedded use
	router := httpapi.NewDomainRouter(svc, hub.Handler(), nil)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	return &Server{
		cfg:   cfg,
		store: store,
		hub:   hub,
		http:  httpServer,
	}, nil
}

// Start starts the embedded server in a goroutine
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash - the main app should handle this
			fmt.Fprintf(os.Stderr, "intermute server error: %v\n", err)
		}
	}()

	// Wait a moment for the server to start
	time.Sleep(50 * time.Millisecond)
	return nil
}

// Stop stops the embedded server gracefully
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

// Addr returns the server's listen address
func (s *Server) Addr() string {
	return s.http.Addr
}

// URL returns the base URL for the server
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s", s.http.Addr)
}

// Store returns the underlying store for direct access if needed
func (s *Server) Store() *sqlite.Store {
	return s.store
}

// NewWithAuth creates an embedded server with API key authentication enabled
func NewWithAuth(cfg Config) (*Server, error) {
	// Apply defaults
	if cfg.DBPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		cfg.DBPath = filepath.Join(home, ".autarch", "data.db")
	}
	if cfg.Port == 0 {
		cfg.Port = 7338
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	// Initialize store
	store, err := sqlite.New(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	// Load keyring from environment
	keyring, err := auth.LoadKeyringFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load auth: %w", err)
	}

	// Create WebSocket hub
	hub := ws.NewHub()

	// Create domain service
	svc := httpapi.NewDomainService(store).WithBroadcaster(hub)

	// Create router with auth middleware
	router := httpapi.NewDomainRouter(svc, hub.Handler(), auth.Middleware(keyring))

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	return &Server{
		cfg:   cfg,
		store: store,
		hub:   hub,
		http:  httpServer,
	}, nil
}
