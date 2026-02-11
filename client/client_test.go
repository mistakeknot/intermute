package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientSendFailsWithoutServer(t *testing.T) {
	// Start a server and immediately close it to get a guaranteed-unused port
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	c := New(deadURL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.SendMessage(ctx, Message{From: "a", To: []string{"b"}, Body: "hi"})
	if err == nil {
		t.Fatal("expected error when server unreachable")
	}
}

func TestClientAppliesBearerAndProject(t *testing.T) {
	errCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/messages" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			errCh <- "missing bearer"
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["project"] != "proj-a" {
			errCh <- "missing project"
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"message_id": "m1", "cursor": 1})
	}))
	defer srv.Close()

	c := New(srv.URL, WithAPIKey("secret"), WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.SendMessage(ctx, Message{From: "a", To: []string{"b"}, Body: "hi"}); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("handler error: %s", err)
	default:
	}
}

func TestClientListAgents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("project") != "proj-a" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agents": []map[string]any{
				{"agent_id": "a1", "name": "agent-1", "project": "proj-a"},
				{"agent_id": "a2", "name": "agent-2", "project": "proj-a"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	agents, err := c.ListAgents(ctx, "")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	if agents[0].ID != "a1" {
		t.Fatalf("expected a1, got %s", agents[0].ID)
	}
}

func TestClientListAgentsWithExplicitProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project") != "override" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"agents": []map[string]any{}})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("default"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.ListAgents(ctx, "override")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
}

func TestClientListThreads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/threads" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("agent") != "bob" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("project") != "proj-a" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"threads": []map[string]any{
				{"thread_id": "t1", "last_cursor": 10, "message_count": 3, "last_from": "alice", "last_body": "Hi"},
			},
			"cursor": 10,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := c.ListThreads(ctx, "bob", 0)
	if err != nil {
		t.Fatalf("list threads failed: %v", err)
	}
	if len(resp.Threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(resp.Threads))
	}
	if resp.Threads[0].ThreadID != "t1" {
		t.Fatalf("expected t1, got %s", resp.Threads[0].ThreadID)
	}
}

func TestClientThreadMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/threads/thread-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("project") != "proj-a" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"thread_id": "thread-1",
			"messages": []map[string]any{
				{"id": "m1", "from": "alice", "body": "Hello"},
				{"id": "m2", "from": "bob", "body": "Hi back"},
			},
			"cursor": 20,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, WithProject("proj-a"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := c.ThreadMessages(ctx, "thread-1", 0)
	if err != nil {
		t.Fatalf("thread messages failed: %v", err)
	}
	if resp.ThreadID != "thread-1" {
		t.Fatalf("expected thread-1, got %s", resp.ThreadID)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Messages))
	}
}
