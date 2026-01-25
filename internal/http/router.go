package httpapi

import (
	"net/http"
)

func NewRouter(svc *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents", svc.handleRegisterAgent)
	mux.HandleFunc("/api/agents/", svc.handleAgentHeartbeat)
	mux.HandleFunc("/api/messages", svc.handleSendMessage)
	mux.HandleFunc("/api/messages/", svc.handleMessageAction)
	mux.HandleFunc("/api/inbox/", svc.handleInbox)
	return mux
}
