package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/names"
)

type registerAgentRequest struct {
	Name         string            `json:"name"`
	SessionID    string            `json:"session_id,omitempty"`
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
		SessionID:    strings.TrimSpace(req.SessionID),
		Project:      strings.TrimSpace(req.Project),
		Capabilities: req.Capabilities,
		Metadata:     req.Metadata,
		Status:       req.Status,
		CreatedAt:    now,
		LastSeen:     now,
	})
	if err != nil {
		if errors.Is(err, core.ErrActiveSessionConflict) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "session_id is in use by an active agent",
				"code":  "active_session_conflict",
			})
			return
		}
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

func (s *Service) handleAgentSubpath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	path = strings.Trim(path, "/")

	// Parse: "<agent-id>/<action>"
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	agentID := parts[0]
	action := parts[1]

	switch action {
	case "heartbeat":
		s.handleAgentHeartbeat(w, r, agentID)
	case "metadata":
		s.handleAgentMetadata(w, r, agentID)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Enforce project scoping for API key auth
	var project string
	info, _ := auth.FromContext(r.Context())
	if info.Mode == auth.ModeAPIKey {
		project = info.Project
	}

	agent, err := s.store.Heartbeat(r.Context(), project, agentID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"agent_id": agent.ID})
}

type updateMetadataRequest struct {
	Metadata map[string]string `json:"metadata"`
}

func (s *Service) handleAgentMetadata(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req updateMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(req.Metadata) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "metadata map required"})
		return
	}

	agent, err := s.store.UpdateAgentMetadata(r.Context(), agentID, req.Metadata)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(agentJSON{
		AgentID:      agent.ID,
		SessionID:    agent.SessionID,
		Name:         agent.Name,
		Project:      agent.Project,
		Capabilities: agent.Capabilities,
		Metadata:     agent.Metadata,
		Status:       agent.Status,
		LastSeen:     agent.LastSeen.Format(time.RFC3339),
		CreatedAt:    agent.CreatedAt.Format(time.RFC3339),
	})
}
