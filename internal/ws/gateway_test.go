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

	httpapi "github.com/mistakeknot/intermute/internal/http"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestWSReceivesMessageEvents(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	hub := NewHub()
	svc := httpapi.NewService(st).WithBroadcaster(hub)
	srv := httptest.NewServer(httpapi.NewRouter(svc, hub.Handler()))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agents/agent-b"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	payload := map[string]any{
		"from": "a",
		"to":   []string{"agent-b"},
		"body": "hi",
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
