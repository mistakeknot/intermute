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

func TestListAgentsCapabilityFilter(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Register agents with capabilities — includes one with empty caps
	for _, tc := range []struct {
		name string
		caps []string
	}{
		{"agent-arch", []string{"review:architecture", "review:code"}},
		{"agent-safety", []string{"review:safety", "review:security"}},
		{"agent-both", []string{"review:architecture", "review:safety"}},
		{"agent-nocaps", []string{}},
	} {
		payload := map[string]any{"name": tc.name, "project": "proj-a", "capabilities": tc.caps}
		buf, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		resp.Body.Close()
	}

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"single match", "?project=proj-a&capability=review:architecture", 2},
		{"multi OR match", "?project=proj-a&capability=review:architecture,review:security", 3},
		{"no match", "?project=proj-a&capability=research:docs", 0},
		{"no filter returns all", "?project=proj-a", 4},
		{"trailing comma ignored", "?project=proj-a&capability=review:architecture,", 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/api/agents" + tc.query)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			var result listAgentsResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if len(result.Agents) != tc.expected {
				t.Fatalf("expected %d agents, got %d", tc.expected, len(result.Agents))
			}
		})
	}
}

func TestCapabilityDiscoveryEndToEnd(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Simulate registration with capabilities (as interlock-register.sh would)
	agents := []struct {
		name string
		caps []string
	}{
		{"fd-architecture", []string{"review:architecture", "review:code"}},
		{"fd-safety", []string{"review:safety", "review:security"}},
		{"repo-research-analyst", []string{"research:codebase", "research:architecture"}},
		{"agent-nocaps", nil},
	}

	for _, a := range agents {
		payload := map[string]any{
			"name":         a.name,
			"project":      "demarch",
			"capabilities": a.caps,
		}
		buf, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("register %s failed: %v", a.name, err)
		}
		resp.Body.Close()
	}

	// Query by single capability — only fd-architecture has review:architecture
	// (repo-research-analyst has research:architecture — different domain prefix)
	resp, err := http.Get(srv.URL + "/api/agents?project=demarch&capability=review:architecture")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result listAgentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent for review:architecture, got %d", len(result.Agents))
	}
	if result.Agents[0].Name != "fd-architecture" {
		t.Fatalf("expected fd-architecture, got %s", result.Agents[0].Name)
	}

	// Query by OR across domains
	resp2, err := http.Get(srv.URL + "/api/agents?project=demarch&capability=review:safety,research:codebase")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var result2 listAgentsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result2.Agents) != 2 {
		t.Fatalf("expected 2 agents for safety+codebase, got %d", len(result2.Agents))
	}

	// Verify capabilities are returned in the response
	for _, a := range result2.Agents {
		if len(a.Capabilities) == 0 {
			t.Errorf("agent %s has no capabilities in response", a.Name)
		}
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
