package storage

import (
	"fmt"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

type Event = core.Event

type Store interface {
	AppendEvent(Event) (uint64, error)
	InboxSince(project, agent string, cursor uint64) ([]core.Message, error)
	RegisterAgent(agent core.Agent) (core.Agent, error)
	Heartbeat(agentID string) (core.Agent, error)
}

// InMemory is a minimal in-memory store for tests.
type InMemory struct {
	cursor uint64
	agents map[string]core.Agent
	inbox  map[string]map[string][]core.Message
}

func NewInMemory() *InMemory {
	return &InMemory{
		agents: make(map[string]core.Agent),
		inbox:  make(map[string]map[string][]core.Message),
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
	for _, agent := range recipients {
		m.inbox[project][agent] = append(m.inbox[project][agent], ev.Message)
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
