package httpapi

import "net/http"

// NewDomainRouter creates a router with both messaging and domain endpoints
func NewDomainRouter(svc *DomainService, wsHandler http.Handler, mw func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	wrap := func(h http.HandlerFunc) http.Handler {
		handler := http.Handler(h)
		if mw != nil {
			handler = mw(handler)
		}
		return handler
	}

	// Existing messaging endpoints
	mux.Handle("/api/agents", wrap(svc.handleAgents))
	mux.Handle("/api/agents/", wrap(svc.handleAgentHeartbeat))
	mux.Handle("/api/messages", wrap(svc.handleSendMessage))
	mux.Handle("/api/messages/", wrap(svc.handleMessageAction))
	mux.Handle("/api/inbox/", wrap(svc.handleInbox))
	mux.Handle("/api/threads", wrap(svc.handleListThreads))
	mux.Handle("/api/threads/", wrap(svc.handleThreadMessages))

	// Domain endpoints
	mux.Handle("/api/specs", wrap(svc.handleSpecs))
	mux.Handle("/api/specs/", wrap(svc.handleSpecByID))
	mux.Handle("/api/epics", wrap(svc.handleEpics))
	mux.Handle("/api/epics/", wrap(svc.handleEpicByID))
	mux.Handle("/api/stories", wrap(svc.handleStories))
	mux.Handle("/api/stories/", wrap(svc.handleStoryByID))
	mux.Handle("/api/tasks", wrap(svc.handleTasks))
	mux.Handle("/api/tasks/", wrap(svc.handleTaskByID))
	mux.Handle("/api/insights", wrap(svc.handleInsights))
	mux.Handle("/api/insights/", wrap(svc.handleInsightByID))
	mux.Handle("/api/sessions", wrap(svc.handleSessions))
	mux.Handle("/api/sessions/", wrap(svc.handleSessionByID))
	mux.Handle("/api/cujs", wrap(svc.handleCUJs))
	mux.Handle("/api/cujs/", wrap(svc.handleCUJByID))

	// WebSocket
	if wsHandler != nil {
		if mw != nil {
			mux.Handle("/ws/agents/", mw(wsHandler))
		} else {
			mux.Handle("/ws/agents/", wsHandler)
		}
	}

	return mux
}
