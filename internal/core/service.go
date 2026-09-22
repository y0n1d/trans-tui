package core

import (
	"context"
	"errors"
	"github.com/y0n1d/trans-tui/internal/translator"
	"sync"
)

// ErrSuperseded reports that a translation was abandoned because a newer
// Translate call superseded it (latest-wins). It is a lifecycle outcome, not a
// translation failure, so callers must not surface it as a provider error.
// Cancellation coming from the caller's context (shutdown, deadline, ...) is
// still reported as context.Canceled.
var ErrSuperseded = errors.New("translation superseded by a newer request")

type Service struct {
	translator translator.Translator
	mu         sync.Mutex
	cancel     context.CancelCauseFunc
}

func NewService(t translator.Translator) *Service {
	return &Service{translator: t}
}

func (s *Service) Translate(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	req.TargetLang = ResolveTargetLang(req.TargetLang, req.Text)

	s.mu.Lock()
	if s.cancel != nil {
		// Latest wins: cancel the in-flight call and tag the cause so that
		// call can tell this apart from an external cancellation.
		s.cancel(ErrSuperseded)
	}
	ctx, cancel := context.WithCancelCause(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	defer cancel(nil)
	result, err := s.translator.Translate(ctx, req)
	if err != nil {
		// Only the cancellation caused by supersession is rewritten; a
		// caller/shutdown cancellation keeps context.Canceled, and a real
		// provider error passes through untouched.
		if errors.Is(err, context.Canceled) && errors.Is(context.Cause(ctx), ErrSuperseded) {
			return result, ErrSuperseded
		}
		return result, err
	}
	result.TargetLang = req.TargetLang
	return result, nil
}
