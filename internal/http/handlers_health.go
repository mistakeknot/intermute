package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type healthChecker interface {
	HealthCheck(context.Context) error
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	if err := s.checkHealth(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Service) checkHealth(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("storage unavailable")
	}
	if checker, ok := s.store.(healthChecker); ok {
		if err := checker.HealthCheck(ctx); err != nil {
			return fmt.Errorf("storage health check: %w", err)
		}
		return nil
	}
	if _, err := s.store.ListAgents(ctx, "", nil); err != nil {
		return fmt.Errorf("storage health check: %w", err)
	}
	return nil
}
