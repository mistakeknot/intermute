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
	mux.Handle("/api/agents/presence", wrap(svc.handleAgentPresence))
	mux.Handle("/api/agents/", wrap(svc.handleAgentSubpath))
	mux.Handle("/api/messages", wrap(svc.handleSendMessage))
	mux.Handle("/api/messages/", wrap(svc.handleMessageAction))
	mux.Handle("/api/inbox/pokes", wrap(svc.handleInboxPokes))
	mux.Handle("/api/inbox/pokes/", wrap(svc.handleInboxPokeAction))
	mux.Handle("/api/inbox/", wrap(svc.handleInbox))
	mux.Handle("/api/threads", wrap(svc.handleListThreads))
	mux.Handle("/api/threads/", wrap(svc.handleThreadMessages))
	mux.Handle("/api/topics/", wrap(svc.handleTopicMessages))
	mux.Handle("/api/broadcast", wrap(svc.handleBroadcast))
	mux.Handle("/api/reservations", wrap(svc.handleReservations))
	mux.Handle("/api/reservations/check", wrap(svc.checkConflicts))
	mux.Handle("/api/reservations/", wrap(svc.handleReservationByID))
	mux.Handle("/api/windows", wrap(svc.handleWindows))
	mux.Handle("/api/windows/", wrap(svc.handleWindowByID))
	if wsHandler != nil {
		if mw != nil {
			mux.Handle("/ws/agents/", mw(wsHandler))
		} else {
			mux.Handle("/ws/agents/", wsHandler)
		}
	}
	return mux
}
