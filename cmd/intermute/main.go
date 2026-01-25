package main

import (
	"log"

	"github.com/mistakeknot/intermute/internal/server"
)

func main() {
	srv, err := server.New(server.Config{Addr: ":7338"})
	if err != nil {
		log.Fatalf("server init failed: %v", err)
	}
	if err := srv.Start(); err != nil {
		log.Fatalf("server start failed: %v", err)
	}
}
