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

func TestPatchAgentMetadata(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Register an agent with initial metadata
	payload := map[string]any{
		"name":     "agent-meta",
		"project":  "proj-a",
		"metadata": map[string]string{"key1": "val1", "key2": "val2"},
	}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	var reg registerAgentResponse
	json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()

	// PATCH metadata: overwrite key1, add key3, preserve key2
	patchPayload := map[string]any{
		"metadata": map[string]string{"key1": "updated", "key3": "new"},
	}
	patchBuf, _ := json.Marshal(patchPayload)
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/agents/"+reg.AgentID+"/metadata", bytes.NewReader(patchBuf))
	req.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch failed: %v", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", patchResp.StatusCode)
	}

	var result agentJSON
	if err := json.NewDecoder(patchResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Verify merge semantics
	if result.Metadata["key1"] != "updated" {
		t.Errorf("key1: expected 'updated', got %q", result.Metadata["key1"])
	}
	if result.Metadata["key2"] != "val2" {
		t.Errorf("key2: expected 'val2' (preserved), got %q", result.Metadata["key2"])
	}
	if result.Metadata["key3"] != "new" {
		t.Errorf("key3: expected 'new', got %q", result.Metadata["key3"])
	}
}

func TestPatchAgentMetadataNotFound(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	patchPayload := map[string]any{
		"metadata": map[string]string{"key": "val"},
	}
	buf, _ := json.Marshal(patchPayload)
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/agents/nonexistent/metadata", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
