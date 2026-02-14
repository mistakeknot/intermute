package ws

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

// dialWS connects a WebSocket client to the given server.
func dialWS(t *testing.T, srv *httptest.Server, agent, project string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agents/" + agent + "?project=" + project
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial %s/%s: %v", agent, project, err)
	}
	return conn
}

// readWSEvent reads a single JSON event from a WS connection with a timeout.
func readWSEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var event map[string]any
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatalf("read event: %v", err)
	}
	return event
}

// sendMsg sends a message via HTTP and returns the response.
func sendMsg(t *testing.T, srvURL, project, from string, to []string, body string) {
	t.Helper()
	payload := map[string]any{"project": project, "from": from, "to": to, "body": body}
	buf, _ := json.Marshal(payload)
	resp, err := http.Post(srvURL+"/api/messages", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("send msg: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send msg status: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWSMultiSubscriberFanout(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	// Two agents subscribe in the same project
	conn1 := dialWS(t, srv, "agent-a", "proj-x")
	defer conn1.Close(websocket.StatusNormalClosure, "")
	conn2 := dialWS(t, srv, "agent-b", "proj-x")
	defer conn2.Close(websocket.StatusNormalClosure, "")

	// Send a message to both agents
	sendMsg(t, srv.URL, "proj-x", "sender", []string{"agent-a", "agent-b"}, "fanout test")

	// Both should receive the event (each targeted individually by the handler)
	ev1 := readWSEvent(t, conn1, 2*time.Second)
	if ev1["type"] != "message.created" {
		t.Fatalf("agent-a expected message.created, got %v", ev1["type"])
	}
	ev2 := readWSEvent(t, conn2, 2*time.Second)
	if ev2["type"] != "message.created" {
		t.Fatalf("agent-b expected message.created, got %v", ev2["type"])
	}
}

func TestWSProjectIsolation(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	// Agent in proj-a
	connA := dialWS(t, srv, "agent-a", "proj-a")
	defer connA.Close(websocket.StatusNormalClosure, "")

	// Agent in proj-b
	connB := dialWS(t, srv, "agent-b", "proj-b")
	defer connB.Close(websocket.StatusNormalClosure, "")

	// Send message in proj-a only
	sendMsg(t, srv.URL, "proj-a", "sender", []string{"agent-a"}, "proj-a only")

	// Agent-a should receive it
	ev := readWSEvent(t, connA, 2*time.Second)
	if ev["type"] != "message.created" {
		t.Fatalf("expected message.created, got %v", ev["type"])
	}

	// Agent-b (proj-b) should NOT receive it — reading should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var noop map[string]any
	err = wsjson.Read(ctx, connB, &noop)
	if err == nil {
		t.Fatal("agent-b in proj-b should NOT have received a proj-a event")
	}
}

func TestWSSubscriptionCleanup(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	// Connect and immediately close
	conn := dialWS(t, srv, "agent-temp", "proj-x")
	conn.Close(websocket.StatusNormalClosure, "done")

	// Give the server a moment to process the close
	time.Sleep(50 * time.Millisecond)

	// Sending a message after client disconnect should not panic
	sendMsg(t, srv.URL, "proj-x", "sender", []string{"agent-temp"}, "after close")
}

func TestWSAgentTargetedDelivery(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	// Both agents in same project
	connA := dialWS(t, srv, "agent-a", "proj-x")
	defer connA.Close(websocket.StatusNormalClosure, "")
	connB := dialWS(t, srv, "agent-b", "proj-x")
	defer connB.Close(websocket.StatusNormalClosure, "")

	// Message only to agent-b
	sendMsg(t, srv.URL, "proj-x", "sender", []string{"agent-b"}, "b only")

	// Agent-b should receive it
	ev := readWSEvent(t, connB, 2*time.Second)
	if ev["type"] != "message.created" {
		t.Fatalf("expected message.created, got %v", ev["type"])
	}

	// Agent-a should NOT receive it
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var noop map[string]any
	err = wsjson.Read(ctx, connA, &noop)
	if err == nil {
		t.Fatal("agent-a should NOT have received a message targeted to agent-b")
	}
}

func TestWSConcurrentBroadcast(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler(), auth.Middleware(nil)))
	defer srv.Close()

	const numSubscribers = 10
	const numMessages = 5

	conns := make([]*websocket.Conn, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		agent := fmt.Sprintf("agent-%d", i)
		conns[i] = dialWS(t, srv, agent, "proj-x")
		defer conns[i].Close(websocket.StatusNormalClosure, "")
	}

	// Send messages to all agents
	allAgents := make([]string, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		allAgents[i] = fmt.Sprintf("agent-%d", i)
	}
	for i := 0; i < numMessages; i++ {
		sendMsg(t, srv.URL, "proj-x", "sender", allAgents, fmt.Sprintf("broadcast-%d", i))
	}

	// Each subscriber should receive all messages
	var wg sync.WaitGroup
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var event map[string]any
				err := wsjson.Read(ctx, conns[idx], &event)
				cancel()
				if err != nil {
					t.Errorf("subscriber %d failed to read message %d: %v", idx, j, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
