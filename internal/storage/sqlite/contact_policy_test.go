package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

func registerTestAgent(t *testing.T, s *Store, id, project string) {
	t.Helper()
	ctx := context.Background()
	_, err := s.RegisterAgent(ctx, core.Agent{
		ID:        id,
		Name:      id,
		Project:   project,
		CreatedAt: time.Now().UTC(),
		LastSeen:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("register agent %s: %v", id, err)
	}
}

func TestContactPolicy_SetAndGet(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()
	registerTestAgent(t, s, "agent-a", "proj")

	// Default is open
	policy, err := s.GetContactPolicy(ctx, "agent-a")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if policy != core.PolicyOpen {
		t.Errorf("expected open, got %s", policy)
	}

	// Set to block_all
	if err := s.SetContactPolicy(ctx, "agent-a", core.PolicyBlockAll); err != nil {
		t.Fatalf("set policy: %v", err)
	}
	policy, err = s.GetContactPolicy(ctx, "agent-a")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if policy != core.PolicyBlockAll {
		t.Errorf("expected block_all, got %s", policy)
	}

	// Set to auto
	if err := s.SetContactPolicy(ctx, "agent-a", core.PolicyAuto); err != nil {
		t.Fatalf("set policy: %v", err)
	}
	policy, err = s.GetContactPolicy(ctx, "agent-a")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if policy != core.PolicyAuto {
		t.Errorf("expected auto, got %s", policy)
	}
}

func TestContactPolicy_GetNonexistentAgent(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()

	// Nonexistent agent returns open (default, not error)
	policy, err := s.GetContactPolicy(ctx, "nobody")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if policy != core.PolicyOpen {
		t.Errorf("expected open for nonexistent, got %s", policy)
	}
}

func TestContactList_AddRemoveList(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()
	registerTestAgent(t, s, "agent-a", "proj")
	registerTestAgent(t, s, "agent-b", "proj")

	// No contacts initially
	contacts, err := s.ListContacts(ctx, "agent-a")
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if len(contacts) != 0 {
		t.Errorf("expected 0 contacts, got %d", len(contacts))
	}

	// Add contact
	if err := s.AddContact(ctx, "agent-a", "agent-b"); err != nil {
		t.Fatalf("add contact: %v", err)
	}
	ok, err := s.IsContact(ctx, "agent-a", "agent-b")
	if err != nil {
		t.Fatalf("is contact: %v", err)
	}
	if !ok {
		t.Error("expected agent-b to be a contact of agent-a")
	}

	// Not symmetric
	ok, err = s.IsContact(ctx, "agent-b", "agent-a")
	if err != nil {
		t.Fatalf("is contact reverse: %v", err)
	}
	if ok {
		t.Error("contacts should not be symmetric")
	}

	// Add duplicate — idempotent
	if err := s.AddContact(ctx, "agent-a", "agent-b"); err != nil {
		t.Fatalf("add duplicate: %v", err)
	}

	contacts, err = s.ListContacts(ctx, "agent-a")
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	if len(contacts) != 1 || contacts[0] != "agent-b" {
		t.Errorf("expected [agent-b], got %v", contacts)
	}

	// Remove
	if err := s.RemoveContact(ctx, "agent-a", "agent-b"); err != nil {
		t.Fatalf("remove contact: %v", err)
	}
	ok, err = s.IsContact(ctx, "agent-a", "agent-b")
	if err != nil {
		t.Fatalf("is contact after remove: %v", err)
	}
	if ok {
		t.Error("agent-b should not be a contact after removal")
	}
}

func TestReservationOverlap(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()
	registerTestAgent(t, s, "agent-a", "proj")
	registerTestAgent(t, s, "agent-b", "proj")

	// No reservations — no overlap
	overlap, err := s.HasReservationOverlap(ctx, "proj", "agent-a", "agent-b")
	if err != nil {
		t.Fatalf("overlap check: %v", err)
	}
	if overlap {
		t.Error("expected no overlap with no reservations")
	}

	// Add overlapping reservations
	now := time.Now().UTC()
	_, err = s.Reserve(ctx, core.Reservation{
		ID: "r1", AgentID: "agent-a", Project: "proj",
		PathPattern: "pkg/*.go", Exclusive: false,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("reserve a: %v", err)
	}
	_, err = s.Reserve(ctx, core.Reservation{
		ID: "r2", AgentID: "agent-b", Project: "proj",
		PathPattern: "pkg/*.go", Exclusive: false,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("reserve b: %v", err)
	}

	// Same directory patterns overlap
	overlap, err = s.HasReservationOverlap(ctx, "proj", "agent-a", "agent-b")
	if err != nil {
		t.Fatalf("overlap check: %v", err)
	}
	if !overlap {
		t.Error("expected overlap between pkg/*.go and pkg/*.go")
	}

	// Different segment counts don't overlap (glob * doesn't cross /)
	if err := s.ReleaseReservation(ctx, "r2", "agent-b"); err != nil {
		t.Fatalf("release: %v", err)
	}
	_, err = s.Reserve(ctx, core.Reservation{
		ID: "r3", AgentID: "agent-b", Project: "proj",
		PathPattern: "pkg/events/*.go", Exclusive: true,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("reserve b non-overlapping: %v", err)
	}
	overlap, err = s.HasReservationOverlap(ctx, "proj", "agent-a", "agent-b")
	if err != nil {
		t.Fatalf("non-overlap check: %v", err)
	}
	if overlap {
		t.Error("pkg/*.go and pkg/events/*.go should NOT overlap (different segment count)")
	}
}

func TestThreadParticipant(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()

	// Create a message with thread
	_, err := s.AppendEvent(ctx, core.Event{
		Type:    core.EventMessageCreated,
		Project: "proj",
		Message: core.Message{
			ID:       "msg-1",
			ThreadID: "thread-1",
			From:     "alice",
			To:       []string{"bob"},
			Body:     "hello",
		},
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	// Alice and Bob are thread participants
	ok, err := s.IsThreadParticipant(ctx, "proj", "thread-1", "alice")
	if err != nil {
		t.Fatalf("is participant alice: %v", err)
	}
	if !ok {
		t.Error("alice should be a thread participant")
	}

	ok, err = s.IsThreadParticipant(ctx, "proj", "thread-1", "bob")
	if err != nil {
		t.Fatalf("is participant bob: %v", err)
	}
	if !ok {
		t.Error("bob should be a thread participant")
	}

	// Charlie is not
	ok, err = s.IsThreadParticipant(ctx, "proj", "thread-1", "charlie")
	if err != nil {
		t.Fatalf("is participant charlie: %v", err)
	}
	if ok {
		t.Error("charlie should not be a thread participant")
	}
}
