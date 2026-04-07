package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

type upsertWindowRequest struct {
	Project     string `json:"project"`
	WindowUUID  string `json:"window_uuid"`
	AgentID     string `json:"agent_id"`
	DisplayName string `json:"display_name"`
}

type windowResponse struct {
	ID           string  `json:"id"`
	Project      string  `json:"project"`
	WindowUUID   string  `json:"window_uuid"`
	AgentID      string  `json:"agent_id"`
	DisplayName  string  `json:"display_name"`
	CreatedAt    string  `json:"created_at"`
	LastActiveAt string  `json:"last_active_at"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
}

type windowListResponse struct {
	Windows []windowResponse `json:"windows"`
}

func toWindowResponse(wi core.WindowIdentity) windowResponse {
	wr := windowResponse{
		ID:           wi.ID,
		Project:      wi.Project,
		WindowUUID:   wi.WindowUUID,
		AgentID:      wi.AgentID,
		DisplayName:  wi.DisplayName,
		CreatedAt:    wi.CreatedAt.Format(time.RFC3339Nano),
		LastActiveAt: wi.LastActiveAt.Format(time.RFC3339Nano),
	}
	if wi.ExpiresAt != nil {
		s := wi.ExpiresAt.Format(time.RFC3339Nano)
		wr.ExpiresAt = &s
	}
	return wr
}

// handleWindows handles POST /api/windows (upsert) and GET /api/windows?project=X (list).
func (s *Service) handleWindows(w http.ResponseWriter, r *http.Request) {
	dispatchByMethod(w, r, methodHandlers{
		get:  s.listWindows,
		post: s.upsertWindow,
	})
}

// handleWindowByID handles DELETE /api/windows/{window_uuid}?project=X (expire).
func (s *Service) handleWindowByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	windowUUID := strings.TrimPrefix(r.URL.Path, "/api/windows/")
	windowUUID = strings.TrimRight(windowUUID, "/")
	if windowUUID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "window_uuid required"})
		return
	}
	project := r.URL.Query().Get("project")
	if project == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "project query param required"})
		return
	}
	if err := s.store.ExpireWindowIdentity(r.Context(), project, windowUUID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "expired"})
}

func (s *Service) upsertWindow(w http.ResponseWriter, r *http.Request) {
	var req upsertWindowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Project == "" || req.WindowUUID == "" || req.AgentID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "project, window_uuid, and agent_id are required"})
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.AgentID
	}
	wi := core.WindowIdentity{
		Project:     req.Project,
		WindowUUID:  req.WindowUUID,
		AgentID:     req.AgentID,
		DisplayName: req.DisplayName,
	}
	result, err := s.store.UpsertWindowIdentity(r.Context(), wi)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if result == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "upsert returned no result"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toWindowResponse(*result))
}

func (s *Service) listWindows(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "project query param required"})
		return
	}
	identities, err := s.store.ListWindowIdentities(r.Context(), project)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	resp := windowListResponse{Windows: make([]windowResponse, len(identities))}
	for i, wi := range identities {
		resp.Windows[i] = toWindowResponse(wi)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
