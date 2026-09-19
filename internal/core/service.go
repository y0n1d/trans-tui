package core

import (
	"context"
	"github.com/y0n1d/trans-tui/internal/translator"
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
	req.TargetLang = ResolveTargetLang(req.TargetLang, req.Text)

	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	defer cancel()
	result, err := s.translator.Translate(ctx, req)
	if err != nil {
		return result, err
	}
	result.TargetLang = req.TargetLang
	return result, nil
}
