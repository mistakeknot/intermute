package httpapi

import (
	"net/http"
)

func NewRouter(svc *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents", svc.handleRegisterAgent)
	mux.HandleFunc("/api/agents/", svc.handleAgentHeartbeat)
	return mux
}
