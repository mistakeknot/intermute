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

func TestHeartbeatAuthEnforcement(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	ring := auth.NewKeyring(true, map[string]string{"secret-a": "proj-a", "secret-b": "proj-b"})
	h := NewRouter(svc, nil, auth.Middleware(ring))

	// Register an agent in proj-a
	registerReq := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader([]byte(`{"name":"agent-x","project":"proj-a"}`)))
	registerReq.RemoteAddr = "203.0.113.10:9999"
	registerReq.Header.Set("Authorization", "Bearer secret-a")
	registerResp := httptest.NewRecorder()
	h.ServeHTTP(registerResp, registerReq)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register failed: %d", registerResp.Code)
	}
	var regResult map[string]any
	_ = json.NewDecoder(registerResp.Body).Decode(&regResult)
	agentID := regResult["agent_id"].(string)

	// Heartbeat with correct project key should succeed
	hbReq := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID+"/heartbeat", nil)
	hbReq.RemoteAddr = "203.0.113.10:9999"
	hbReq.Header.Set("Authorization", "Bearer secret-a")
	hbResp := httptest.NewRecorder()
	h.ServeHTTP(hbResp, hbReq)
	if hbResp.Code != http.StatusOK {
		t.Fatalf("heartbeat with correct project expected 200, got %d", hbResp.Code)
	}

	// Heartbeat with wrong project key should fail
	hbReq2 := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID+"/heartbeat", nil)
	hbReq2.RemoteAddr = "203.0.113.10:9999"
	hbReq2.Header.Set("Authorization", "Bearer secret-b")
	hbResp2 := httptest.NewRecorder()
	h.ServeHTTP(hbResp2, hbReq2)
	if hbResp2.Code != http.StatusNotFound {
		t.Fatalf("heartbeat with wrong project expected 404, got %d", hbResp2.Code)
	}

	// Heartbeat from localhost (no auth required) should succeed
	hbReq3 := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID+"/heartbeat", nil)
	hbReq3.RemoteAddr = "127.0.0.1:9999"
	hbResp3 := httptest.NewRecorder()
	h.ServeHTTP(hbResp3, hbReq3)
	if hbResp3.Code != http.StatusOK {
		t.Fatalf("heartbeat from localhost expected 200, got %d", hbResp3.Code)
	}
}
