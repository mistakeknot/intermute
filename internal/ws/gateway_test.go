package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/auth"
	httpapi "github.com/mistakeknot/intermute/internal/http"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestWSAuthRejection(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	ring := auth.NewKeyring(true, map[string]string{"secret-a": "proj-a", "secret-b": "proj-b"})
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	router := httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(ring))

	t.Run("remote IP without bearer rejected", func(t *testing.T) {
		srv := httptest.NewServer(router)
		defer srv.Close()

		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/ws/agents/agent-a?project=proj-a", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.10")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("bearer with wrong project param rejected", func(t *testing.T) {
		srv := httptest.NewServer(router)
		defer srv.Close()

		// This test uses httptest's internal transport which sets RemoteAddr
		// to a non-localhost address, so we need to use httptest.NewRecorder
		req := httptest.NewRequest(http.MethodGet, "/ws/agents/agent-a?project=proj-b", nil)
		req.RemoteAddr = "203.0.113.10:9999"
		req.Header.Set("Authorization", "Bearer secret-a") // key for proj-a, but asking for proj-b

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for project mismatch, got %d", rr.Code)
		}
	})

	t.Run("localhost with project param accepted", func(t *testing.T) {
		srv := httptest.NewServer(router)
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agents/agent-a?project=proj-a"
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("ws dial failed (should accept localhost): %v", err)
		}
		conn.Close(websocket.StatusNormalClosure, "")
	})

	t.Run("valid bearer with matching project accepted", func(t *testing.T) {
		srv := httptest.NewServer(router)
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agents/agent-a?project=proj-a"
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader: http.Header{
				"Authorization": []string{"Bearer secret-a"},
			},
		})
		if err != nil {
			t.Fatalf("ws dial failed (valid auth): %v", err)
		}
		conn.Close(websocket.StatusNormalClosure, "")
	})
}

func TestWSReceivesMessageEvents(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agents/agent-b?project=proj-a"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	payload := map[string]any{
		"project": "proj-a",
		"from":    "a",
		"to":      []string{"agent-b"},
		"body":    "hi",
	}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/messages", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send failed: %d", resp.StatusCode)
	}

	var event map[string]any
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("read event: %v", err)
	}
	if event["type"] != "message.created" {
		t.Fatalf("expected message.created, got %v", event["type"])
	}
}
