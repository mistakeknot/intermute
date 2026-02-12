package sqlite

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

func TestSQLiteInboxSinceCursor(t *testing.T) {
	st := NewSQLiteTest(t)
	c1, _ := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Agent: "a", Message: core.Message{ID: "m1", Project: "proj-a", From: "x", To: []string{"a"}, Body: "hi"}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Agent: "a", Message: core.Message{ID: "m2", Project: "proj-a", From: "x", To: []string{"a"}, Body: "hi2"}})
	msgs, err := st.InboxSince("proj-a", "a", c1, 0)
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

	msgsA, err := st.InboxSince("proj-a", "a", 0, 0)
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
	_, _ = st.Heartbeat("proj", a1.ID)

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

func TestSQLiteThreadMessages(t *testing.T) {
	st := NewSQLiteTest(t)

	// Create messages in a thread
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m1", ThreadID: "thread-1", Project: "proj", From: "alice", To: []string{"bob"}, Body: "Hello",
	}})
	c2, _ := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m2", ThreadID: "thread-1", Project: "proj", From: "bob", To: []string{"alice"}, Body: "Hi back",
	}})
	// Message in different thread
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m3", ThreadID: "thread-2", Project: "proj", From: "alice", To: []string{"bob"}, Body: "Other thread",
	}})

	// Get all messages in thread-1
	msgs, err := st.ThreadMessages("proj", "thread-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != "m1" || msgs[1].ID != "m2" {
		t.Fatalf("wrong message order: %s, %s", msgs[0].ID, msgs[1].ID)
	}

	// Get messages after cursor
	msgs, err = st.ThreadMessages("proj", "thread-1", c2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after cursor, got %d", len(msgs))
	}
}

func TestSQLiteListThreads(t *testing.T) {
	st := NewSQLiteTest(t)

	// Create messages in threads
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m1", ThreadID: "thread-1", Project: "proj", From: "alice", To: []string{"bob"}, Body: "Hello",
	}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m2", ThreadID: "thread-1", Project: "proj", From: "bob", To: []string{"alice"}, Body: "Hi back",
	}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m3", ThreadID: "thread-2", Project: "proj", From: "alice", To: []string{"bob"}, Body: "Another thread",
	}})

	// List threads for bob
	threads, err := st.ListThreads("proj", "bob", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	// Most recent first
	if threads[0].ThreadID != "thread-2" {
		t.Fatalf("expected thread-2 first (most recent), got %s", threads[0].ThreadID)
	}
	if threads[1].MessageCount != 2 {
		t.Fatalf("expected 2 messages in thread-1, got %d", threads[1].MessageCount)
	}
}

func TestSQLiteThreadProjectIsolation(t *testing.T) {
	st := NewSQLiteTest(t)

	// Create threads in different projects
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m1", ThreadID: "thread-1", Project: "proj-a", From: "alice", To: []string{"bob"}, Body: "Proj A",
	}})
	_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
		ID: "m2", ThreadID: "thread-1", Project: "proj-b", From: "alice", To: []string{"bob"}, Body: "Proj B",
	}})

	// List threads should be isolated by project
	threadsA, err := st.ListThreads("proj-a", "bob", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(threadsA) != 1 {
		t.Fatalf("expected 1 thread in proj-a, got %d", len(threadsA))
	}

	threadsB, err := st.ListThreads("proj-b", "bob", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(threadsB) != 1 {
		t.Fatalf("expected 1 thread in proj-b, got %d", len(threadsB))
	}
}

func TestSQLiteThreadPagination(t *testing.T) {
	st := NewSQLiteTest(t)

	// Create multiple threads
	var lastCursor uint64
	for i := 1; i <= 5; i++ {
		c, _ := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
			ID: "m" + string(rune('0'+i)), ThreadID: "thread-" + string(rune('0'+i)), Project: "proj", From: "alice", To: []string{"bob"}, Body: "Message",
		}})
		if i == 3 {
			lastCursor = c
		}
	}

	// Get threads with limit
	threads, err := st.ListThreads("proj", "bob", 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads with limit, got %d", len(threads))
	}

	// Get threads after cursor
	threads, err = st.ListThreads("proj", "bob", lastCursor, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads after cursor %d, got %d", lastCursor, len(threads))
	}
}

func TestSQLiteThreadPaginationCursorFromPage(t *testing.T) {
	st := NewSQLiteTest(t)

	// Create multiple threads with increasing cursors.
	for i := 1; i <= 5; i++ {
		_, _ = st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
			ID: "m" + string(rune('0'+i)), ThreadID: "thread-" + string(rune('0'+i)), Project: "proj", From: "alice", To: []string{"bob"}, Body: "Message",
		}})
	}

	firstPage, err := st.ListThreads("proj", "bob", 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firstPage) != 2 {
		t.Fatalf("expected 2 threads on first page, got %d", len(firstPage))
	}

	// Use the last item on the page as the next cursor.
	nextCursor := firstPage[len(firstPage)-1].LastCursor
	secondPage, err := st.ListThreads("proj", "bob", nextCursor, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(secondPage) != 2 {
		t.Fatalf("expected 2 threads on second page after cursor %d, got %d", nextCursor, len(secondPage))
	}
	if secondPage[0].LastCursor >= nextCursor {
		t.Fatalf("expected older threads after cursor %d, got cursor %d", nextCursor, secondPage[0].LastCursor)
	}
}

func TestFileReservation(t *testing.T) {
	st := NewSQLiteTest(t)

	// Reserve a file
	res, err := st.Reserve(core.Reservation{
		AgentID:     "agent-1",
		Project:     "autarch",
		PathPattern: "pkg/events/*.go",
		Exclusive:   true,
		Reason:      "Refactoring events package",
		TTL:         30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if res.ID == "" {
		t.Error("expected reservation ID")
	}

	got, err := st.GetReservation(res.ID)
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}
	if got.AgentID != "agent-1" {
		t.Fatalf("expected reservation owner agent-1, got %s", got.AgentID)
	}

	// Check active reservations
	active, err := st.ActiveReservations("autarch")
	if err != nil {
		t.Fatalf("active reservations: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active reservation, got %d", len(active))
	}
	if active[0].AgentID != "agent-1" {
		t.Errorf("expected agent-1, got %s", active[0].AgentID)
	}

	// Check agent reservations
	agentRes, err := st.AgentReservations("agent-1")
	if err != nil {
		t.Fatalf("agent reservations: %v", err)
	}
	if len(agentRes) != 1 {
		t.Errorf("expected 1 reservation for agent-1, got %d", len(agentRes))
	}

	// Release the reservation
	if err := st.ReleaseReservation(res.ID, "agent-1"); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Verify it's no longer active
	active, _ = st.ActiveReservations("autarch")
	if len(active) != 0 {
		t.Errorf("expected 0 active reservations after release, got %d", len(active))
	}

	if _, err := st.GetReservation("does-not-exist"); err == nil {
		t.Fatal("expected get reservation to fail for missing id")
	}
}

func TestFileReservationOverlapSubsetAndSuperset(t *testing.T) {
	st := NewSQLiteTest(t)

	_, err := st.Reserve(core.Reservation{
		AgentID:     "agent-1",
		Project:     "autarch",
		PathPattern: "pkg/events/*.go",
		Exclusive:   true,
	})
	if err != nil {
		t.Fatalf("seed reserve: %v", err)
	}

	_, err = st.Reserve(core.Reservation{
		AgentID:     "agent-2",
		Project:     "autarch",
		PathPattern: "pkg/events/reconcile.go",
		Exclusive:   true,
	})
	if err == nil {
		t.Fatal("expected overlap conflict for subset path")
	}

	st2 := NewSQLiteTest(t)
	_, err = st2.Reserve(core.Reservation{
		AgentID:     "agent-1",
		Project:     "autarch",
		PathPattern: "pkg/events/reconcile.go",
		Exclusive:   true,
	})
	if err != nil {
		t.Fatalf("seed literal reserve: %v", err)
	}
	_, err = st2.Reserve(core.Reservation{
		AgentID:     "agent-2",
		Project:     "autarch",
		PathPattern: "pkg/events/*.go",
		Exclusive:   true,
	})
	if err == nil {
		t.Fatal("expected overlap conflict for superset glob")
	}
}

func TestFileReservationOverlapPartial(t *testing.T) {
	st := NewSQLiteTest(t)

	_, err := st.Reserve(core.Reservation{
		AgentID:     "agent-1",
		Project:     "autarch",
		PathPattern: "pkg/*/reconcile.go",
		Exclusive:   true,
	})
	if err != nil {
		t.Fatalf("seed reserve: %v", err)
	}

	_, err = st.Reserve(core.Reservation{
		AgentID:     "agent-2",
		Project:     "autarch",
		PathPattern: "pkg/events/*.go",
		Exclusive:   true,
	})
	if err == nil {
		t.Fatal("expected overlap conflict for partial glob intersection")
	}
}

func TestFileReservationSharedOverlapSemantics(t *testing.T) {
	st := NewSQLiteTest(t)

	_, err := st.Reserve(core.Reservation{
		AgentID:     "agent-1",
		Project:     "autarch",
		PathPattern: "pkg/events/*.go",
		Exclusive:   false,
	})
	if err != nil {
		t.Fatalf("seed shared reserve: %v", err)
	}

	_, err = st.Reserve(core.Reservation{
		AgentID:     "agent-2",
		Project:     "autarch",
		PathPattern: "pkg/events/reconcile.go",
		Exclusive:   false,
	})
	if err != nil {
		t.Fatalf("shared/shared overlap should be allowed: %v", err)
	}

	_, err = st.Reserve(core.Reservation{
		AgentID:     "agent-3",
		Project:     "autarch",
		PathPattern: "pkg/events/reconcile.go",
		Exclusive:   true,
	})
	if err == nil {
		t.Fatal("expected overlap conflict for exclusive against active shared")
	}
}

func TestInboxCounts(t *testing.T) {
	st := NewSQLiteTest(t)

	// Send 3 messages to bob
	for i := 1; i <= 3; i++ {
		_, err := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: core.Message{
			ID:      fmt.Sprintf("m%d", i),
			Project: "proj",
			From:    "alice",
			To:      []string{"bob"},
			Body:    fmt.Sprintf("Message %d", i),
		}})
		if err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// Check counts before reading
	total, unread, err := st.InboxCounts("proj", "bob")
	if err != nil {
		t.Fatalf("inbox counts: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if unread != 3 {
		t.Errorf("expected unread=3, got %d", unread)
	}

	// Mark one as read
	if err := st.MarkRead("proj", "m1", "bob"); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	// Check counts after reading
	total, unread, err = st.InboxCounts("proj", "bob")
	if err != nil {
		t.Fatalf("inbox counts: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total=3 after read, got %d", total)
	}
	if unread != 2 {
		t.Errorf("expected unread=2 after read, got %d", unread)
	}
}

func TestReservationExpiry(t *testing.T) {
	st := NewSQLiteTest(t)

	// Reserve with very short TTL (already expired)
	_, err := st.Reserve(core.Reservation{
		AgentID:     "agent-1",
		Project:     "autarch",
		PathPattern: "*.go",
		TTL:         -1 * time.Second, // Already expired
	})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// Should not appear in active reservations
	active, _ := st.ActiveReservations("autarch")
	if len(active) != 0 {
		t.Errorf("expected 0 active reservations (expired), got %d", len(active))
	}
}

func TestRecipientTracking(t *testing.T) {
	st := NewSQLiteTest(t)

	// Send message to multiple recipients
	msg := core.Message{
		ID:      "m1",
		Project: "proj",
		From:    "alice",
		To:      []string{"bob", "charlie"},
		CC:      []string{"dave"},
		Subject: "Meeting",
		Body:    "Let's meet",
	}
	_, err := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: msg})
	if err != nil {
		t.Fatalf("append event: %v", err)
	}

	// Mark read for one recipient
	if err := st.MarkRead("proj", "m1", "bob"); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	// Check status
	status, err := st.RecipientStatus("proj", "m1")
	if err != nil {
		t.Fatalf("recipient status: %v", err)
	}
	if len(status) != 3 { // bob, charlie, dave (To + CC)
		t.Errorf("expected 3 recipients, got %d", len(status))
	}

	// Bob should be marked read
	bobStatus, ok := status["bob"]
	if !ok {
		t.Fatal("bob not in status")
	}
	if bobStatus.ReadAt == nil {
		t.Error("bob should be marked read")
	}

	// Charlie should not be marked read
	charlieStatus, ok := status["charlie"]
	if !ok {
		t.Fatal("charlie not in status")
	}
	if charlieStatus.ReadAt != nil {
		t.Error("charlie should not be marked read")
	}

	// Mark ack for bob
	if err := st.MarkAck("proj", "m1", "bob"); err != nil {
		t.Fatalf("mark ack: %v", err)
	}

	status, _ = st.RecipientStatus("proj", "m1")
	if status["bob"].AckAt == nil {
		t.Error("bob should be marked ack'd")
	}
}

func TestMessageWithMetadata(t *testing.T) {
	st := NewSQLiteTest(t)

	// Create a message with subject, CC, and BCC
	msg := core.Message{
		ID:          "m1",
		Project:     "proj",
		From:        "alice",
		To:          []string{"bob"},
		CC:          []string{"charlie"},
		BCC:         []string{"dave"},
		Subject:     "Important Update",
		Body:        "Test message body",
		Importance:  "high",
		AckRequired: true,
	}
	_, err := st.AppendEvent(core.Event{Type: core.EventMessageCreated, Message: msg})
	if err != nil {
		t.Fatalf("append event: %v", err)
	}

	// Fetch from inbox and verify metadata is preserved
	msgs, err := st.InboxSince("proj", "bob", 0, 0)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Subject != "Important Update" {
		t.Errorf("expected subject 'Important Update', got '%s'", msgs[0].Subject)
	}
	if msgs[0].Importance != "high" {
		t.Errorf("expected importance 'high', got '%s'", msgs[0].Importance)
	}
	if !msgs[0].AckRequired {
		t.Error("expected ack_required=true")
	}
	if len(msgs[0].CC) != 1 || msgs[0].CC[0] != "charlie" {
		t.Errorf("expected CC=['charlie'], got %v", msgs[0].CC)
	}
	if len(msgs[0].BCC) != 1 || msgs[0].BCC[0] != "dave" {
		t.Errorf("expected BCC=['dave'], got %v", msgs[0].BCC)
	}
}

func TestSQLiteThreadBackfillIncludesSender(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "intermute.db")

	// Seed a pre-thread_index database, then let migrations/backfill run.
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := rawDB.Exec(schema); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := rawDB.Exec(
		`INSERT INTO events (cursor, id, type, agent, project, message_id, thread_id, from_agent, to_json, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "e1", string(core.EventMessageCreated), "alice", "proj", "m1", "thread-1", "alice", `["bob"]`, "Hello", now,
	); err != nil {
		t.Fatalf("seed events: %v", err)
	}
	if _, err := rawDB.Exec(
		`INSERT INTO messages (project, message_id, thread_id, from_agent, to_json, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"proj", "m1", "thread-1", "alice", `["bob"]`, "Hello", now,
	); err != nil {
		t.Fatalf("seed messages: %v", err)
	}
	if _, err := rawDB.Exec(
		`INSERT INTO inbox_index (project, agent, cursor, message_id)
		 VALUES (?, ?, ?, ?)`,
		"proj", "bob", 1, "m1",
	); err != nil {
		t.Fatalf("seed inbox: %v", err)
	}
	_ = rawDB.Close()

	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.db.Close()

	threadsBob, err := st.ListThreads("proj", "bob", 0, 10)
	if err != nil {
		t.Fatalf("list threads for bob: %v", err)
	}
	if len(threadsBob) != 1 {
		t.Fatalf("expected bob to see 1 thread, got %d", len(threadsBob))
	}

	threadsAlice, err := st.ListThreads("proj", "alice", 0, 10)
	if err != nil {
		t.Fatalf("list threads for alice: %v", err)
	}
	if len(threadsAlice) != 1 {
		t.Fatalf("expected sender alice to see 1 thread after backfill, got %d", len(threadsAlice))
	}
}
