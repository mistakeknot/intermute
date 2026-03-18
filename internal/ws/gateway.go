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
	mu       sync.RWMutex
	conns    map[string]map[string]map[*websocket.Conn]struct{}
	numConns int // total connection count for pre-allocation
	snapPool sync.Pool
}

func NewHub() *Hub {
	h := &Hub{conns: make(map[string]map[string]map[*websocket.Conn]struct{})}
	h.snapPool.New = func() any {
		return &snapBuf{entries: make([]connEntry, 0, 16)}
	}
	return h
}

// snapBuf is a pooled buffer for snapshot results. Using a struct pointer
// avoids allocating a new *[]connEntry on every Put.
type snapBuf struct {
	entries []connEntry
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
	buf := h.snapshot(project, agent)
	if len(buf.entries) == 0 {
		h.putSnapshot(buf)
		return
	}
	for _, e := range buf.entries {
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
	h.putSnapshot(buf)
}

func (h *Hub) snapshot(project, agent string) *snapBuf {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buf := h.snapPool.Get().(*snapBuf)
	buf.entries = buf.entries[:0]

	// Pre-grow to known total if the pooled slice is too small.
	if cap(buf.entries) < h.numConns {
		buf.entries = make([]connEntry, 0, h.numConns)
	}

	collectAgent := func(proj string, m map[string]map[*websocket.Conn]struct{}, target string) {
		if target == "" {
			for agentName, conns := range m {
				for conn := range conns {
					buf.entries = append(buf.entries, connEntry{conn: conn, project: proj, agent: agentName})
				}
			}
			return
		}
		for conn := range m[target] {
			buf.entries = append(buf.entries, connEntry{conn: conn, project: proj, agent: target})
		}
	}
	if project != "" {
		if perAgent, ok := h.conns[project]; ok {
			collectAgent(project, perAgent, agent)
		}
	} else {
		for proj, perAgent := range h.conns {
			collectAgent(proj, perAgent, agent)
		}
	}

	return buf
}

// putSnapshot returns a snapshot buffer to the pool. Callers must not use
// the buffer after this call.
func (h *Hub) putSnapshot(buf *snapBuf) {
	// Clear references so GC can collect closed conns.
	for i := range buf.entries {
		buf.entries[i] = connEntry{}
	}
	buf.entries = buf.entries[:0]
	h.snapPool.Put(buf)
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
	h.numConns++
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
	h.numConns--
	if len(perAgent) == 0 {
		delete(perProject, agent)
	}
	if len(perProject) == 0 {
		delete(h.conns, project)
	}
}
