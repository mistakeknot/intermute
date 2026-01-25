package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mistakeknot/intermute/internal/storage"
)

func TestRegisterAgent(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	payload := map[string]any{"name": "agent-a"}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListAgents(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Register two agents
	for _, name := range []string{"agent-a", "agent-b"} {
		payload := map[string]any{"name": name, "project": "proj-a"}
		buf, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		resp.Body.Close()
	}

	// List agents
	resp, err := http.Get(srv.URL + "/api/agents?project=proj-a")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result listAgentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(result.Agents))
	}
}

func TestListAgentsProjectFilter(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Register agents in different projects
	for _, tc := range []struct{ name, project string }{
		{"agent-a", "proj-a"},
		{"agent-b", "proj-b"},
	} {
		payload := map[string]any{"name": tc.name, "project": tc.project}
		buf, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		resp.Body.Close()
	}

	// List agents for proj-a only
	resp, err := http.Get(srv.URL + "/api/agents?project=proj-a")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer resp.Body.Close()

	var result listAgentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result.Agents))
	}
	if result.Agents[0].Project != "proj-a" {
		t.Fatalf("expected proj-a, got %s", result.Agents[0].Project)
	}
}
