package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/mistakeknot/intermute/internal/storage/sqlite"
)

func TestListThreadsAndGetMessages(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Send messages in a thread
	for _, msg := range []map[string]any{
		{"from": "alice", "to": []string{"bob"}, "thread_id": "thread-1", "body": "Hello"},
		{"from": "bob", "to": []string{"alice"}, "thread_id": "thread-1", "body": "Hi back"},
	} {
		buf, _ := json.Marshal(msg)
		resp, err := http.Post(srv.URL+"/api/messages", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("send failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("send failed: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	// List threads for bob
	resp, err := http.Get(srv.URL + "/api/threads?agent=bob&project=")
	if err != nil {
		t.Fatalf("list threads failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list threads failed: %d", resp.StatusCode)
	}

	var listResp listThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	resp.Body.Close()

	if len(listResp.Threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(listResp.Threads))
	}
	if listResp.Threads[0].ThreadID != "thread-1" {
		t.Fatalf("expected thread-1, got %s", listResp.Threads[0].ThreadID)
	}
	if listResp.Threads[0].MessageCount != 2 {
		t.Fatalf("expected 2 messages, got %d", listResp.Threads[0].MessageCount)
	}

	// Get thread messages
	resp, err = http.Get(srv.URL + "/api/threads/thread-1?project=")
	if err != nil {
		t.Fatalf("get thread messages failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get thread messages failed: %d", resp.StatusCode)
	}

	var threadResp threadMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&threadResp); err != nil {
		t.Fatalf("decode thread response: %v", err)
	}
	resp.Body.Close()

	if threadResp.ThreadID != "thread-1" {
		t.Fatalf("expected thread-1, got %s", threadResp.ThreadID)
	}
	if len(threadResp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(threadResp.Messages))
	}
	if threadResp.Messages[0].From != "alice" {
		t.Fatalf("expected first message from alice, got %s", threadResp.Messages[0].From)
	}
}

func TestListThreadsRequiresAgent(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/threads")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestThreadMessagesRequiresThreadID(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/threads/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListThreadsMethodNotAllowed(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/threads?agent=bob", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestListThreadsPaginationCursor(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewService(st)
	srv := httptest.NewServer(NewRouter(svc, nil, nil))
	defer srv.Close()

	// Create five single-message threads.
	for i := 1; i <= 5; i++ {
		payload := map[string]any{
			"from":      "alice",
			"to":        []string{"bob"},
			"thread_id": "thread-" + strconv.Itoa(i),
			"body":      "Message",
		}
		buf, _ := json.Marshal(payload)
		resp, err := http.Post(srv.URL+"/api/messages", "application/json", bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("send failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("send failed: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	q := url.Values{}
	q.Set("agent", "bob")
	q.Set("limit", "2")

	resp, err := http.Get(srv.URL + "/api/threads?" + q.Encode())
	if err != nil {
		t.Fatalf("list threads failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list threads failed: %d", resp.StatusCode)
	}
	var first listThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	resp.Body.Close()
	if len(first.Threads) != 2 {
		t.Fatalf("expected 2 threads on first page, got %d", len(first.Threads))
	}

	q.Set("cursor", strconv.FormatUint(first.Cursor, 10))
	resp, err = http.Get(srv.URL + "/api/threads?" + q.Encode())
	if err != nil {
		t.Fatalf("list threads second page failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list threads second page failed: %d", resp.StatusCode)
	}
	var second listThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	resp.Body.Close()
	if len(second.Threads) != 2 {
		t.Fatalf("expected 2 threads on second page, got %d", len(second.Threads))
	}
	if second.Threads[0].LastCursor >= first.Threads[len(first.Threads)-1].LastCursor {
		t.Fatalf("expected older threads on second page, got cursor %d after %d",
			second.Threads[0].LastCursor, first.Threads[len(first.Threads)-1].LastCursor)
	}
}
