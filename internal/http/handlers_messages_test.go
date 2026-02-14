package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
