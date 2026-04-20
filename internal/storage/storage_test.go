package storage

import (
	"context"
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestInboxSinceProjectScoped(t *testing.T) {
	ctx := context.Background()
	st := NewInMemory()
	_, _ = st.AppendEvent(ctx, Event{Type: core.EventMessageCreated, Message: core.Message{ID: "m1", Project: "proj-a", From: "x", To: []string{"a"}}})
	_, _ = st.AppendEvent(ctx, Event{Type: core.EventMessageCreated, Message: core.Message{ID: "m2", Project: "proj-b", From: "x", To: []string{"a"}}})

	msgsA, err := st.InboxSince(ctx, "proj-a", "a", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgsA) != 1 || msgsA[0].ID != "m1" {
		t.Fatalf("expected only proj-a message, got %+v", msgsA)
	}

	msgsAll, err := st.InboxSince(ctx, "", "a", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgsAll) != 2 {
		t.Fatalf("expected 2 messages across projects, got %d", len(msgsAll))
	}
}

func TestInMemorySatisfiesStoreInterface(t *testing.T) {
	var _ Store = (*InMemory)(nil)

	im := NewInMemory()
	ctx := context.Background()
	if err := im.SetAgentFocusState(ctx, "a", "at-prompt"); err != nil {
		t.Errorf("SetAgentFocusState: %v", err)
	}
	if fs, _, err := im.GetAgentFocusState(ctx, "a"); err != nil || fs == "" {
		t.Errorf("GetAgentFocusState: fs=%q err=%v", fs, err)
	}
	if pp, err := im.ListPendingPokes(ctx, "p", "a"); err != nil || pp == nil {
		t.Errorf("ListPendingPokes: got nil slice or err=%v", err)
	}
	if cursors, err := im.AppendEvents(ctx); err != nil || cursors == nil {
		t.Errorf("AppendEvents: cursors=%v err=%v", cursors, err)
	}
	if ok, err := im.LiveTransportEnabled(ctx); err != nil || !ok {
		t.Errorf("LiveTransportEnabled: ok=%v err=%v", ok, err)
	}
}
