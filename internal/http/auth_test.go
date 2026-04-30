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

func TestPresenceAPIKeyProjectEnforcement(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	ring := auth.NewKeyring(true, map[string]string{"secret-a": "proj-a", "secret-b": "proj-b"})
	h := NewRouter(svc, nil, auth.Middleware(ring))

	register := func(token, name, project string) string {
		buf, _ := json.Marshal(map[string]any{
			"name":    name,
			"project": project,
			"metadata": map[string]string{
				"agent_kind":     "codex",
				"repo":           "/home/mk/projects/Sylveste/core/intermute",
				"active_bead_id": "sylveste-kgfi.2",
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(buf))
		req.RemoteAddr = "203.0.113.10:9999"
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("register %s expected 200, got %d", name, rr.Code)
		}
		var out registerAgentResponse
		if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
			t.Fatalf("decode register %s failed: %v", name, err)
		}
		return out.AgentID
	}

	projectAID := register("secret-a", "project-a-agent", "proj-a")
	register("secret-b", "project-b-agent", "proj-b")

	defaultReq := httptest.NewRequest(http.MethodGet, "/api/agents/presence?active_bead_id=sylveste-kgfi.2", nil)
	defaultReq.RemoteAddr = "203.0.113.10:9999"
	defaultReq.Header.Set("Authorization", "Bearer secret-a")
	defaultResp := httptest.NewRecorder()
	h.ServeHTTP(defaultResp, defaultReq)
	if defaultResp.Code != http.StatusOK {
		t.Fatalf("presence default project expected 200, got %d", defaultResp.Code)
	}
	var defaultResult agentPresenceResponse
	if err := json.NewDecoder(defaultResp.Body).Decode(&defaultResult); err != nil {
		t.Fatalf("decode default presence failed: %v", err)
	}
	if len(defaultResult.Agents) != 1 || defaultResult.Agents[0].AgentID != projectAID {
		t.Fatalf("expected API-key request to default to proj-a agent %s, got %+v", projectAID, defaultResult.Agents)
	}

	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/agents/presence?project=proj-b&active_bead_id=sylveste-kgfi.2", nil)
	forbiddenReq.RemoteAddr = "203.0.113.10:9999"
	forbiddenReq.Header.Set("Authorization", "Bearer secret-a")
	forbiddenResp := httptest.NewRecorder()
	h.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("presence cross-project query expected 403, got %d", forbiddenResp.Code)
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
