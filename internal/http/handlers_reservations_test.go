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
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create reservation expected 201, got %d", createResp.Code)
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

// newReservationTestEnv creates a test env using NewRouter (which includes
// reservation endpoints). NewDomainRouter does not register reservation routes.
type reservationTestEnv struct {
	srv   *httptest.Server
	store *sqlite.Store
}

func newReservationTestEnv(t *testing.T) *reservationTestEnv {
	t.Helper()
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	t.Cleanup(srv.Close)
	return &reservationTestEnv{srv: srv, store: st}
}

func (e *reservationTestEnv) post(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	buf, _ := json.Marshal(body)
	resp, err := http.Post(e.srv.URL+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (e *reservationTestEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(e.srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func TestReservationCreateAndList(t *testing.T) {
	env := newReservationTestEnv(t)
	const project = "proj-test"

	// Create a reservation
	resp := env.post(t, "/api/reservations", map[string]any{
		"agent_id":     "agent-a",
		"project":      project,
		"path_pattern": "src/*.go",
		"exclusive":    true,
		"reason":       "refactoring",
		"ttl_minutes":  10,
	})
	requireStatus(t, resp, http.StatusCreated)
	res := decodeJSON[map[string]any](t, resp)
	if res["id"] == nil || res["id"] == "" {
		t.Fatal("expected reservation id")
	}
	if res["is_active"] != true {
		t.Fatalf("expected is_active=true, got %v", res["is_active"])
	}

	// List active by project
	listResp := env.get(t, "/api/reservations?project="+project)
	requireStatus(t, listResp, http.StatusOK)
	listData := decodeJSON[map[string]any](t, listResp)
	reservations := listData["reservations"].([]any)
	if len(reservations) != 1 {
		t.Fatalf("expected 1 active reservation, got %d", len(reservations))
	}

	// List by agent
	agentResp := env.get(t, "/api/reservations?agent=agent-a")
	requireStatus(t, agentResp, http.StatusOK)
	agentData := decodeJSON[map[string]any](t, agentResp)
	agentRes := agentData["reservations"].([]any)
	if len(agentRes) != 1 {
		t.Fatalf("expected 1 reservation for agent-a, got %d", len(agentRes))
	}
}

func TestReservationOverlapConflict(t *testing.T) {
	env := newReservationTestEnv(t)
	const project = "proj-test"

	// Create first exclusive reservation
	resp1 := env.post(t, "/api/reservations", map[string]any{
		"agent_id":     "agent-a",
		"project":      project,
		"path_pattern": "internal/http/*.go",
		"exclusive":    true,
	})
	requireStatus(t, resp1, http.StatusCreated)
	resp1.Body.Close()

	// Second exclusive overlapping reservation should fail with 409 Conflict
	resp2 := env.post(t, "/api/reservations", map[string]any{
		"agent_id":     "agent-b",
		"project":      project,
		"path_pattern": "internal/http/router.go",
		"exclusive":    true,
	})
	requireStatus(t, resp2, http.StatusConflict)
	conflictBody := decodeJSON[map[string]any](t, resp2)
	if conflictBody["error"] != "reservation_conflict" {
		t.Fatalf("expected error=reservation_conflict, got %v", conflictBody["error"])
	}
	conflicts := conflictBody["conflicts"].([]any)
	if len(conflicts) == 0 {
		t.Fatal("expected at least one conflict detail")
	}
	detail := conflicts[0].(map[string]any)
	if detail["pattern"] == nil || detail["held_by"] == nil {
		t.Fatal("conflict detail missing pattern or held_by")
	}
}

func TestReservationSharedAllowed(t *testing.T) {
	env := newReservationTestEnv(t)
	const project = "proj-test"

	// Two shared overlapping reservations should both succeed
	resp1 := env.post(t, "/api/reservations", map[string]any{
		"agent_id":     "agent-a",
		"project":      project,
		"path_pattern": "docs/*.md",
		"exclusive":    false,
	})
	requireStatus(t, resp1, http.StatusCreated)
	resp1.Body.Close()

	resp2 := env.post(t, "/api/reservations", map[string]any{
		"agent_id":     "agent-b",
		"project":      project,
		"path_pattern": "docs/README.md",
		"exclusive":    false,
	})
	requireStatus(t, resp2, http.StatusCreated)
	resp2.Body.Close()

	// Verify both are active
	listResp := env.get(t, "/api/reservations?project="+project)
	requireStatus(t, listResp, http.StatusOK)
	listData := decodeJSON[map[string]any](t, listResp)
	reservations := listData["reservations"].([]any)
	if len(reservations) != 2 {
		t.Fatalf("expected 2 shared reservations, got %d", len(reservations))
	}
}

func TestReservationReleaseAndVerify(t *testing.T) {
	env := newReservationTestEnv(t)
	const project = "proj-test"

	// Create reservation
	resp := env.post(t, "/api/reservations", map[string]any{
		"agent_id":     "agent-a",
		"project":      project,
		"path_pattern": "src/*.go",
		"exclusive":    true,
	})
	requireStatus(t, resp, http.StatusCreated)
	res := decodeJSON[map[string]any](t, resp)
	resID := res["id"].(string)

	// Release via store (the HTTP DELETE endpoint requires auth agent matching)
	if err := env.store.ReleaseReservation(nil, resID, "agent-a"); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Verify no longer active
	listResp := env.get(t, "/api/reservations?project="+project)
	requireStatus(t, listResp, http.StatusOK)
	listData := decodeJSON[map[string]any](t, listResp)
	reservations := listData["reservations"].([]any)
	if len(reservations) != 0 {
		t.Fatalf("expected 0 active reservations after release, got %d", len(reservations))
	}
}

func TestReservationListRequiresProjectOrAgent(t *testing.T) {
	env := newReservationTestEnv(t)

	// No project or agent param → 400
	resp := env.get(t, "/api/reservations")
	requireStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}
