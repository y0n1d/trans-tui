package core

import (
	"context"
	"my-trans/internal/translator"
	"sync"
)

type Service struct {
	translator translator.Translator
	mu         sync.Mutex
	cancel     context.CancelFunc
}

func NewService(t translator.Translator) *Service {
	return &Service{translator: t}
}

func (s *Service) Translate(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	defer cancel()
	return s.translator.Translate(ctx, req)
}
