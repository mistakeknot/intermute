package storage

import (
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
	AppendEvent(Event) (uint64, error)
	InboxSince(project, agent string, cursor uint64) ([]core.Message, error)
	ThreadMessages(project, threadID string, cursor uint64) ([]core.Message, error)
	ListThreads(project, agent string, cursor uint64, limit int) ([]ThreadSummary, error)
	RegisterAgent(agent core.Agent) (core.Agent, error)
	Heartbeat(project, agentID string) (core.Agent, error)
	ListAgents(project string) ([]core.Agent, error)
	// Per-recipient tracking
	MarkRead(project, messageID, agentID string) error
	MarkAck(project, messageID, agentID string) error
	RecipientStatus(project, messageID string) (map[string]*core.RecipientStatus, error)
}

// InMemory is a minimal in-memory store for tests.
type InMemory struct {
	cursor      uint64
	agents      map[string]core.Agent
	inbox       map[string]map[string][]core.Message
	messages    map[string]map[string]core.Message       // project -> messageID -> message
	threadIndex map[string]map[string]map[string]uint64  // project -> threadID -> agent -> lastCursor
}

func NewInMemory() *InMemory {
	return &InMemory{
		agents:      make(map[string]core.Agent),
		inbox:       make(map[string]map[string][]core.Message),
		messages:    make(map[string]map[string]core.Message),
		threadIndex: make(map[string]map[string]map[string]uint64),
	}
}

func (m *InMemory) AppendEvent(ev Event) (uint64, error) {
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

func (m *InMemory) InboxSince(project, agent string, cursor uint64) ([]core.Message, error) {
	collect := func(msgs []core.Message) []core.Message {
		out := make([]core.Message, 0, len(msgs))
		for _, msg := range msgs {
			if msg.Cursor > cursor {
				out = append(out, msg)
			}
		}
		return out
	}
	if project != "" {
		return collect(m.inbox[project][agent]), nil
	}
	var out []core.Message
	for _, perAgent := range m.inbox {
		out = append(out, collect(perAgent[agent])...)
	}
	return out, nil
}

func (m *InMemory) ThreadMessages(project, threadID string, cursor uint64) ([]core.Message, error) {
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

func (m *InMemory) ListThreads(project, agent string, cursor uint64, limit int) ([]ThreadSummary, error) {
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

func (m *InMemory) RegisterAgent(agent core.Agent) (core.Agent, error) {
	if agent.ID == "" {
		agent.ID = agent.Name
	}
	if agent.SessionID == "" {
		agent.SessionID = agent.ID + "-session"
	}
	m.agents[agent.ID] = agent
	return agent, nil
}

func (m *InMemory) Heartbeat(project, agentID string) (core.Agent, error) {
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

func (m *InMemory) ListAgents(project string) ([]core.Agent, error) {
	var out []core.Agent
	for _, agent := range m.agents {
		if project == "" || agent.Project == project {
			out = append(out, agent)
		}
	}
	return out, nil
}

// MarkRead marks a message as read by a specific recipient (stub for in-memory store)
func (m *InMemory) MarkRead(project, messageID, agentID string) error {
	return nil // In-memory store doesn't track per-recipient status
}

// MarkAck marks a message as acknowledged by a specific recipient (stub for in-memory store)
func (m *InMemory) MarkAck(project, messageID, agentID string) error {
	return nil // In-memory store doesn't track per-recipient status
}

// RecipientStatus returns the read/ack status for all recipients (stub for in-memory store)
func (m *InMemory) RecipientStatus(project, messageID string) (map[string]*core.RecipientStatus, error) {
	return make(map[string]*core.RecipientStatus), nil // In-memory store doesn't track per-recipient status
}
