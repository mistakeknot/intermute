package ws

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const writeTimeout = 5 * time.Second

type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[string]map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]map[string]map[*websocket.Conn]struct{})}
}

func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/ws/agents/")
		agent := strings.Trim(path, "/")
		if agent == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requestedProject := strings.TrimSpace(r.URL.Query().Get("project"))
		info, _ := auth.FromContext(r.Context())
		project := info.Project
		if info.Mode == auth.ModeAPIKey {
			if requestedProject != "" && requestedProject != project {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		} else if project == "" {
			project = requestedProject
		}
		if info.AgentID != "" && info.AgentID != agent {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}

		h.add(project, agent, conn)
		defer h.remove(project, agent, conn)

		ctx := r.Context()
		for {
			var v any
			if err := wsjson.Read(ctx, conn, &v); err != nil {
				return
			}
		}
	}
}

type connEntry struct {
	conn    *websocket.Conn
	project string
	agent   string
}

func (h *Hub) Broadcast(project, agent string, event any) {
	entries := h.snapshot(project, agent)
	if len(entries) == 0 {
		return
	}
	for _, e := range entries {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		err := wsjson.Write(ctx, e.conn, event)
		cancel()
		if err != nil {
			go func(e connEntry) {
				e.conn.Close(websocket.StatusGoingAway, "write error")
				h.remove(e.project, e.agent, e.conn)
			}(e)
		}
	}
}

func (h *Hub) snapshot(project, agent string) []connEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []connEntry
	collectAgent := func(proj string, m map[string]map[*websocket.Conn]struct{}, target string) {
		if target == "" {
			for agentName, conns := range m {
				for conn := range conns {
					out = append(out, connEntry{conn: conn, project: proj, agent: agentName})
				}
			}
			return
		}
		for conn := range m[target] {
			out = append(out, connEntry{conn: conn, project: proj, agent: target})
		}
	}
	if project != "" {
		if perAgent, ok := h.conns[project]; ok {
			collectAgent(project, perAgent, agent)
		}
		return out
	}
	for proj, perAgent := range h.conns {
		collectAgent(proj, perAgent, agent)
	}
	return out
}

func (h *Hub) add(project, agent string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	perProject, ok := h.conns[project]
	if !ok {
		perProject = make(map[string]map[*websocket.Conn]struct{})
		h.conns[project] = perProject
	}
	perAgent, ok := perProject[agent]
	if !ok {
		perAgent = make(map[*websocket.Conn]struct{})
		perProject[agent] = perAgent
	}
	perAgent[conn] = struct{}{}
}

func (h *Hub) remove(project, agent string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	perProject, ok := h.conns[project]
	if !ok {
		return
	}
	perAgent, ok := perProject[agent]
	if !ok {
		return
	}
	delete(perAgent, conn)
	if len(perAgent) == 0 {
		delete(perProject, agent)
	}
	if len(perProject) == 0 {
		delete(h.conns, project)
	}
}
