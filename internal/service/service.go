package service

import (
	"context"
	"sync"

	"orderissue/internal/usecase"
)

func New(d usecase.Deps) *Service {
	return &Service{d: d}
}

func (s *Service) readContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.d.ReadTimeout)
}

func (s *Service) writeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.d.WriteTimeout)
}

func (s *Service) lockOrder(ref string) func() {
	m, _ := s.orderLocks.LoadOrStore(ref, &sync.Mutex{})
	mu, _ := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
