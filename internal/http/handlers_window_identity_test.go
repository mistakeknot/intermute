package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestWindowUpsert_CreateAndLookup(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-alpha",
		Project: project,
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	// Create a window identity
	resp := env.post(t, "/api/windows", map[string]any{
		"project":            project,
		"window_uuid":        "win-abc-123",
		"agent_id":           agent.ID,
		"display_name":       "Alpha Agent",
		"registration_token": agent.Token,
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)

	if result["window_uuid"] != "win-abc-123" {
		t.Fatalf("expected window_uuid=win-abc-123, got %v", result["window_uuid"])
	}
	if result["agent_id"] != agent.ID {
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
	if win["agent_id"] != agent.ID {
		t.Fatalf("expected agent_id=%s in list, got %v", agent.ID, win["agent_id"])
	}
}

func TestWindowUpsert_TouchOnReuse(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-beta",
		Project: project,
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	// First upsert
	resp1 := env.post(t, "/api/windows", map[string]any{
		"project":            project,
		"window_uuid":        "win-reuse",
		"agent_id":           agent.ID,
		"display_name":       "Beta",
		"registration_token": agent.Token,
	})
	requireStatus(t, resp1, http.StatusOK)
	result1 := decodeJSON[map[string]any](t, resp1)
	lastActive1 := result1["last_active_at"].(string)

	// Second upsert with same UUID — should update last_active_at
	resp2 := env.post(t, "/api/windows", map[string]any{
		"project":            project,
		"window_uuid":        "win-reuse",
		"agent_id":           agent.ID,
		"display_name":       "Beta Updated",
		"registration_token": agent.Token,
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
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-gamma",
		Project: project,
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	// Create
	resp := env.post(t, "/api/windows", map[string]any{
		"project":            project,
		"window_uuid":        "win-expire",
		"agent_id":           agent.ID,
		"display_name":       "Gamma",
		"registration_token": agent.Token,
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
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-x",
		Project: "proj",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	// Missing project
	resp := env.post(t, "/api/windows", map[string]any{
		"window_uuid":        "win-x",
		"agent_id":           agent.ID,
		"registration_token": agent.Token,
	})
	requireStatus(t, resp, http.StatusBadRequest)

	// Missing window_uuid
	resp = env.post(t, "/api/windows", map[string]any{
		"project":            "proj",
		"agent_id":           agent.ID,
		"registration_token": agent.Token,
	})
	requireStatus(t, resp, http.StatusBadRequest)

	// Missing agent_id
	resp = env.post(t, "/api/windows", map[string]any{
		"project":            "proj",
		"window_uuid":        "win-x",
		"registration_token": agent.Token,
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
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-delta",
		Project: "proj",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	resp := env.post(t, "/api/windows", map[string]any{
		"project":            "proj",
		"window_uuid":        "win-default",
		"agent_id":           agent.ID,
		"registration_token": agent.Token,
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)
	if result["display_name"] != agent.ID {
		t.Fatalf("expected display_name=%s, got %v", agent.ID, result["display_name"])
	}
}

func TestWindowList_ProjectScoped(t *testing.T) {
	env := newTestEnv(t)
	agentA, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-a1",
		Project: "alpha",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent-a1: %v", err)
	}
	agentB, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-b1",
		Project: "beta",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent-b1: %v", err)
	}

	// Create windows in different projects
	resp1 := env.post(t, "/api/windows", map[string]any{
		"project":            "alpha",
		"window_uuid":        "win-1",
		"agent_id":           agentA.ID,
		"display_name":       "A1",
		"registration_token": agentA.Token,
	})
	requireStatus(t, resp1, http.StatusOK)

	resp2 := env.post(t, "/api/windows", map[string]any{
		"project":            "beta",
		"window_uuid":        "win-2",
		"agent_id":           agentB.ID,
		"display_name":       "B1",
		"registration_token": agentB.Token,
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
	if win["agent_id"] != agentA.ID {
		t.Fatalf("alpha: expected agent_id=%s, got %v", agentA.ID, win["agent_id"])
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

func TestWindowUpsert_TmuxTargetAccepted(t *testing.T) {
	env := newTestEnv(t)
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-tmux",
		Project: "proj",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	resp := env.post(t, "/api/windows", map[string]any{
		"project":            "proj",
		"window_uuid":        "win-tmux",
		"agent_id":           agent.ID,
		"display_name":       "Tmux Agent",
		"tmux_target":        "sylveste:0.0",
		"registration_token": agent.Token,
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)
	if result["tmux_target"] != "sylveste:0.0" {
		t.Fatalf("expected tmux_target=sylveste:0.0, got %v", result["tmux_target"])
	}
}

func TestWindowUpsert_RejectsInvalidTmuxTarget(t *testing.T) {
	env := newTestEnv(t)
	agent, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-invalid",
		Project: "proj",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	resp := env.post(t, "/api/windows", map[string]any{
		"project":            "proj",
		"window_uuid":        "win-invalid",
		"agent_id":           agent.ID,
		"tmux_target":        "bad;target",
		"registration_token": agent.Token,
	})
	requireStatus(t, resp, http.StatusBadRequest)
	result := decodeJSON[map[string]string](t, resp)
	if result["error"] != "invalid tmux_target" {
		t.Fatalf("expected invalid tmux_target error, got %#v", result)
	}
}

func TestWindowUpsert_RejectsRegistrationTokenMismatch(t *testing.T) {
	env := newTestEnv(t)
	agentA, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-a",
		Project: "proj",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent-a: %v", err)
	}
	agentB, err := env.store.RegisterAgent(context.Background(), core.Agent{
		Name:    "agent-b",
		Project: "proj",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent-b: %v", err)
	}

	resp := env.post(t, "/api/windows", map[string]any{
		"project":            "proj",
		"window_uuid":        "win-mismatch",
		"agent_id":           agentA.ID,
		"tmux_target":        "sylveste:1.0",
		"registration_token": agentB.Token,
	})
	requireStatus(t, resp, http.StatusForbidden)
	result := decodeJSON[map[string]string](t, resp)
	if result["error"] != "agent_token_mismatch" {
		t.Fatalf("expected agent_token_mismatch, got %#v", result)
	}
}
