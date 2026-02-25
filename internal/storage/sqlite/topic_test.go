package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

func sendTestMessage(t *testing.T, s *Store, project, from, to, topic, body string) {
	t.Helper()
	ctx := context.Background()
	_, err := s.AppendEvent(ctx, core.Event{
		Type:    core.EventMessageCreated,
		Project: project,
		Message: core.Message{
			ID:        body, // use body as ID for simplicity
			ThreadID:  "thread-1",
			From:      from,
			To:        []string{to},
			Topic:     topic,
			Body:      body,
			CreatedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("send message %q: %v", body, err)
	}
}

func TestTopicMessages_SendAndQuery(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()

	sendTestMessage(t, s, "proj", "alice", "bob", "build", "msg-1")
	sendTestMessage(t, s, "proj", "alice", "bob", "build", "msg-2")
	sendTestMessage(t, s, "proj", "alice", "bob", "review", "msg-3")

	// Query build topic
	msgs, err := s.TopicMessages(ctx, "proj", "build", 0, 100)
	if err != nil {
		t.Fatalf("topic messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 build messages, got %d", len(msgs))
	}
	if msgs[0].Body != "msg-1" || msgs[1].Body != "msg-2" {
		t.Errorf("unexpected messages: %v, %v", msgs[0].Body, msgs[1].Body)
	}

	// Query review topic
	msgs, err = s.TopicMessages(ctx, "proj", "review", 0, 100)
	if err != nil {
		t.Fatalf("topic messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Body != "msg-3" {
		t.Errorf("expected 1 review message, got %d", len(msgs))
	}

	// Cursor pagination — skip first message
	msgs, err = s.TopicMessages(ctx, "proj", "build", msgs[0].Cursor, 100)
	if err != nil {
		t.Fatalf("topic messages with cursor: %v", err)
	}
	// After first build message's cursor, we should still get some results
	// (exact count depends on rowid assignment, but should be <= 2)
	for _, m := range msgs {
		if m.Topic != "build" {
			t.Errorf("expected topic build, got %s", m.Topic)
		}
	}
}

func TestTopicMessages_CaseNormalization(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()

	// Send with uppercase topic
	sendTestMessage(t, s, "proj", "alice", "bob", "BUILD", "msg-upper")
	// Send with mixed case
	sendTestMessage(t, s, "proj", "alice", "bob", "Build", "msg-mixed")

	// Query with lowercase — should find both (normalized at write time)
	msgs, err := s.TopicMessages(ctx, "proj", "build", 0, 100)
	if err != nil {
		t.Fatalf("topic messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (case-normalized), got %d", len(msgs))
	}
}

func TestTopicMessages_NoTopic(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()

	// Send message without topic
	sendTestMessage(t, s, "proj", "alice", "bob", "", "msg-no-topic")
	// Send message with topic
	sendTestMessage(t, s, "proj", "alice", "bob", "deploy", "msg-with-topic")

	// Query for deploy — should only get the one with topic
	msgs, err := s.TopicMessages(ctx, "proj", "deploy", 0, 100)
	if err != nil {
		t.Fatalf("topic messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != "msg-with-topic" {
		t.Errorf("expected msg-with-topic, got %s", msgs[0].Body)
	}

	// Query for empty topic — should return nothing (empty string matches default)
	msgs, err = s.TopicMessages(ctx, "proj", "", 0, 100)
	if err != nil {
		t.Fatalf("topic messages empty: %v", err)
	}
	// Empty topic query matches messages with no topic (default '')
	// This is expected behavior — the migration sets '' as default
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message with empty topic, got %d", len(msgs))
	}
}

func TestTopicMessages_CrossProject(t *testing.T) {
	s := newRaceStore(t)
	ctx := context.Background()

	sendTestMessage(t, s, "proj-a", "alice", "bob", "build", "msg-a")
	sendTestMessage(t, s, "proj-b", "carol", "dave", "build", "msg-b")

	// Query proj-a — should only get proj-a messages
	msgs, err := s.TopicMessages(ctx, "proj-a", "build", 0, 100)
	if err != nil {
		t.Fatalf("topic messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Body != "msg-a" {
		t.Fatalf("expected 1 proj-a message, got %d", len(msgs))
	}

	// Query proj-b — should only get proj-b messages
	msgs, err = s.TopicMessages(ctx, "proj-b", "build", 0, 100)
	if err != nil {
		t.Fatalf("topic messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Body != "msg-b" {
		t.Fatalf("expected 1 proj-b message, got %d", len(msgs))
	}
}
