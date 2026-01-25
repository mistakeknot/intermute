package sqlite

import (
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestSQLiteInboxSinceCursor(t *testing.T) {
	st := NewSQLiteTest(t)
	c1, _ := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Agent: "a", Message: core.Message{ID: "m1", Project: "proj-a", From: "x", To: []string{"a"}, Body: "hi"}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Agent: "a", Message: core.Message{ID: "m2", Project: "proj-a", From: "x", To: []string{"a"}, Body: "hi2"}})
	msgs, err := st.InboxSince("proj-a", "a", c1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after cursor=%d, got %d", c1, len(msgs))
	}
	if msgs[0].ID != "m2" {
		t.Fatalf("expected m2, got %s", msgs[0].ID)
	}
}

func TestSQLiteProjectIsolation(t *testing.T) {
	st := NewSQLiteTest(t)
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{ID: "m1", Project: "proj-a", From: "x", To: []string{"a"}}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{ID: "m2", Project: "proj-b", From: "x", To: []string{"a"}}})

	msgsA, err := st.InboxSince("proj-a", "a", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgsA) != 1 || msgsA[0].Project != "proj-a" {
		t.Fatalf("expected only proj-a messages, got %+v", msgsA)
	}
}
