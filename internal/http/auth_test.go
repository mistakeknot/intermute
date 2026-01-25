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

func TestAPIKeyProjectEnforcement(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	ring := auth.NewKeyring(true, map[string]string{"secret": "proj-a"})
	h := NewRouter(svc, nil, auth.Middleware(ring))

	makeReq := func(path string, payload map[string]any) *httptest.ResponseRecorder {
		buf, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(buf))
		req.RemoteAddr = "203.0.113.10:9999"
		req.Header.Set("Authorization", "Bearer secret")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := makeReq("/api/agents", map[string]any{"name": "agent-a"}); rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing project, got %d", rr.Code)
	}
	if rr := makeReq("/api/agents", map[string]any{"name": "agent-a", "project": "proj-b"}); rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for project mismatch, got %d", rr.Code)
	}
	if rr := makeReq("/api/agents", map[string]any{"name": "agent-a", "project": "proj-a"}); rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for project match, got %d", rr.Code)
	}

	if rr := makeReq("/api/messages", map[string]any{"from": "a", "to": []string{"b"}}); rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing project, got %d", rr.Code)
	}
	if rr := makeReq("/api/messages", map[string]any{"project": "proj-b", "from": "a", "to": []string{"b"}}); rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for project mismatch, got %d", rr.Code)
	}
	if rr := makeReq("/api/messages", map[string]any{"project": "proj-a", "from": "a", "to": []string{"b"}}); rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for project match, got %d", rr.Code)
	}
}
