package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

type Event = core.Event

// ThreadSummary represents a thread with aggregated metadata.
type ThreadSummary struct {
	ThreadID     string
	LastCursor   uint64
	MessageCount int
	LastFrom     string
	LastBody     string
	LastAt       time.Time
}

type Store interface {
	AppendEvent(ctx context.Context, ev Event) (uint64, error)
	InboxSince(ctx context.Context, project, agent string, cursor uint64, limit int) ([]core.Message, error)
	ThreadMessages(ctx context.Context, project, threadID string, cursor uint64) ([]core.Message, error)
	ListThreads(ctx context.Context, project, agent string, cursor uint64, limit int) ([]ThreadSummary, error)
	RegisterAgent(ctx context.Context, agent core.Agent) (core.Agent, error)
	Heartbeat(ctx context.Context, project, agentID string) (core.Agent, error)
	ListAgents(ctx context.Context, project string) ([]core.Agent, error)
	// Per-recipient tracking
	MarkRead(ctx context.Context, project, messageID, agentID string) error
	MarkAck(ctx context.Context, project, messageID, agentID string) error
	RecipientStatus(ctx context.Context, project, messageID string) (map[string]*core.RecipientStatus, error)
	// Inbox counts
	InboxCounts(ctx context.Context, project, agentID string) (total int, unread int, err error)
	// Agent metadata merge (PATCH semantics: incoming keys overwrite, absent keys preserved)
	UpdateAgentMetadata(ctx context.Context, agentID string, meta map[string]string) (core.Agent, error)
	// File reservations
	Reserve(ctx context.Context, r core.Reservation) (*core.Reservation, error)
	GetReservation(ctx context.Context, id string) (*core.Reservation, error)
	ReleaseReservation(ctx context.Context, id, agentID string) error
	ActiveReservations(ctx context.Context, project string) ([]core.Reservation, error)
	AgentReservations(ctx context.Context, agentID string) ([]core.Reservation, error)
	CheckConflicts(ctx context.Context, project, pathPattern string, exclusive bool) ([]core.ConflictDetail, error)
}

// InMemory is a minimal in-memory store for tests.
type InMemory struct {
	cursor      uint64
	agents      map[string]core.Agent
	inbox       map[string]map[string][]core.Message
	messages    map[string]map[string]core.Message      // project -> messageID -> message
	threadIndex map[string]map[string]map[string]uint64 // project -> threadID -> agent -> lastCursor
}

func NewInMemory() *InMemory {
	return &InMemory{
		agents:      make(map[string]core.Agent),
		inbox:       make(map[string]map[string][]core.Message),
		messages:    make(map[string]map[string]core.Message),
		threadIndex: make(map[string]map[string]map[string]uint64),
	}
}

func (m *InMemory) AppendEvent(_ context.Context, ev Event) (uint64, error) {
	m.cursor++
	if ev.Type != core.EventMessageCreated {
		return m.cursor, nil
	}
	project := ev.Message.Project
	if project == "" {
		project = ev.Project
	}
	recipients := ev.Message.To
	if len(recipients) == 0 && ev.Agent != "" {
		recipients = []string{ev.Agent}
	}
	ev.Message.Cursor = m.cursor
	ev.Message.Project = project
	if _, ok := m.inbox[project]; !ok {
		m.inbox[project] = make(map[string][]core.Message)
	}
	if _, ok := m.messages[project]; !ok {
		m.messages[project] = make(map[string]core.Message)
	}
	m.messages[project][ev.Message.ID] = ev.Message
	for _, agent := range recipients {
		m.inbox[project][agent] = append(m.inbox[project][agent], ev.Message)
	}
	// Update thread index if message has a thread ID
	if ev.Message.ThreadID != "" {
		if _, ok := m.threadIndex[project]; !ok {
			m.threadIndex[project] = make(map[string]map[string]uint64)
		}
		if _, ok := m.threadIndex[project][ev.Message.ThreadID]; !ok {
			m.threadIndex[project][ev.Message.ThreadID] = make(map[string]uint64)
		}
		// Add sender and all recipients to thread index
		participants := append([]string{ev.Message.From}, recipients...)
		for _, agent := range participants {
			m.threadIndex[project][ev.Message.ThreadID][agent] = m.cursor
		}
	}
	return m.cursor, nil
}

func (m *InMemory) InboxSince(_ context.Context, project, agent string, cursor uint64, limit int) ([]core.Message, error) {
	collect := func(msgs []core.Message) []core.Message {
		out := make([]core.Message, 0, len(msgs))
		for _, msg := range msgs {
			if msg.Cursor > cursor {
				out = append(out, msg)
			}
		}
		return out
	}
	var out []core.Message
	if project != "" {
		out = collect(m.inbox[project][agent])
	} else {
		for _, perAgent := range m.inbox {
			out = append(out, collect(perAgent[agent])...)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *InMemory) ThreadMessages(_ context.Context, project, threadID string, cursor uint64) ([]core.Message, error) {
	var out []core.Message
	projectMsgs := m.messages[project]
	if projectMsgs == nil {
		return out, nil
	}
	for _, msg := range projectMsgs {
		if msg.ThreadID == threadID && msg.Cursor > cursor {
			out = append(out, msg)
		}
	}
	// Sort by cursor ascending
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].Cursor > out[j].Cursor {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (m *InMemory) ListThreads(_ context.Context, project, agent string, cursor uint64, limit int) ([]ThreadSummary, error) {
	var out []ThreadSummary
	projectThreads := m.threadIndex[project]
	if projectThreads == nil {
		return out, nil
	}
	for threadID, agents := range projectThreads {
		lastCursor, ok := agents[agent]
		if !ok || lastCursor <= cursor {
			continue
		}
		// Find last message in thread
		var lastMsg core.Message
		var count int
		for _, msg := range m.messages[project] {
			if msg.ThreadID == threadID {
				count++
				if lastMsg.Cursor < msg.Cursor {
					lastMsg = msg
				}
			}
		}
		out = append(out, ThreadSummary{
			ThreadID:     threadID,
			LastCursor:   lastCursor,
			MessageCount: count,
			LastFrom:     lastMsg.From,
			LastBody:     lastMsg.Body,
			LastAt:       lastMsg.CreatedAt,
		})
	}
	// Sort by last cursor descending
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].LastCursor < out[j].LastCursor {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *InMemory) RegisterAgent(_ context.Context, agent core.Agent) (core.Agent, error) {
	// Check for session_id reuse
	if agent.SessionID != "" {
		for _, existing := range m.agents {
			if existing.SessionID == agent.SessionID {
				if time.Since(existing.LastSeen) < core.SessionStaleThreshold {
					return core.Agent{}, core.ErrActiveSessionConflict
				}
				// Reuse: update existing agent
				existing.Name = agent.Name
				existing.Capabilities = agent.Capabilities
				existing.Metadata = agent.Metadata
				existing.Status = agent.Status
				existing.LastSeen = time.Now().UTC()
				m.agents[existing.ID] = existing
				return existing, nil
			}
		}
	}
	if agent.ID == "" {
		agent.ID = agent.Name
	}
	if agent.SessionID == "" {
		agent.SessionID = agent.ID + "-session"
	}
	m.agents[agent.ID] = agent
	return agent, nil
}

func (m *InMemory) Heartbeat(_ context.Context, project, agentID string) (core.Agent, error) {
	agent, ok := m.agents[agentID]
	if !ok {
		return core.Agent{}, fmt.Errorf("agent not found")
	}
	if project != "" && agent.Project != project {
		return core.Agent{}, fmt.Errorf("agent not found")
	}
	agent.LastSeen = time.Now().UTC()
	m.agents[agentID] = agent
	return agent, nil
}

func (m *InMemory) ListAgents(_ context.Context, project string) ([]core.Agent, error) {
	var out []core.Agent
	for _, agent := range m.agents {
		if project == "" || agent.Project == project {
			out = append(out, agent)
		}
	}
	return out, nil
}

// MarkRead marks a message as read by a specific recipient (stub for in-memory store)
func (m *InMemory) MarkRead(_ context.Context, project, messageID, agentID string) error {
	return nil // In-memory store doesn't track per-recipient status
}

// MarkAck marks a message as acknowledged by a specific recipient (stub for in-memory store)
func (m *InMemory) MarkAck(_ context.Context, project, messageID, agentID string) error {
	return nil // In-memory store doesn't track per-recipient status
}

// RecipientStatus returns the read/ack status for all recipients (stub for in-memory store)
func (m *InMemory) RecipientStatus(_ context.Context, project, messageID string) (map[string]*core.RecipientStatus, error) {
	return make(map[string]*core.RecipientStatus), nil // In-memory store doesn't track per-recipient status
}

// InboxCounts returns total and unread counts (stub for in-memory store)
func (m *InMemory) InboxCounts(_ context.Context, project, agentID string) (int, int, error) {
	msgs := m.inbox[project][agentID]
	return len(msgs), len(msgs), nil // In-memory doesn't track read status, so all are "unread"
}

// UpdateAgentMetadata merges metadata keys into an existing agent (stub for in-memory store)
func (m *InMemory) UpdateAgentMetadata(_ context.Context, agentID string, meta map[string]string) (core.Agent, error) {
	agent, ok := m.agents[agentID]
	if !ok {
		return core.Agent{}, fmt.Errorf("agent not found")
	}
	if agent.Metadata == nil {
		agent.Metadata = make(map[string]string)
	}
	for k, v := range meta {
		agent.Metadata[k] = v
	}
	agent.LastSeen = time.Now().UTC()
	m.agents[agentID] = agent
	return agent, nil
}

// Reserve creates a file reservation (stub for in-memory store)
func (m *InMemory) Reserve(_ context.Context, r core.Reservation) (*core.Reservation, error) {
	return &r, nil // In-memory store doesn't track reservations
}

// GetReservation returns a reservation by ID (stub for in-memory store)
func (m *InMemory) GetReservation(_ context.Context, id string) (*core.Reservation, error) {
	return nil, core.ErrNotFound
}

// ReleaseReservation releases a file reservation (stub for in-memory store)
func (m *InMemory) ReleaseReservation(_ context.Context, id, agentID string) error {
	return nil // In-memory store doesn't track reservations
}

// ActiveReservations returns active reservations (stub for in-memory store)
func (m *InMemory) ActiveReservations(_ context.Context, project string) ([]core.Reservation, error) {
	return nil, nil // In-memory store doesn't track reservations
}

// AgentReservations returns an agent's reservations (stub for in-memory store)
func (m *InMemory) AgentReservations(_ context.Context, agentID string) ([]core.Reservation, error) {
	return nil, nil // In-memory store doesn't track reservations
}

// CheckConflicts returns conflicting reservations (stub for in-memory store)
func (m *InMemory) CheckConflicts(_ context.Context, project, pathPattern string, exclusive bool) ([]core.ConflictDetail, error) {
	return nil, nil // In-memory store doesn't track reservations
}
