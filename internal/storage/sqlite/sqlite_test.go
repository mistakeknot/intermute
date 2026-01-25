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

func TestSQLiteListAgents(t *testing.T) {
	st := NewSQLiteTest(t)

	// Register agents in different projects
	_, err := st.RegisterAgent(core.Agent{Name: "agent-a", Project: "proj-a", Status: "active"})
	if err != nil {
		t.Fatalf("register agent-a: %v", err)
	}
	_, err = st.RegisterAgent(core.Agent{Name: "agent-b", Project: "proj-b", Status: "idle"})
	if err != nil {
		t.Fatalf("register agent-b: %v", err)
	}

	// List all agents
	all, err := st.ListAgents("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(all))
	}

	// List by project
	projA, err := st.ListAgents("proj-a")
	if err != nil {
		t.Fatalf("list proj-a: %v", err)
	}
	if len(projA) != 1 {
		t.Fatalf("expected 1 agent in proj-a, got %d", len(projA))
	}
	if projA[0].Name != "agent-a" {
		t.Fatalf("expected agent-a, got %s", projA[0].Name)
	}
}

func TestSQLiteListAgentsOrderByLastSeen(t *testing.T) {
	st := NewSQLiteTest(t)

	// Register two agents
	a1, _ := st.RegisterAgent(core.Agent{Name: "agent-first", Project: "proj"})
	_, _ = st.RegisterAgent(core.Agent{Name: "agent-second", Project: "proj"})

	// Heartbeat the first agent to make it more recent
	_, _ = st.Heartbeat(a1.ID)

	agents, err := st.ListAgents("proj")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	// First agent should be first (most recent heartbeat)
	if agents[0].Name != "agent-first" {
		t.Fatalf("expected agent-first first (most recent), got %s", agents[0].Name)
	}
}
