package storage

import "github.com/mistakeknot/intermute/internal/core"

type Event = core.Event

type Store interface {
	AppendEvent(Event) (uint64, error)
	InboxSince(agent string, cursor uint64) ([]core.Message, error)
}

// InMemory is a minimal in-memory store for tests.
type InMemory struct {
	cursor uint64
}

func NewInMemory() *InMemory {
	return &InMemory{}
}

func (m *InMemory) AppendEvent(_ Event) (uint64, error) {
	m.cursor++
	return m.cursor, nil
}

func (m *InMemory) InboxSince(_ string, _ uint64) ([]core.Message, error) {
	return nil, nil
}
