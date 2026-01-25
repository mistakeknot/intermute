package sqlite

import (
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestSQLiteInboxSinceCursor(t *testing.T) {
	st := NewSQLiteTest(t)
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Agent: "a", Message: core.Message{ID: "m1", From: "x", To: []string{"a"}, Body: "hi"}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Agent: "a", Message: core.Message{ID: "m2", From: "x", To: []string{"a"}, Body: "hi2"}})
	msgs, err := st.InboxSince("a", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after cursor=1, got %d", len(msgs))
	}
	if msgs[0].ID != "m2" {
		t.Fatalf("expected m2, got %s", msgs[0].ID)
	}
}
