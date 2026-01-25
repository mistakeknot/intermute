package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientSendAndInbox(t *testing.T) {
	c := New("http://localhost:7338")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.SendMessage(ctx, Message{From: "a", To: []string{"b"}, Body: "hi"})
	if err == nil {
		t.Fatalf("expected failure without server")
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
