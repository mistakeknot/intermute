package main

import (
	"log"

	"github.com/mistakeknot/intermute/internal/auth"
	httpapi "github.com/mistakeknot/intermute/internal/http"
	"github.com/mistakeknot/intermute/internal/server"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
	"github.com/mistakeknot/intermute/internal/ws"
)

func main() {
	store, err := sqlite.New("intermute.db")
	if err != nil {
		log.Fatalf("store init failed: %v", err)
	}
	keyring, err := auth.LoadKeyringFromEnv()
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}
	hub := ws.NewHub()
	svc := httpapi.NewService(store).WithBroadcaster(hub)
	router := httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(keyring))

	srv, err := server.New(server.Config{Addr: ":7338", Handler: router})
	if err != nil {
		log.Fatalf("server init failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("server start failed: %v", err)
	}
}
