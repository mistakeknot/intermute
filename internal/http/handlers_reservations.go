package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/core"
)

type reservationRequest struct {
	AgentID     string `json:"agent_id"`
	Project     string `json:"project"`
	PathPattern string `json:"path_pattern"`
	Exclusive   bool   `json:"exclusive"`
	Reason      string `json:"reason"`
	TTLMinutes  int    `json:"ttl_minutes"` // TTL in minutes
}

type apiReservation struct {
	ID          string  `json:"id"`
	AgentID     string  `json:"agent_id"`
	Project     string  `json:"project"`
	PathPattern string  `json:"path_pattern"`
	Exclusive   bool    `json:"exclusive"`
	Reason      string  `json:"reason,omitempty"`
	CreatedAt   string  `json:"created_at"`
	ExpiresAt   string  `json:"expires_at"`
	ReleasedAt  *string `json:"released_at,omitempty"`
	IsActive    bool    `json:"is_active"`
}

type reservationsResponse struct {
	Reservations []apiReservation `json:"reservations"`
}

func toAPIReservation(r core.Reservation) apiReservation {
	api := apiReservation{
		ID:          r.ID,
		AgentID:     r.AgentID,
		Project:     r.Project,
		PathPattern: r.PathPattern,
		Exclusive:   r.Exclusive,
		Reason:      r.Reason,
		CreatedAt:   r.CreatedAt.Format(time.RFC3339Nano),
		ExpiresAt:   r.ExpiresAt.Format(time.RFC3339Nano),
		IsActive:    r.IsActive(),
	}
	if r.ReleasedAt != nil {
		s := r.ReleasedAt.Format(time.RFC3339Nano)
		api.ReleasedAt = &s
	}
	return api
}

// ReservationStore is the subset of Store methods needed for reservation handlers
type ReservationStore interface {
	Reserve(ctx context.Context, r core.Reservation) (*core.Reservation, error)
	GetReservation(ctx context.Context, id string) (*core.Reservation, error)
	ReleaseReservation(ctx context.Context, id, agentID string) error
	ActiveReservations(ctx context.Context, project string) ([]core.Reservation, error)
	AgentReservations(ctx context.Context, agentID string) ([]core.Reservation, error)
	CheckConflicts(ctx context.Context, project, pathPattern string, exclusive bool) ([]core.ConflictDetail, error)
}

func (s *Service) handleReservations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listReservations(w, r)
	case http.MethodPost:
		s.createReservation(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleReservationByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Extract ID from path: /api/reservations/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/reservations/")
	id := strings.Trim(path, "/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.releaseReservation(w, r, id)
}

func (s *Service) createReservation(w http.ResponseWriter, r *http.Request) {
	var req reservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.AgentID == "" || req.PathPattern == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	info, _ := auth.FromContext(r.Context())
	project := req.Project
	if project == "" {
		project = info.Project
	}
	if info.Mode == auth.ModeAPIKey && project != info.Project {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	ttl := 30 * time.Minute
	if req.TTLMinutes > 0 {
		ttl = time.Duration(req.TTLMinutes) * time.Minute
	}

	res, err := s.store.Reserve(r.Context(), core.Reservation{
		AgentID:     req.AgentID,
		Project:     project,
		PathPattern: req.PathPattern,
		Exclusive:   req.Exclusive,
		Reason:      req.Reason,
		TTL:         ttl,
	})
	if err != nil {
		var conflictErr *core.ConflictError
		if errors.As(err, &conflictErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":     "reservation_conflict",
				"conflicts": conflictErr.Conflicts,
			})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPIReservation(*res))
}

func (s *Service) listReservations(w http.ResponseWriter, r *http.Request) {
	info, _ := auth.FromContext(r.Context())
	project := r.URL.Query().Get("project")
	if project == "" {
		project = info.Project
	}
	agentID := r.URL.Query().Get("agent")

	var reservations []core.Reservation
	var err error

	if agentID != "" {
		reservations, err = s.store.AgentReservations(r.Context(), agentID)
	} else if project != "" {
		reservations, err = s.store.ActiveReservations(r.Context(), project)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	apiRes := make([]apiReservation, 0, len(reservations))
	for _, r := range reservations {
		apiRes = append(apiRes, toAPIReservation(r))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reservationsResponse{Reservations: apiRes})
}

func (s *Service) checkConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	project := r.URL.Query().Get("project")
	pattern := r.URL.Query().Get("pattern")
	if project == "" || pattern == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	exclusive := r.URL.Query().Get("exclusive") != "false" // default true

	conflicts, err := s.store.CheckConflicts(r.Context(), project, pattern, exclusive)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"conflicts": conflicts,
	})
}

func (s *Service) releaseReservation(w http.ResponseWriter, r *http.Request, id string) {
	reservation, err := s.store.GetReservation(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	info, _ := auth.FromContext(r.Context())
	if reservation.AgentID != info.AgentID {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := s.store.ReleaseReservation(r.Context(), id, info.AgentID); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}
