package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/names"
)

type registerAgentRequest struct {
	Name         string            `json:"name"`
	Project      string            `json:"project"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata"`
	Status       string            `json:"status"`
}

type registerAgentResponse struct {
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	Cursor    uint64 `json:"cursor"`
}

type listAgentsResponse struct {
	Agents []agentJSON `json:"agents"`
}

type agentJSON struct {
	AgentID      string            `json:"agent_id"`
	SessionID    string            `json:"session_id"`
	Name         string            `json:"name"`
	Project      string            `json:"project"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata"`
	Status       string            `json:"status"`
	LastSeen     string            `json:"last_seen"`
	CreatedAt    string            `json:"created_at"`
}

func (s *Service) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListAgents(w, r)
	case http.MethodPost:
		s.handleRegisterAgent(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleListAgents(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	info, _ := auth.FromContext(r.Context())

	if info.Mode == auth.ModeAPIKey {
		if project == "" {
			project = info.Project
		} else if project != info.Project {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	agents, err := s.store.ListAgents(r.Context(), project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	out := make([]agentJSON, 0, len(agents))
	for _, a := range agents {
		out = append(out, agentJSON{
			AgentID:      a.ID,
			SessionID:    a.SessionID,
			Name:         a.Name,
			Project:      a.Project,
			Capabilities: a.Capabilities,
			Metadata:     a.Metadata,
			Status:       a.Status,
			LastSeen:     a.LastSeen.Format(time.RFC3339),
			CreatedAt:    a.CreatedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listAgentsResponse{Agents: out})
}

func (s *Service) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req registerAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = names.Generate()
	}
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey {
		if strings.TrimSpace(req.Project) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Project != info.Project {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	now := time.Now().UTC()
	agent, err := s.store.RegisterAgent(r.Context(), core.Agent{
		Name:         req.Name,
		Project:      strings.TrimSpace(req.Project),
		Capabilities: req.Capabilities,
		Metadata:     req.Metadata,
		Status:       req.Status,
		CreatedAt:    now,
		LastSeen:     now,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(registerAgentResponse{
		AgentID:   agent.ID,
		SessionID: agent.SessionID,
		Name:      agent.Name,
		Cursor:    0,
	})
}

func (s *Service) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	if !strings.HasSuffix(path, "/heartbeat") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	id := strings.TrimSuffix(path, "/heartbeat")
	id = strings.Trim(id, "/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Enforce project scoping for API key auth
	var project string
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey {
		project = info.Project
	}

	agent, err := s.store.Heartbeat(r.Context(), project, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"agent_id": agent.ID})
}
