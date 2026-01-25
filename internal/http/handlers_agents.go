package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
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
	Cursor    uint64 `json:"cursor"`
}

func (s *Service) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req registerAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
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
	agent, err := s.store.RegisterAgent(core.Agent{
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
	agent, err := s.store.Heartbeat(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"agent_id": agent.ID})
}
