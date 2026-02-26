package httpapi

import (
	"net/http"
	"testing"
)

func TestWindowUpsert_CreateAndLookup(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// Create a window identity
	resp := env.post(t, "/api/windows", map[string]any{
		"project":      project,
		"window_uuid":  "win-abc-123",
		"agent_id":     "agent-alpha",
		"display_name": "Alpha Agent",
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)

	if result["window_uuid"] != "win-abc-123" {
		t.Fatalf("expected window_uuid=win-abc-123, got %v", result["window_uuid"])
	}
	if result["agent_id"] != "agent-alpha" {
		t.Fatalf("expected agent_id=agent-alpha, got %v", result["agent_id"])
	}
	if result["display_name"] != "Alpha Agent" {
		t.Fatalf("expected display_name=Alpha Agent, got %v", result["display_name"])
	}
	if result["id"] == nil || result["id"].(string) == "" {
		t.Fatal("expected non-empty id")
	}

	// List and verify it appears
	listResp := env.get(t, "/api/windows?project="+project)
	requireStatus(t, listResp, http.StatusOK)
	listResult := decodeJSON[map[string]any](t, listResp)
	windows := listResult["windows"].([]any)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	win := windows[0].(map[string]any)
	if win["agent_id"] != "agent-alpha" {
		t.Fatalf("expected agent_id=agent-alpha in list, got %v", win["agent_id"])
	}
}

func TestWindowUpsert_TouchOnReuse(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// First upsert
	resp1 := env.post(t, "/api/windows", map[string]any{
		"project":      project,
		"window_uuid":  "win-reuse",
		"agent_id":     "agent-beta",
		"display_name": "Beta",
	})
	requireStatus(t, resp1, http.StatusOK)
	result1 := decodeJSON[map[string]any](t, resp1)
	lastActive1 := result1["last_active_at"].(string)

	// Second upsert with same UUID — should update last_active_at
	resp2 := env.post(t, "/api/windows", map[string]any{
		"project":      project,
		"window_uuid":  "win-reuse",
		"agent_id":     "agent-beta",
		"display_name": "Beta Updated",
	})
	requireStatus(t, resp2, http.StatusOK)
	result2 := decodeJSON[map[string]any](t, resp2)

	if result2["display_name"] != "Beta Updated" {
		t.Fatalf("expected updated display_name, got %v", result2["display_name"])
	}
	// last_active_at should be >= the first one (same second is ok)
	lastActive2 := result2["last_active_at"].(string)
	if lastActive2 < lastActive1 {
		t.Fatalf("expected last_active_at to be updated: %s >= %s", lastActive2, lastActive1)
	}

	// Only 1 entry in the list (not 2)
	listResp := env.get(t, "/api/windows?project="+project)
	requireStatus(t, listResp, http.StatusOK)
	listResult := decodeJSON[map[string]any](t, listResp)
	windows := listResult["windows"].([]any)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window after upsert, got %d", len(windows))
	}
}

func TestWindowExpire(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// Create
	resp := env.post(t, "/api/windows", map[string]any{
		"project":      project,
		"window_uuid":  "win-expire",
		"agent_id":     "agent-gamma",
		"display_name": "Gamma",
	})
	requireStatus(t, resp, http.StatusOK)

	// Expire
	delResp := env.delete(t, "/api/windows/win-expire?project="+project)
	requireStatus(t, delResp, http.StatusOK)

	// List should return 0
	listResp := env.get(t, "/api/windows?project="+project)
	requireStatus(t, listResp, http.StatusOK)
	listResult := decodeJSON[map[string]any](t, listResp)
	windows := listResult["windows"].([]any)
	if len(windows) != 0 {
		t.Fatalf("expected 0 windows after expire, got %d", len(windows))
	}
}

func TestWindowUpsert_MissingFields(t *testing.T) {
	env := newTestEnv(t)

	// Missing project
	resp := env.post(t, "/api/windows", map[string]any{
		"window_uuid": "win-x",
		"agent_id":    "agent-x",
	})
	requireStatus(t, resp, http.StatusBadRequest)

	// Missing window_uuid
	resp = env.post(t, "/api/windows", map[string]any{
		"project":  "proj",
		"agent_id": "agent-x",
	})
	requireStatus(t, resp, http.StatusBadRequest)

	// Missing agent_id
	resp = env.post(t, "/api/windows", map[string]any{
		"project":     "proj",
		"window_uuid": "win-x",
	})
	requireStatus(t, resp, http.StatusBadRequest)
}

func TestWindowList_MissingProject(t *testing.T) {
	env := newTestEnv(t)
	resp := env.get(t, "/api/windows")
	requireStatus(t, resp, http.StatusBadRequest)
}

func TestWindowExpire_MissingProject(t *testing.T) {
	env := newTestEnv(t)
	resp := env.delete(t, "/api/windows/win-x")
	requireStatus(t, resp, http.StatusBadRequest)
}

func TestWindowUpsert_DisplayNameDefaultsToAgentID(t *testing.T) {
	env := newTestEnv(t)
	resp := env.post(t, "/api/windows", map[string]any{
		"project":     "proj",
		"window_uuid": "win-default",
		"agent_id":    "agent-delta",
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)
	if result["display_name"] != "agent-delta" {
		t.Fatalf("expected display_name=agent-delta, got %v", result["display_name"])
	}
}

func TestWindowList_ProjectScoped(t *testing.T) {
	env := newTestEnv(t)

	// Create windows in different projects
	resp1 := env.post(t, "/api/windows", map[string]any{
		"project":      "alpha",
		"window_uuid":  "win-1",
		"agent_id":     "agent-a1",
		"display_name": "A1",
	})
	requireStatus(t, resp1, http.StatusOK)

	resp2 := env.post(t, "/api/windows", map[string]any{
		"project":      "beta",
		"window_uuid":  "win-2",
		"agent_id":     "agent-b1",
		"display_name": "B1",
	})
	requireStatus(t, resp2, http.StatusOK)

	// List for alpha — should only see 1
	listAlpha := env.get(t, "/api/windows?project=alpha")
	requireStatus(t, listAlpha, http.StatusOK)
	alphaResult := decodeJSON[map[string]any](t, listAlpha)
	alphaWindows := alphaResult["windows"].([]any)
	if len(alphaWindows) != 1 {
		t.Fatalf("alpha: expected 1 window, got %d", len(alphaWindows))
	}
	win := alphaWindows[0].(map[string]any)
	if win["agent_id"] != "agent-a1" {
		t.Fatalf("alpha: expected agent_id=agent-a1, got %v", win["agent_id"])
	}

	// List for beta — should only see 1
	listBeta := env.get(t, "/api/windows?project=beta")
	requireStatus(t, listBeta, http.StatusOK)
	betaResult := decodeJSON[map[string]any](t, listBeta)
	betaWindows := betaResult["windows"].([]any)
	if len(betaWindows) != 1 {
		t.Fatalf("beta: expected 1 window, got %d", len(betaWindows))
	}
}
