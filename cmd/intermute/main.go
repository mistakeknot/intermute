package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/cli"
	httpapi "github.com/mistakeknot/intermute/internal/http"
	"github.com/mistakeknot/intermute/internal/server"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
	"github.com/mistakeknot/intermute/internal/ws"
)

func main() {
	root := &cobra.Command{
		Use:   "intermute",
		Short: "intermute - Agent coordination and domain API server",
	}

	root.AddCommand(serveCmd())
	root.AddCommand(initCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	var (
		port       int
		host       string
		dbPath     string
		socketPath string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the intermute server",
		Long: `Start the intermute HTTP server providing:
  - Agent messaging and coordination APIs
  - Domain APIs (specs, epics, stories, tasks, insights, sessions)
  - WebSocket support for real-time updates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := sqlite.New(dbPath)
			if err != nil {
				return fmt.Errorf("store init: %w", err)
			}

			// Wrap store with circuit breaker + retry resilience
			resilient := sqlite.NewResilient(store)

			// Bootstrap dev key if keys file is missing
			keysPath := auth.ResolveKeysPath()
			bootstrap, err := auth.BootstrapDevKey(keysPath, "dev")
			if err != nil {
				log.Printf("warning: bootstrap failed: %v", err)
			} else if bootstrap.Created {
				log.Printf("generated dev key for project %q", bootstrap.Project)
				log.Printf("  key: %s", bootstrap.Key)
				log.Printf("  file: %s", bootstrap.KeysFile)
			}

			keyring, err := auth.LoadKeyringFromEnv()
			if err != nil {
				return fmt.Errorf("auth init: %w", err)
			}

			hub := ws.NewHub()

			// Start reservation sweeper (60s interval, 5min heartbeat grace)
			sweeper := sqlite.NewSweeper(store, hub, 60*time.Second, 5*time.Minute)
			sweeper.Start(context.Background())

			svc := httpapi.NewDomainService(resilient).WithBroadcaster(hub)
			router := httpapi.NewDomainRouter(svc, hub.Handler(), auth.Middleware(keyring))

			addr := fmt.Sprintf("%s:%d", host, port)
			srv, err := server.New(server.Config{Addr: addr, SocketPath: socketPath, Handler: router})
			if err != nil {
				return fmt.Errorf("server init: %w", err)
			}

			// Handle shutdown signals
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-quit
				log.Println("shutting down...")

				// 1. Stop sweeper
				sweeper.Stop()
				log.Println("sweeper stopped")

				// 2. Drain in-flight HTTP requests
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)

				// 3. Checkpoint WAL and close database
				if err := store.Close(); err != nil {
					log.Printf("store close: %v", err)
				}
				log.Println("database closed")
			}()

			log.Printf("intermute server starting on %s", addr)
			if socketPath != "" {
				log.Printf("intermute unix socket: %s", socketPath)
			}
			if err := srv.Start(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("server: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&port, "port", 7338, "HTTP server port")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "HTTP server bind address")
	cmd.Flags().StringVar(&dbPath, "db", "intermute.db", "SQLite database path")
	cmd.Flags().StringVar(&socketPath, "socket", "", "Unix domain socket path (e.g. /var/run/intermute.sock)")

	return cmd
}

func initCmd() *cobra.Command {
	var (
		project  string
		keysFile string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize auth keys for a project",
		Long: `Creates or updates the keys file with a new API key for the specified project.

The generated key can be used for Bearer authentication when making requests
from outside localhost. Localhost requests are allowed without authentication
by default.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keysFile == "" {
				keysFile = auth.ResolveKeysPath()
			}

			key, err := cli.InitKeysFile(keysFile, project)
			if err != nil {
				return err
			}

			fmt.Printf("Created API key for project %q in %s\n\n", project, keysFile)
			fmt.Printf("Key: %s\n\n", key)
			fmt.Println("To use this key, set the environment variable or add the header:")
			fmt.Printf("  export INTERMUTE_API_KEY=%s\n", key)
			fmt.Printf("  -H \"Authorization: Bearer %s\"\n", key)
			fmt.Println("\nTo start the server with these keys:")
			fmt.Printf("  INTERMUTE_KEYS_FILE=%s intermute serve\n", keysFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name (required)")
	cmd.Flags().StringVar(&keysFile, "keys-file", "", "Path to keys file (default: intermute.keys.yaml)")
	_ = cmd.MarkFlagRequired("project")

	return cmd
}
