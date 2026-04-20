package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/livetransport"
	"github.com/mistakeknot/intermute/internal/storage/sqlite"
)

func TestInboxPokesListAndAck(t *testing.T) {
	st, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	svc := NewDomainService(st)

	_, err = st.RegisterAgent(context.Background(), core.Agent{
		Name:    "bob",
		Project: "p1",
		Status:  "active",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	_, err = st.AppendEvent(context.Background(), core.Event{
		Type:    core.EventPeerWindowPoke,
		Project: "p1",
		Message: core.Message{
			ID:        "m1",
			Project:   "p1",
			From:      "alice",
			To:        []string{"bob"},
			Body:      "rebase",
			CreatedAt: time.Now().UTC(),
			Metadata: map[string]string{
				"poke_result": core.PokeResultDeferred,
			},
		},
	})
	if err != nil {
		t.Fatalf("append poke: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/inbox/pokes?agent=bob&project=p1", nil)
	rr := httptest.NewRecorder()
	svc.handleInboxPokes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeJSONFromRecorder[InboxPokesResponse](t, rr)
	if len(got.Pokes) != 1 {
		t.Fatalf("expected 1 poke, got %d", len(got.Pokes))
	}
	if got.Pokes[0].MessageID != "m1" {
		t.Fatalf("expected message_id m1, got %q", got.Pokes[0].MessageID)
	}
	if got.Pokes[0].Body != livetransport.WrapEnvelope("alice", "", "rebase") {
		t.Fatalf("unexpected wrapped body: %q", got.Pokes[0].Body)
	}

	ackReq := httptest.NewRequest(http.MethodPost, "/api/inbox/pokes/m1/ack?agent=bob&project=p1", nil)
	ackRR := httptest.NewRecorder()
	svc.handleInboxPokeAction(ackRR, ackReq)
	if ackRR.Code != http.StatusOK {
		t.Fatalf("expected ack 200, got %d", ackRR.Code)
	}
	_ = decodeJSONFromRecorder[inboxPokeAckResponse](t, ackRR)

	req = httptest.NewRequest(http.MethodGet, "/api/inbox/pokes?agent=bob&project=p1", nil)
	rr = httptest.NewRecorder()
	svc.handleInboxPokes(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 after ack, got %d", rr.Code)
	}
	got = decodeJSONFromRecorder[InboxPokesResponse](t, rr)
	if len(got.Pokes) != 0 {
		t.Fatalf("expected 0 pokes after ack, got %d", len(got.Pokes))
	}
}

func decodeJSONFromRecorder[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rr.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}
