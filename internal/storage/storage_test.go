package storage

import (
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestInboxSinceProjectScoped(t *testing.T) {
	st := NewInMemory()
	_, _ = st.AppendEvent(Event{Type: core.EventMessageCreated, Message: core.Message{ID: "m1", Project: "proj-a", From: "x", To: []string{"a"}}})
	_, _ = st.AppendEvent(Event{Type: core.EventMessageCreated, Message: core.Message{ID: "m2", Project: "proj-b", From: "x", To: []string{"a"}}})

	msgsA, err := st.InboxSince("proj-a", "a", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgsA) != 1 || msgsA[0].ID != "m1" {
		t.Fatalf("expected only proj-a message, got %+v", msgsA)
	}

	msgsAll, err := st.InboxSince("", "a", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgsAll) != 2 {
		t.Fatalf("expected 2 messages across projects, got %d", len(msgsAll))
	}
}
