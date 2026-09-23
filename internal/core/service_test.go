package core

import (
	"context"
	"errors"
	"github.com/y0n1d/trans-tui/internal/translator"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockTranslator struct {
	delay     time.Duration
	result    translator.TranslationResult
	err       error
	called    int32
	cancelled int32
}

func (m *mockTranslator) Translate(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	atomic.AddInt32(&m.called, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			atomic.AddInt32(&m.cancelled, 1)
			return translator.TranslationResult{}, ctx.Err()
		}
	}
	if m.err != nil {
		return translator.TranslationResult{}, m.err
	}
	return m.result, nil
}

func TestService_Translate_Success(t *testing.T) {
	mock := &mockTranslator{
		result: translator.TranslationResult{
			Translation: "你好",
			Provider:    "mock",
		},
	}
	svc := NewService(mock)

	result, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text:       "Hello",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "你好" {
		t.Errorf("expected '你好', got '%s'", result.Translation)
	}
	if result.Provider != "mock" {
		t.Errorf("expected 'mock', got '%s'", result.Provider)
	}
}

func TestService_Translate_ErrorPreserved(t *testing.T) {
	originalErr := errors.New("API key not set: environment variable OPENAI_API_KEY is empty or missing")
	mock := &mockTranslator{err: originalErr}
	svc := NewService(mock)

	_, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text:       "Hello",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != originalErr.Error() {
		t.Errorf("expected '%s', got '%s'", originalErr.Error(), err.Error())
	}
	if errors.Is(err, context.Canceled) {
		t.Error("error should not be context.Canceled")
	}
}

func TestService_Translate_ErrorNotContextCanceled(t *testing.T) {
	testCases := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"api key not set", errors.New("API key not set: env var"), "API key not set"},
		{"http 401", errors.New("API returned status 401: unauthorized"), "401"},
		{"http 403", errors.New("DeepL API error (403): Authorization failed"), "403"},
		{"http 429", errors.New("API returned status 429: rate limited"), "429"},
		{"http 500", errors.New("API returned status 500: internal error"), "500"},
		{"network error", errors.New("request failed: dial tcp: connection refused"), "connection refused"},
		{"timeout", errors.New("request failed: context deadline exceeded"), "deadline exceeded"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockTranslator{err: tc.err}
			svc := NewService(mock)

			_, err := svc.Translate(context.Background(), translator.TranslationRequest{
				Text:       "Hello",
				SourceLang: "en",
				TargetLang: "zh",
			})

			if err == nil {
				t.Fatal("expected error")
			}
			if errors.Is(err, context.Canceled) {
				t.Errorf("error should not be context.Canceled, got: %s", err.Error())
			}
			if !containsStr(err.Error(), tc.wantMsg) {
				t.Errorf("expected '%s' in error, got: %s", tc.wantMsg, err.Error())
			}
		})
	}
}

// TestService_Translate_NewCancelsOld proves the latest-wins lifecycle: while
// the first request is provably in flight inside the provider, a second
// Translate supersedes it. Coordination goes through channels
// (translateFunc/waitSignal/waitErr from service_cancel_test.go) instead of
// sleeping to guess whether the first request has started or finished.
func TestService_Translate_NewCancelsOld(t *testing.T) {
	started := make(chan struct{})
	svc := NewService(translateFunc(func(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
		if req.Text == "Hello" {
			// First request: report the start, then block until the
			// service cancels us — as a real in-flight HTTP call would.
			close(started)
			<-ctx.Done()
			return translator.TranslationResult{}, ctx.Err()
		}
		return translator.TranslationResult{Translation: "result", Provider: "mock"}, nil
	}))

	// Start first request (blocks until it is superseded)
	firstErr := make(chan error, 1)
	go func() {
		_, err := svc.Translate(context.Background(), translator.TranslationRequest{
			Text:       "Hello",
			SourceLang: "en",
			TargetLang: "zh",
		})
		firstErr <- err
	}()

	// The first request is now in flight inside the provider.
	waitSignal(t, started, "first translate to start")

	// Start second request (should cancel first)
	result, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text:       "World",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "result" {
		t.Errorf("expected 'result', got '%s'", result.Translation)
	}

	// The first request can only return after its provider observed the
	// cancellation (the provider blocks on <-ctx.Done()), so its arrival —
	// not a sleep — is the observable proof that it was cancelled.
	gotFirst := waitErr(t, firstErr, "first translate to return after being superseded")
	if gotFirst == nil {
		t.Error("expected first request to be cancelled, it returned success")
	}
}

func TestService_Translate_NewDoesNotCancelSelf(t *testing.T) {
	mock := &mockTranslator{
		delay: 100 * time.Millisecond,
		result: translator.TranslationResult{
			Translation: "result",
			Provider:    "mock",
		},
	}
	svc := NewService(mock)

	result, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text:       "Hello",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "result" {
		t.Errorf("expected 'result', got '%s'", result.Translation)
	}
	// Should not be cancelled
	if atomic.LoadInt32(&mock.cancelled) > 0 {
		t.Error("request should not be cancelled")
	}
}

// TestService_Translate_ShutdownCancelsInflight simulates a shutdown: the
// caller cancels its context while the request is provably in flight inside
// the provider. Coordination goes through channels (translateFunc/waitSignal/
// waitErr from service_cancel_test.go) instead of sleeping to guess whether
// the request has started.
func TestService_Translate_ShutdownCancelsInflight(t *testing.T) {
	started := make(chan struct{})
	svc := NewService(translateFunc(func(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
		// The provider is executing: report the start, then block until the
		// cancellation reaches us — as a real in-flight HTTP call would.
		close(started)
		<-ctx.Done()
		return translator.TranslationResult{}, ctx.Err()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := svc.Translate(ctx, translator.TranslationRequest{
			Text:       "Hello",
			SourceLang: "en",
			TargetLang: "zh",
		})
		done <- err
	}()

	// The request is now in flight inside the provider.
	waitSignal(t, started, "translate to start")

	// Cancel the caller context (simulates shutdown).
	cancel()

	got := waitErr(t, done, "in-flight translate to return after cancellation")
	if got == nil {
		t.Fatal("expected error after cancellation")
	}
	if !errors.Is(got, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", got)
	}
}

func TestService_Translate_ConcurrentSafety(t *testing.T) {
	mock := &mockTranslator{
		delay: 10 * time.Millisecond,
		result: translator.TranslationResult{
			Translation: "result",
			Provider:    "mock",
		},
	}
	svc := NewService(mock)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Translate(context.Background(), translator.TranslationRequest{
				Text:       "Hello",
				SourceLang: "en",
				TargetLang: "zh",
			})
			// Some may be cancelled, that's ok
			_ = err
		}()
	}

	wg.Wait()
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
