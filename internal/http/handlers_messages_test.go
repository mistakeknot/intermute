package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/livetransport"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
)

func TestSendMessageAndFetchInbox(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	payload := map[string]any{
		"from": "a",
		"to":   []string{"b"},
		"body": "hi",
	}
	buf, _ := json.Marshal(payload)
	send, err := http.Post(srv.URL+"/api/messages", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if send.StatusCode != http.StatusOK {
		t.Fatalf("send failed: %d", send.StatusCode)
	}

	inbox, err := http.Get(srv.URL + "/api/inbox/b")
	if err != nil {
		t.Fatalf("inbox failed: %v", err)
	}
	if inbox.StatusCode != http.StatusOK {
		t.Fatalf("inbox failed: %d", inbox.StatusCode)
	}
}

// sendTestMessage is a helper that sends a message and returns the message_id.
func sendTestMessage(t *testing.T, env *testEnv, project, from string, to []string, body string) string {
	t.Helper()
	resp := env.post(t, "/api/messages", map[string]any{
		"project": project,
		"from":    from,
		"to":      to,
		"body":    body,
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)
	return result["message_id"].(string)
}

func TestMessageReadAction(t *testing.T) {
	env := newTestEnv(t)
	msgID := sendTestMessage(t, env, "proj", "alice", []string{"bob"}, "hello")

	resp := env.post(t, "/api/messages/"+msgID+"/read?project=proj", map[string]any{
		"agent": "bob",
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func TestMessageAckAction(t *testing.T) {
	env := newTestEnv(t)
	msgID := sendTestMessage(t, env, "proj", "alice", []string{"bob"}, "please ack")

	resp := env.post(t, "/api/messages/"+msgID+"/ack?project=proj", map[string]any{
		"agent": "bob",
	})
	requireStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func TestMessageActionInvalidAction(t *testing.T) {
	env := newTestEnv(t)
	msgID := sendTestMessage(t, env, "proj", "alice", []string{"bob"}, "test")

	resp := env.post(t, "/api/messages/"+msgID+"/invalid", map[string]any{
		"agent": "bob",
	})
	requireStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

func TestMessageActionMethodNotAllowed(t *testing.T) {
	env := newTestEnv(t)

	// GET on a message action path should be 405
	resp := env.get(t, "/api/messages/some-id/read")
	requireStatus(t, resp, http.StatusMethodNotAllowed)
	resp.Body.Close()
}

func TestInboxCounts(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// Send 3 messages to bob
	for i := 0; i < 3; i++ {
		sendTestMessage(t, env, project, "alice", []string{"bob"}, fmt.Sprintf("msg-%d", i))
	}

	// Check initial counts: 3 total, 3 unread
	resp := env.get(t, "/api/inbox/bob/counts?project="+project)
	requireStatus(t, resp, http.StatusOK)
	counts := decodeJSON[map[string]any](t, resp)
	if int(counts["total"].(float64)) != 3 {
		t.Fatalf("expected total=3, got %v", counts["total"])
	}
	if int(counts["unread"].(float64)) != 3 {
		t.Fatalf("expected unread=3, got %v", counts["unread"])
	}

	// Mark first message as read by fetching inbox to get message IDs
	inboxResp := env.get(t, "/api/inbox/bob?project="+project)
	requireStatus(t, inboxResp, http.StatusOK)
	inbox := decodeJSON[map[string]any](t, inboxResp)
	messages := inbox["messages"].([]any)
	firstMsgID := messages[0].(map[string]any)["id"].(string)

	readResp := env.post(t, "/api/messages/"+firstMsgID+"/read?project="+project, map[string]any{
		"agent": "bob",
	})
	requireStatus(t, readResp, http.StatusOK)
	readResp.Body.Close()

	// Check counts again: 3 total, 2 unread
	resp2 := env.get(t, "/api/inbox/bob/counts?project="+project)
	requireStatus(t, resp2, http.StatusOK)
	counts2 := decodeJSON[map[string]any](t, resp2)
	if int(counts2["total"].(float64)) != 3 {
		t.Fatalf("expected total=3, got %v", counts2["total"])
	}
	if int(counts2["unread"].(float64)) != 2 {
		t.Fatalf("expected unread=2, got %v", counts2["unread"])
	}
}

func TestInboxCountsEmptyAgent(t *testing.T) {
	env := newTestEnv(t)

	// /api/inbox/ with no agent should return 400
	resp := env.get(t, "/api/inbox/")
	requireStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()
}

func TestInboxSincePagination(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// Send 5 messages
	for i := 0; i < 5; i++ {
		sendTestMessage(t, env, project, "alice", []string{"bob"}, fmt.Sprintf("msg-%d", i))
	}

	// Fetch with limit=2
	resp := env.get(t, "/api/inbox/bob?project="+project+"&limit=2")
	requireStatus(t, resp, http.StatusOK)
	page1 := decodeJSON[map[string]any](t, resp)
	msgs1 := page1["messages"].([]any)
	if len(msgs1) != 2 {
		t.Fatalf("expected 2 messages in page 1, got %d", len(msgs1))
	}
	cursor := page1["cursor"].(float64)

	// Fetch next page using cursor
	resp2 := env.get(t, fmt.Sprintf("/api/inbox/bob?project=%s&limit=2&since_cursor=%d", project, int(cursor)))
	requireStatus(t, resp2, http.StatusOK)
	page2 := decodeJSON[map[string]any](t, resp2)
	msgs2 := page2["messages"].([]any)
	if len(msgs2) != 2 {
		t.Fatalf("expected 2 messages in page 2, got %d", len(msgs2))
	}

	// Fetch last page
	cursor2 := page2["cursor"].(float64)
	resp3 := env.get(t, fmt.Sprintf("/api/inbox/bob?project=%s&limit=2&since_cursor=%d", project, int(cursor2)))
	requireStatus(t, resp3, http.StatusOK)
	page3 := decodeJSON[map[string]any](t, resp3)
	msgs3 := page3["messages"].([]any)
	if len(msgs3) != 1 {
		t.Fatalf("expected 1 message in page 3, got %d", len(msgs3))
	}
}

func TestSendMessageValidation(t *testing.T) {
	env := newTestEnv(t)

	t.Run("missing from", func(t *testing.T) {
		resp := env.post(t, "/api/messages", map[string]any{
			"to":   []string{"bob"},
			"body": "hi",
		})
		requireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})

	t.Run("missing to", func(t *testing.T) {
		resp := env.post(t, "/api/messages", map[string]any{
			"from": "alice",
			"body": "hi",
		})
		requireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})

	t.Run("empty to", func(t *testing.T) {
		resp := env.post(t, "/api/messages", map[string]any{
			"from": "alice",
			"to":   []string{},
			"body": "hi",
		})
		requireStatus(t, resp, http.StatusBadRequest)
		resp.Body.Close()
	})
}

func TestStaleAcksEndpoint(t *testing.T) {
	env := newTestEnv(t)
	const project = "proj"

	// Send a message with ack_required
	resp := env.post(t, "/api/messages", map[string]any{
		"project":      project,
		"from":         "alice",
		"to":           []string{"bob"},
		"body":         "please ack",
		"subject":      "urgent",
		"ack_required": true,
	})
	requireStatus(t, resp, http.StatusOK)
	result := decodeJSON[map[string]any](t, resp)
	msgID := result["message_id"].(string)

	// Use ttl_seconds=0 so the just-sent message is immediately stale
	staleResp := env.get(t, fmt.Sprintf("/api/inbox/bob/stale-acks?project=%s&ttl_seconds=0", project))
	requireStatus(t, staleResp, http.StatusOK)
	var staleResult staleAcksResponse
	if err := json.NewDecoder(staleResp.Body).Decode(&staleResult); err != nil {
		t.Fatalf("decode: %v", err)
	}
	staleResp.Body.Close()

	if staleResult.Count != 1 {
		t.Fatalf("expected 1 stale ack, got %d", staleResult.Count)
	}
	if staleResult.TTLSeconds != 0 {
		t.Fatalf("expected ttl_seconds=0, got %d", staleResult.TTLSeconds)
	}
	if staleResult.Messages[0].ID != msgID {
		t.Fatalf("expected message %s, got %s", msgID, staleResult.Messages[0].ID)
	}
	if staleResult.Messages[0].Subject != "urgent" {
		t.Fatalf("expected subject=urgent, got %s", staleResult.Messages[0].Subject)
	}
}

func TestStaleAcksEndpoint_EmptyResult(t *testing.T) {
	env := newTestEnv(t)

	// No messages at all — should return empty list
	resp := env.get(t, "/api/inbox/nobody/stale-acks?project=proj&ttl_seconds=1800")
	requireStatus(t, resp, http.StatusOK)
	var result staleAcksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	if result.Count != 0 {
		t.Fatalf("expected 0, got %d", result.Count)
	}
	if len(result.Messages) != 0 {
		t.Fatalf("expected empty messages, got %d", len(result.Messages))
	}
}

type fakeLiveDelivery struct {
	mu    sync.Mutex
	calls []livetransport.Target
	fail  bool
}

func (f *fakeLiveDelivery) Deliver(target *livetransport.Target, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if target != nil {
		f.calls = append(f.calls, *target)
	}
	if f.fail {
		return fmt.Errorf("inject failed")
	}
	return nil
}

func (f *fakeLiveDelivery) ValidateTarget(_ *livetransport.Target) error {
	return nil
}

func newTransportTestService(t *testing.T) (*Service, *fakeLiveDelivery) {
	t.Helper()
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	fake := &fakeLiveDelivery{}
	return NewService(st).WithLiveDelivery(fake), fake
}

func registerTransportAgent(t *testing.T, svc *Service, project, agentID string) {
	t.Helper()
	_, err := svc.store.RegisterAgent(context.Background(), core.Agent{
		ID:      agentID,
		Name:    agentID,
		Project: project,
		Token:   agentID + "-token",
	})
	if err != nil {
		t.Fatalf("RegisterAgent(%s): %v", agentID, err)
	}
}

func setTransportFocusState(t *testing.T, svc *Service, agentID, state string) {
	t.Helper()
	if err := svc.store.SetAgentFocusState(context.Background(), agentID, state); err != nil {
		t.Fatalf("SetAgentFocusState(%s): %v", agentID, err)
	}
}

func setTransportLivePolicy(t *testing.T, svc *Service, agentID string, policy core.ContactPolicy) {
	t.Helper()
	if err := svc.store.SetLiveContactPolicy(context.Background(), agentID, policy); err != nil {
		t.Fatalf("SetLiveContactPolicy(%s): %v", agentID, err)
	}
}

func setTransportWindow(t *testing.T, svc *Service, project, agentID, windowUUID, tmuxTarget string) {
	t.Helper()
	_, err := svc.store.UpsertWindowIdentity(context.Background(), core.WindowIdentity{
		Project:     project,
		WindowUUID:  windowUUID,
		AgentID:     agentID,
		DisplayName: agentID,
		TmuxTarget:  tmuxTarget,
	})
	if err != nil {
		t.Fatalf("UpsertWindowIdentity(%s): %v", agentID, err)
	}
}

func sendTransportRequest(t *testing.T, svc *Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(body))
	rr := httptest.NewRecorder()
	svc.handleSendMessage(rr, req)
	return rr
}

func TestSendTransportLiveRecipientBusy(t *testing.T) {
	svc, fake := newTransportTestService(t)
	registerTransportAgent(t, svc, "p1", "bob")
	setTransportLivePolicy(t, svc, "bob", core.PolicyOpen)
	setTransportFocusState(t, svc, "bob", core.FocusStateToolUse)

	rr := sendTransportRequest(t, svc, `{"project":"p1","from":"alice","to":["bob"],"body":"rebase please","transport":"live"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "recipient_busy") {
		t.Fatalf("want recipient_busy body, got %s", rr.Body.String())
	}
	if len(fake.calls) != 0 {
		t.Fatalf("expected no live delivery calls, got %d", len(fake.calls))
	}
}

func TestSendTransportBothInjectsAndPersists(t *testing.T) {
	svc, fake := newTransportTestService(t)
	registerTransportAgent(t, svc, "p1", "bob")
	setTransportLivePolicy(t, svc, "bob", core.PolicyOpen)
	setTransportFocusState(t, svc, "bob", core.FocusStateAtPrompt)
	setTransportWindow(t, svc, "p1", "bob", "w-bob", "sylveste:0.0")

	rr := sendTransportRequest(t, svc, `{"project":"p1","from":"alice","to":["bob"],"body":"rebase please","transport":"both"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"delivery":"injected"`) {
		t.Fatalf("want injected delivery, got %s", rr.Body.String())
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 live delivery call, got %d", len(fake.calls))
	}

	msgs, err := svc.store.InboxSince(context.Background(), "p1", "bob", 0, 10)
	if err != nil {
		t.Fatalf("InboxSince: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(msgs))
	}
	if msgs[0].Transport != core.TransportBoth {
		t.Fatalf("expected transport=both, got %q", msgs[0].Transport)
	}

	pending, err := svc.store.ListPendingPokes(context.Background(), "p1", "bob")
	if err != nil {
		t.Fatalf("ListPendingPokes: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending pokes after inject, got %d", len(pending))
	}
}

func TestSendTransportBothBusyDefersAtomically(t *testing.T) {
	svc, fake := newTransportTestService(t)
	registerTransportAgent(t, svc, "p1", "bob")
	setTransportLivePolicy(t, svc, "bob", core.PolicyOpen)
	setTransportFocusState(t, svc, "bob", core.FocusStateThinking)

	rr := sendTransportRequest(t, svc, `{"project":"p1","from":"alice","to":["bob"],"body":"rebase please","transport":"both"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"delivery":"deferred"`) {
		t.Fatalf("want deferred delivery, got %s", rr.Body.String())
	}
	if len(fake.calls) != 0 {
		t.Fatalf("expected no live delivery calls, got %d", len(fake.calls))
	}

	msgs, err := svc.store.InboxSince(context.Background(), "p1", "bob", 0, 10)
	if err != nil {
		t.Fatalf("InboxSince: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 inbox message, got %d", len(msgs))
	}

	pending, err := svc.store.ListPendingPokes(context.Background(), "p1", "bob")
	if err != nil {
		t.Fatalf("ListPendingPokes: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending poke, got %d", len(pending))
	}
	if pending[0].MessageID != msgs[0].ID {
		t.Fatalf("pending poke message mismatch: got %s want %s", pending[0].MessageID, msgs[0].ID)
	}
}

func TestSendTransportLivePolicyDenied(t *testing.T) {
	svc, fake := newTransportTestService(t)
	registerTransportAgent(t, svc, "p1", "bob")
	setTransportLivePolicy(t, svc, "bob", core.PolicyBlockAll)
	setTransportFocusState(t, svc, "bob", core.FocusStateAtPrompt)
	setTransportWindow(t, svc, "p1", "bob", "w-bob", "sylveste:0.0")

	rr := sendTransportRequest(t, svc, `{"project":"p1","from":"alice","to":["bob"],"body":"x","transport":"live"}`)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(fake.calls) != 0 {
		t.Fatalf("expected no live delivery calls, got %d", len(fake.calls))
	}
}

func TestSendTransportLiveInjectFailureDoesNotDefer(t *testing.T) {
	svc, fake := newTransportTestService(t)
	fake.fail = true
	registerTransportAgent(t, svc, "p1", "bob")
	setTransportLivePolicy(t, svc, "bob", core.PolicyOpen)
	setTransportFocusState(t, svc, "bob", core.FocusStateAtPrompt)
	setTransportWindow(t, svc, "p1", "bob", "w-bob", "sylveste:0.0")

	rr := sendTransportRequest(t, svc, `{"project":"p1","from":"alice","to":["bob"],"body":"x","transport":"live"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d: %s", rr.Code, rr.Body.String())
	}

	msgs, err := svc.store.InboxSince(context.Background(), "p1", "bob", 0, 10)
	if err != nil {
		t.Fatalf("InboxSince: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no durable inbox message for live-only failure, got %d", len(msgs))
	}

	pending, err := svc.store.ListPendingPokes(context.Background(), "p1", "bob")
	if err != nil {
		t.Fatalf("ListPendingPokes: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no deferred pending poke for live-only failure, got %d", len(pending))
	}
}

func TestSendTransportLiveRateLimit(t *testing.T) {
	svc, _ := newTransportTestService(t)
	registerTransportAgent(t, svc, "p1", "bob")
	setTransportLivePolicy(t, svc, "bob", core.PolicyOpen)
	setTransportFocusState(t, svc, "bob", core.FocusStateAtPrompt)
	setTransportWindow(t, svc, "p1", "bob", "w-bob", "sylveste:0.0")

	body := `{"project":"p1","from":"alice","to":["bob"],"body":"x","transport":"live"}`
	for i := 0; i < liveRateLimit; i++ {
		rr := sendTransportRequest(t, svc, body)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d want 200, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	rr := sendTransportRequest(t, svc, body)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "retry_after_seconds") {
		t.Fatalf("expected retry_after_seconds body, got %s", rr.Body.String())
	}
}
