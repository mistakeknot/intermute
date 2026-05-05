package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/storage"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
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

func TestRegisterAgentInvalidSessionIDReturns400(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// A non-UUID session_id (e.g., the literal string "unknown") should produce
	// 400 Bad Request with a structured error, not 500 Internal Server Error.
	payload := map[string]any{"name": "agent-bad", "session_id": "not-a-uuid"}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body["code"] != "invalid_session_id" {
		t.Fatalf("expected code=invalid_session_id, got %q", body["code"])
	}
	if body["error"] == "" {
		t.Fatalf("expected non-empty error message")
	}
}

func TestDomainRouterExposesAgentPresence(t *testing.T) {
	env := newTestEnv(t)

	resp := env.get(t, "/api/agents/presence")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from domain router presence endpoint, got %d", resp.StatusCode)
	}

	var result struct {
		Agents []any `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode presence response failed: %v", err)
	}
	if len(result.Agents) != 0 {
		t.Fatalf("expected no agents in empty store, got %+v", result.Agents)
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

func TestPresenceAgentsFiltersByRepoAndActiveBeadID(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	type presenceAgent struct {
		AgentID      string   `json:"agent_id"`
		Kind         string   `json:"kind"`
		Status       string   `json:"status"`
		LastSeen     string   `json:"last_seen"`
		Repo         string   `json:"repo"`
		Files        []string `json:"files"`
		Objective    string   `json:"objective"`
		Confidence   string   `json:"confidence"`
		ActiveBeadID string   `json:"active_bead_id"`
		ThreadID     string   `json:"thread_id"`
	}
	type presenceResponse struct {
		Agents []presenceAgent `json:"agents"`
	}

	repo := "/home/mk/projects/Sylveste/core/intermute"
	beadID := "sylveste-kgfi.2"

	register := func(t *testing.T, payload map[string]any) string {
		t.Helper()
		buf, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("register expected 200, got %d", resp.StatusCode)
		}
		var out registerAgentResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode register failed: %v", err)
		}
		return out.AgentID
	}

	matchingID := register(t, map[string]any{
		"name":    "claude-code",
		"project": "sylveste",
		"status":  "active",
		"metadata": map[string]string{
			"agent_kind":             "claude-code",
			"repo":                   repo,
			"active_bead_id":         beadID,
			"thread_id":              beadID,
			"active_bead_confidence": "reported",
			"files_touched":          `["internal/http/handlers_agents.go","internal/http/handlers_agents_test.go"]`,
			"objective":              "Add bead presence read model",
		},
	})
	register(t, map[string]any{
		"name":    "same-repo-different-bead",
		"project": "sylveste",
		"status":  "active",
		"metadata": map[string]string{
			"agent_kind":             "codex",
			"repo":                   repo,
			"active_bead_id":         "sylveste-kgfi.3",
			"active_bead_confidence": "reported",
		},
	})
	register(t, map[string]any{
		"name":    "same-bead-different-repo",
		"project": "sylveste",
		"status":  "active",
		"metadata": map[string]string{
			"agent_kind":             "claude-code",
			"repo":                   "/home/mk/projects/Sylveste/interverse/intermux",
			"active_bead_id":         beadID,
			"active_bead_confidence": "reported",
		},
	})
	register(t, map[string]any{
		"name":    "ambiguous-candidate-only",
		"project": "sylveste",
		"status":  "active",
		"metadata": map[string]string{
			"agent_kind":             "claude-code",
			"repo":                   repo,
			"active_bead_id":         "",
			"active_bead_candidates": `["sylveste-kgfi.2","sylveste-kgfi.4"]`,
			"active_bead_confidence": "unknown",
		},
	})

	query := url.Values{}
	query.Set("repo", repo)
	query.Set("active_bead_id", beadID)
	resp, err := http.Get(srv.URL + "/api/agents/presence?" + query.Encode())
	if err != nil {
		t.Fatalf("presence request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result presenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 matching presence agent, got %d: %+v", len(result.Agents), result.Agents)
	}
	agent := result.Agents[0]
	if agent.AgentID != matchingID {
		t.Fatalf("expected matching agent %s, got %s", matchingID, agent.AgentID)
	}
	if agent.Kind != "claude-code" || agent.Status != "active" || agent.Repo != repo {
		t.Fatalf("unexpected compact presence fields: %+v", agent)
	}
	if agent.LastSeen == "" {
		t.Fatalf("expected last_seen to be populated")
	}
	if agent.ActiveBeadID != beadID || agent.ThreadID != beadID || agent.Confidence != "reported" {
		t.Fatalf("unexpected bead correlation fields: %+v", agent)
	}
	if agent.Objective != "Add bead presence read model" {
		t.Fatalf("unexpected objective %q", agent.Objective)
	}
	if len(agent.Files) != 2 || agent.Files[0] != "internal/http/handlers_agents.go" || agent.Files[1] != "internal/http/handlers_agents_test.go" {
		t.Fatalf("unexpected files: %#v", agent.Files)
	}
}

func TestPresenceAgentsProjectsProducerMetadataStatus(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	repo := "/home/mk/projects/Sylveste/core/intermute"
	beadID := "sylveste-kgfi.2"
	payload := map[string]any{
		"name":    "metadata-status-agent",
		"project": "sylveste",
		"metadata": map[string]string{
			"agent_kind":     "codex",
			"repo":           repo,
			"active_bead_id": beadID,
			"status":         "idle",
		},
	}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register expected 200, got %d", resp.StatusCode)
	}

	query := url.Values{}
	query.Set("active_bead_id", beadID)
	presenceResp, err := http.Get(srv.URL + "/api/agents/presence?" + query.Encode())
	if err != nil {
		t.Fatalf("presence request failed: %v", err)
	}
	defer presenceResp.Body.Close()
	if presenceResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", presenceResp.StatusCode)
	}

	var result struct {
		Agents []struct {
			AgentID    string `json:"agent_id"`
			Status     string `json:"status"`
			Repo       string `json:"repo"`
			Confidence string `json:"confidence"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(presenceResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 matching presence agent, got %d: %+v", len(result.Agents), result.Agents)
	}
	if result.Agents[0].Status != "idle" {
		t.Fatalf("expected producer metadata status idle, got %+v", result.Agents[0])
	}
	if result.Agents[0].Repo != repo {
		t.Fatalf("expected repo %q, got %+v", repo, result.Agents[0])
	}
	if result.Agents[0].Confidence != "unknown" {
		t.Fatalf("expected default confidence unknown, got %+v", result.Agents[0])
	}
}

func TestPresenceAgentsUnauthenticatedProjectFilter(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	repo := "/home/mk/projects/Sylveste/core/intermute"
	beadID := "sylveste-kgfi.2"
	register := func(t *testing.T, name, project string) string {
		t.Helper()
		buf, _ := json.Marshal(map[string]any{
			"name":    name,
			"project": project,
			"metadata": map[string]string{
				"agent_kind":     "codex",
				"repo":           repo,
				"active_bead_id": beadID,
			},
		})
		resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("register expected 200, got %d", resp.StatusCode)
		}
		var out registerAgentResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode register failed: %v", err)
		}
		return out.AgentID
	}

	projectAID := register(t, "project-a-agent", "sylveste")
	register(t, "project-b-agent", "athenverse")

	query := url.Values{}
	query.Set("project", "sylveste")
	query.Set("active_bead_id", beadID)
	resp, err := http.Get(srv.URL + "/api/agents/presence?" + query.Encode())
	if err != nil {
		t.Fatalf("presence project-filter request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result agentPresenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 1 || result.Agents[0].AgentID != projectAID {
		t.Fatalf("expected only sylveste presence agent %s, got %+v", projectAID, result.Agents)
	}
}

func TestPresenceAgentsNormalizesProjectedMetadata(t *testing.T) {
	svc := NewService(storage.NewInMemory())
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	repo := "/home/mk/projects/Sylveste/core/intermute"
	beadID := "sylveste-kgfi.2"
	payload := map[string]any{
		"name":    "whitespace-agent",
		"project": "sylveste",
		"status":  " active ",
		"metadata": map[string]string{
			"agent_kind":             "  claude-code  ",
			"repo":                   "  " + repo + "  ",
			"active_bead_id":         "  " + beadID + "  ",
			"thread_id":              "  " + beadID + "  ",
			"active_bead_confidence": "  reported  ",
			"files_touched":          `[" internal/http/handlers_agents.go ",""," internal/http/handlers_agents_test.go "]`,
			"objective":              "  Add bead presence read model  ",
			"status":                 "  stuck  ",
		},
	}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register expected 200, got %d", resp.StatusCode)
	}

	query := url.Values{}
	query.Set("repo", repo)
	query.Set("active_bead_id", beadID)
	presenceResp, err := http.Get(srv.URL + "/api/agents/presence?" + query.Encode())
	if err != nil {
		t.Fatalf("presence request failed: %v", err)
	}
	defer presenceResp.Body.Close()
	if presenceResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", presenceResp.StatusCode)
	}
	var result agentPresenceResponse
	if err := json.NewDecoder(presenceResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected one normalized presence agent, got %+v", result.Agents)
	}
	agent := result.Agents[0]
	if agent.Kind != "claude-code" || agent.Status != "stuck" || agent.Repo != repo {
		t.Fatalf("expected normalized kind/status/repo, got %+v", agent)
	}
	if agent.Objective != "Add bead presence read model" || agent.ActiveBeadID != beadID || agent.ThreadID != beadID || agent.Confidence != "reported" {
		t.Fatalf("expected normalized bead metadata fields, got %+v", agent)
	}
	if len(agent.Files) != 2 || agent.Files[0] != "internal/http/handlers_agents.go" || agent.Files[1] != "internal/http/handlers_agents_test.go" {
		t.Fatalf("expected normalized nonblank files, got %#v", agent.Files)
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

func TestHeartbeatAcceptsFocusState(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	payload := map[string]any{"name": "agent-focus", "project": "proj-a"}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	var reg registerAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	resp.Body.Close()

	hbBody, _ := json.Marshal(map[string]any{"focus_state": core.FocusStateAtPrompt})
	hbResp, err := http.Post(srv.URL+"/api/agents/"+reg.AgentID+"/heartbeat", "application/json", bytes.NewReader(hbBody))
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	defer hbResp.Body.Close()
	if hbResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", hbResp.StatusCode)
	}

	state, _, err := st.GetAgentFocusState(context.Background(), reg.AgentID)
	if err != nil {
		t.Fatalf("GetAgentFocusState: %v", err)
	}
	if state != core.FocusStateAtPrompt {
		t.Fatalf("focus_state = %q, want %q", state, core.FocusStateAtPrompt)
	}
}

func TestHeartbeatRejectsInvalidFocusState(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	payload := map[string]any{"name": "agent-focus", "project": "proj-a"}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	var reg registerAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	resp.Body.Close()

	hbBody, _ := json.Marshal(map[string]any{"focus_state": "bogus"})
	hbResp, err := http.Post(srv.URL+"/api/agents/"+reg.AgentID+"/heartbeat", "application/json", bytes.NewReader(hbBody))
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	defer hbResp.Body.Close()
	if hbResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", hbResp.StatusCode)
	}
}

func TestPolicyEndpointAcceptsLiveContactPolicy(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	payload := map[string]any{"name": "agent-policy", "project": "proj-a"}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/agents", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	var reg registerAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	resp.Body.Close()

	reqBody, _ := json.Marshal(map[string]any{"live_contact_policy": string(core.PolicyBlockAll)})
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/agents/"+reg.AgentID+"/policy", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("policy update failed: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", putResp.StatusCode)
	}

	var result getPolicyResponse
	if err := json.NewDecoder(putResp.Body).Decode(&result); err != nil {
		t.Fatalf("decode policy response: %v", err)
	}
	if result.LiveContactPolicy != string(core.PolicyBlockAll) {
		t.Fatalf("response live_contact_policy = %q, want %q", result.LiveContactPolicy, core.PolicyBlockAll)
	}

	livePolicy, err := st.GetLiveContactPolicy(context.Background(), reg.AgentID)
	if err != nil {
		t.Fatalf("GetLiveContactPolicy: %v", err)
	}
	if livePolicy != core.PolicyBlockAll {
		t.Fatalf("live_contact_policy = %q, want %q", livePolicy, core.PolicyBlockAll)
	}
}
