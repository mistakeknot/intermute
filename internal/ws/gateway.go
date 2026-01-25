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
	mu    sync.Mutex
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

func (h *Hub) Broadcast(project, agent string, event any) {
	conns := h.snapshot(project, agent)
	if len(conns) == 0 {
		return
	}
	for _, conn := range conns {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		_ = wsjson.Write(ctx, conn, event)
		cancel()
	}
}

func (h *Hub) snapshot(project, agent string) []*websocket.Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*websocket.Conn
	collectAgent := func(m map[string]map[*websocket.Conn]struct{}, target string) {
		if target == "" {
			for _, conns := range m {
				for conn := range conns {
					out = append(out, conn)
				}
			}
			return
		}
		for conn := range m[target] {
			out = append(out, conn)
		}
	}
	if project != "" {
		if perAgent, ok := h.conns[project]; ok {
			collectAgent(perAgent, agent)
		}
		return out
	}
	for _, perAgent := range h.conns {
		collectAgent(perAgent, agent)
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
