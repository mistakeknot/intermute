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
