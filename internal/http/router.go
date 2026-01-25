package httpapi

import "net/http"

func NewRouter(svc *Service, wsHandler http.Handler, mw func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	wrap := func(h http.HandlerFunc) http.Handler {
		handler := http.Handler(h)
		if mw != nil {
			handler = mw(handler)
		}
		return handler
	}
	mux.Handle("/api/agents", wrap(svc.handleAgents))
	mux.Handle("/api/agents/", wrap(svc.handleAgentHeartbeat))
	mux.Handle("/api/messages", wrap(svc.handleSendMessage))
	mux.Handle("/api/messages/", wrap(svc.handleMessageAction))
	mux.Handle("/api/inbox/", wrap(svc.handleInbox))
	mux.Handle("/api/threads", wrap(svc.handleListThreads))
	mux.Handle("/api/threads/", wrap(svc.handleThreadMessages))
	if wsHandler != nil {
		if mw != nil {
			mux.Handle("/ws/agents/", mw(wsHandler))
		} else {
			mux.Handle("/ws/agents/", wsHandler)
		}
	}
	return mux
}
