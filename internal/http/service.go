package httpapi

import "github.com/mistakeknot/intermute/internal/storage"

type Service struct {
	store storage.Store
}

func NewService(store storage.Store) *Service {
	return &Service{store: store}
}
