package httpapi

import (
	"net/http"
	"testing"
)

// registerAgent creates an agent via the API and returns its ID.
func registerAgent(t *testing.T, env *testEnv, name, project string) string {
	t.Helper()
	resp := env.post(t, "/api/agents", map[string]any{
		"name":    name,
		"project": project,
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)
	return result["agent_id"].(string)
}

func setPolicy(t *testing.T, env *testEnv, agentID, policy string) {
	t.Helper()
	resp := env.post(t, "/api/agents/"+agentID+"/policy", map[string]any{
		"policy": policy,
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func TestBroadcast_SendAndReceive(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	alice := registerAgent(t, env, "alice", project)
	bob := registerAgent(t, env, "bob", project)
	carol := registerAgent(t, env, "carol", project)

	resp := env.post(t, "/api/broadcast", map[string]any{
		"project": project,
		"from":    alice,
		"topic":   "deploy",
		"body":    "deploying v2",
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)

	delivered := int(result["delivered"].(float64))
	if delivered != 2 {
		t.Fatalf("expected delivered=2 (bob+carol), got %d", delivered)
	}
	if result["message_id"] == nil || result["message_id"].(string) == "" {
		t.Fatal("expected non-empty message_id")
	}

	// Verify bob's inbox has the broadcast
	inboxResp := env.get(t, "/api/inbox/"+bob+"?project="+project)
	requireStatus(t, inboxResp, http.StatusOK)
	inbox := decodeJSON[map[string]any](t, inboxResp)
	msgs := inbox["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("bob expected 1 message, got %d", len(msgs))
	}
	msg := msgs[0].(map[string]any)
	if msg["body"].(string) != "deploying v2" {
		t.Errorf("unexpected body: %s", msg["body"])
	}
	if msg["topic"].(string) != "deploy" {
		t.Errorf("expected topic=deploy, got %s", msg["topic"])
	}

	// Verify carol also got it
	carolResp := env.get(t, "/api/inbox/"+carol+"?project="+project)
	requireStatus(t, carolResp, http.StatusOK)
	carolInbox := decodeJSON[map[string]any](t, carolResp)
	carolMsgs := carolInbox["messages"].([]any)
	if len(carolMsgs) != 1 {
		t.Fatalf("carol expected 1 message, got %d", len(carolMsgs))
	}

	// Verify alice (sender) did NOT receive
	aliceResp := env.get(t, "/api/inbox/"+alice+"?project="+project)
	requireStatus(t, aliceResp, http.StatusOK)
	aliceInbox := decodeJSON[map[string]any](t, aliceResp)
	aliceMsgs := aliceInbox["messages"].([]any)
	if len(aliceMsgs) != 0 {
		t.Fatalf("alice (sender) should not receive broadcast, got %d", len(aliceMsgs))
	}
}

func TestBroadcast_PolicyFiltering(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	alice := registerAgent(t, env, "alice", project)
	bob := registerAgent(t, env, "bob", project)
	carol := registerAgent(t, env, "carol", project)
	_ = registerAgent(t, env, "dave", project) // open policy (default)

	// Bob blocks all messages
	setPolicy(t, env, bob, "block_all")
	// Carol only accepts contacts (alice is not in carol's contacts)
	setPolicy(t, env, carol, "contacts_only")
	// Dave is open (default)

	resp := env.post(t, "/api/broadcast", map[string]any{
		"project": project,
		"from":    alice,
		"topic":   "review",
		"body":    "please review",
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)

	delivered := int(result["delivered"].(float64))
	if delivered != 1 {
		t.Fatalf("expected delivered=1 (dave only), got %d", delivered)
	}

	denied := result["denied"].([]any)
	if len(denied) != 2 {
		t.Fatalf("expected 2 denied (bob+carol), got %d", len(denied))
	}
}

func TestBroadcast_SelfExclusion(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// Only one agent in project — sender
	alice := registerAgent(t, env, "alice", project)

	resp := env.post(t, "/api/broadcast", map[string]any{
		"project": project,
		"from":    alice,
		"topic":   "test",
		"body":    "hello?",
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)

	delivered := int(result["delivered"].(float64))
	if delivered != 0 {
		t.Fatalf("expected delivered=0 (only agent is sender), got %d", delivered)
	}
}

func TestBroadcast_TopicRequired(t *testing.T) {
	env := newTestEnv(t)

	resp := env.post(t, "/api/broadcast", map[string]any{
		"project": "proj",
		"from":    "alice",
		"topic":   "",
		"body":    "no topic",
	})
	requireStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

func TestBroadcast_MissingFields(t *testing.T) {
	env := newTestEnv(t)

	t.Run("missing from", func(t *testing.T) {
		resp := env.post(t, "/api/broadcast", map[string]any{
			"project": "proj",
			"topic":   "test",
			"body":    "hi",
		})
		requireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})

	t.Run("missing body", func(t *testing.T) {
		resp := env.post(t, "/api/broadcast", map[string]any{
			"project": "proj",
			"from":    "alice",
			"topic":   "test",
		})
		requireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})
}

func TestBroadcast_RateLimit(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	alice := registerAgent(t, env, "alice", project)
	_ = registerAgent(t, env, "bob", project)

	// Send 10 broadcasts — all should succeed
	for i := 0; i < 10; i++ {
		resp := env.post(t, "/api/broadcast", map[string]any{
			"project": project,
			"from":    alice,
			"topic":   "test",
			"body":    "msg",
		})
		requireStatus(t, resp, http.StatusOK)
		resp.Body.Close()
	}

	// 11th should be rate limited
	resp := env.post(t, "/api/broadcast", map[string]any{
		"project": project,
		"from":    alice,
		"topic":   "test",
		"body":    "one too many",
	})
	requireStatus(t, resp, http.StatusTooManyRequests)
	if resp.Header.Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
	resp.Body.Close()
}

func TestBroadcast_TopicDiscoverable(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	alice := registerAgent(t, env, "alice", project)
	_ = registerAgent(t, env, "bob", project)

	// Broadcast with a topic
	resp := env.post(t, "/api/broadcast", map[string]any{
		"project": project,
		"from":    alice,
		"topic":   "deploy",
		"body":    "deploying now",
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify the broadcast is discoverable via topic messages
	topicResp := env.get(t, "/api/topics/"+project+"/deploy")
	requireStatus(t, topicResp, http.StatusOK)
	topicResult := decodeJSON[map[string]any](t, topicResp)
	msgs := topicResult["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in deploy topic, got %d", len(msgs))
	}
	msg := msgs[0].(map[string]any)
	if msg["body"].(string) != "deploying now" {
		t.Errorf("unexpected body: %s", msg["body"])
	}
}
