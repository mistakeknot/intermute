package ws

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const writeTimeout = 5 * time.Second

type Hub struct {
	mu    sync.Mutex
	conns map[string]map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]map[*websocket.Conn]struct{})}
}

func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/ws/agents/")
		agent := strings.Trim(path, "/")
		if agent == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		h.add(agent, conn)
		defer h.remove(agent, conn)

		ctx := r.Context()
		for {
			var v any
			if err := wsjson.Read(ctx, conn, &v); err != nil {
				return
			}
		}
	}
}

func (h *Hub) Broadcast(agent string, event any) {
	conns := h.snapshot(agent)
	if len(conns) == 0 {
		return
	}
	for _, conn := range conns {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		_ = wsjson.Write(ctx, conn, event)
		cancel()
	}
}

func (h *Hub) snapshot(agent string) []*websocket.Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*websocket.Conn
	if agent != "" {
		for conn := range h.conns[agent] {
			out = append(out, conn)
		}
		return out
	}
	for _, m := range h.conns {
		for conn := range m {
			out = append(out, conn)
		}
	}
	return out
}

func (h *Hub) add(agent string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.conns[agent]
	if !ok {
		m = make(map[*websocket.Conn]struct{})
		h.conns[agent] = m
	}
	m[conn] = struct{}{}
}

func (h *Hub) remove(agent string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.conns[agent]
	if !ok {
		return
	}
	delete(m, conn)
	if len(m) == 0 {
		delete(h.conns, agent)
	}
}
