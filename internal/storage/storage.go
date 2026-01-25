package storage

import (
	"fmt"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

type Event = core.Event

type Store interface {
	AppendEvent(Event) (uint64, error)
	InboxSince(agent string, cursor uint64) ([]core.Message, error)
	RegisterAgent(agent core.Agent) (core.Agent, error)
	Heartbeat(agentID string) (core.Agent, error)
}

// InMemory is a minimal in-memory store for tests.
type InMemory struct {
	cursor uint64
	agents map[string]core.Agent
}

func NewInMemory() *InMemory {
	return &InMemory{agents: make(map[string]core.Agent)}
}

func (m *InMemory) AppendEvent(_ Event) (uint64, error) {
	m.cursor++
	return m.cursor, nil
}

func (m *InMemory) InboxSince(_ string, _ uint64) ([]core.Message, error) {
	return nil, nil
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

func (m *InMemory) Heartbeat(agentID string) (core.Agent, error) {
	agent, ok := m.agents[agentID]
	if !ok {
		return core.Agent{}, fmt.Errorf("agent not found")
	}
	agent.LastSeen = time.Now().UTC()
	m.agents[agentID] = agent
	return agent, nil
}
