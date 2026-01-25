package httpapi

import "github.com/mistakeknot/intermute/internal/storage"

type Service struct {
	store storage.Store
	bus   Broadcaster
}

type Broadcaster interface {
	Broadcast(agent string, event any)
}

func NewService(store storage.Store) *Service {
	return &Service{store: store}
}

func (s *Service) WithBroadcaster(b Broadcaster) *Service {
	s.bus = b
	return s
}
