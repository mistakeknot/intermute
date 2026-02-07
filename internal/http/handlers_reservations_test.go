package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistakeknot/intermute/internal/auth"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
)

func TestReleaseReservationOwnershipEnforced(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	ring := auth.NewKeyring(true, map[string]string{"secret": "proj-a"})
	h := NewRouter(svc, nil, auth.Middleware(ring))

	createBody, _ := json.Marshal(map[string]any{
		"agent_id":     "agent-a",
		"project":      "proj-a",
		"path_pattern": "internal/http/*.go",
		"exclusive":    true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/reservations", bytes.NewReader(createBody))
	createReq.RemoteAddr = "203.0.113.10:9999"
	createReq.Header.Set("Authorization", "Bearer secret")
	createReq.Header.Set("X-Agent-ID", "agent-a")
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	h.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create reservation expected 200, got %d", createResp.Code)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected reservation id")
	}

	forbiddenReq := httptest.NewRequest(http.MethodDelete, "/api/reservations/"+created.ID, nil)
	forbiddenReq.RemoteAddr = "203.0.113.10:9999"
	forbiddenReq.Header.Set("Authorization", "Bearer secret")
	forbiddenReq.Header.Set("X-Agent-ID", "agent-b")
	forbiddenResp := httptest.NewRecorder()
	h.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("cross-agent release expected 403, got %d", forbiddenResp.Code)
	}

	okReq := httptest.NewRequest(http.MethodDelete, "/api/reservations/"+created.ID, nil)
	okReq.RemoteAddr = "203.0.113.10:9999"
	okReq.Header.Set("Authorization", "Bearer secret")
	okReq.Header.Set("X-Agent-ID", "agent-a")
	okResp := httptest.NewRecorder()
	h.ServeHTTP(okResp, okReq)
	if okResp.Code != http.StatusOK {
		t.Fatalf("owner release expected 200, got %d", okResp.Code)
	}

	missingReq := httptest.NewRequest(http.MethodDelete, "/api/reservations/does-not-exist", nil)
	missingReq.RemoteAddr = "203.0.113.10:9999"
	missingReq.Header.Set("Authorization", "Bearer secret")
	missingReq.Header.Set("X-Agent-ID", "agent-a")
	missingResp := httptest.NewRecorder()
	h.ServeHTTP(missingResp, missingReq)
	if missingResp.Code != http.StatusNotFound {
		t.Fatalf("missing reservation expected 404, got %d", missingResp.Code)
	}
}
