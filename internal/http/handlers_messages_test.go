package httpapi

import (
	"bytes"
	"encoding/json"
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
