package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/y0n1d/trans-tui/internal/translator"
)

// translateFunc adapts a plain function to translator.Translator so tests can
// script per-request behavior and coordinate through channels instead of
// sleeping to guess at timing.
type translateFunc func(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error)

func (f translateFunc) Translate(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	return f(ctx, req)
}

// testWaitTimeout only bounds how long a test waits before failing outright;
// it is never used to decide whether an expected event has happened.
const testWaitTimeout = 5 * time.Second

func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(testWaitTimeout):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func waitErr(t *testing.T, ch <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(testWaitTimeout):
		t.Fatalf("timed out waiting for %s", what)
		return nil
	}
}

// TestServiceTranslateSupersededRequestIsNotAFailure proves the latest-wins
// lifecycle: A starts executing inside the provider, B arrives, the service
// cancels A through its own latest-wins mechanism, and A must not report that
// cancellation as an ordinary translation failure while B still succeeds.
func TestServiceTranslateSupersededRequestIsNotAFailure(t *testing.T) {
	startedA := make(chan struct{})
	// aCallerErr carries the state of A's caller context at the moment A was
	// canceled. nil means the caller never canceled A, so the only possible
	// cancel source is the service's own latest-wins mechanism.
	aCallerErr := make(chan error, 1)

	callerACtx, cancelACaller := context.WithCancel(context.Background())
	defer cancelACaller()

	svc := NewService(translateFunc(func(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
		if req.Text != "A" {
			return translator.TranslationResult{Translation: "B result", Provider: "fake"}, nil
		}
		close(startedA)
		<-ctx.Done()
		aCallerErr <- callerACtx.Err()
		return translator.TranslationResult{}, ctx.Err()
	}))

	errA := make(chan error, 1)
	go func() {
		_, err := svc.Translate(callerACtx, translator.TranslationRequest{
			Text:       "A",
			SourceLang: "auto",
			TargetLang: "auto",
		})
		errA <- err
	}()

	// A is now executing inside the provider.
	waitSignal(t, startedA, "translate A to start")

	// B arrives; latest-wins cancels A as a side effect of this call.
	resultB, errB := svc.Translate(context.Background(), translator.TranslationRequest{
		Text:       "B",
		SourceLang: "auto",
		TargetLang: "auto",
	})
	if errB != nil {
		t.Fatalf("translate B: unexpected error: %v", errB)
	}
	if resultB.Translation != "B result" {
		t.Errorf("translate B translation = %q, want %q", resultB.Translation, "B result")
	}

	gotA := waitErr(t, errA, "translate A to return")
	if !errors.Is(gotA, ErrSuperseded) {
		t.Errorf("translate A error = %v, want ErrSuperseded", gotA)
	}
	if errors.Is(gotA, context.Canceled) {
		t.Errorf("translate A error must not surface as a plain context.Canceled translation failure: %v", gotA)
	}

	callerErr := waitErr(t, aCallerErr, "translate A cancellation to be observed")
	if callerErr != nil {
		t.Errorf("A must be canceled by the service's latest-wins mechanism, but A's caller context is already done: %v", callerErr)
	}
}

// TestServiceTranslateExternalCancellationStillReportsCanceled guards the
// other half of the distinction: cancellation originating from the caller
// context (shutdown, IPC deadline, ...) is still reported as
// context.Canceled, never as ErrSuperseded.
func TestServiceTranslateExternalCancellationStillReportsCanceled(t *testing.T) {
	started := make(chan struct{})
	svc := NewService(translateFunc(func(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
		close(started)
		<-ctx.Done()
		return translator.TranslationResult{}, ctx.Err()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Translate(ctx, translator.TranslationRequest{
			Text:       "A",
			SourceLang: "auto",
			TargetLang: "auto",
		})
		errCh <- err
	}()

	waitSignal(t, started, "translate A to start")
	cancel()

	got := waitErr(t, errCh, "translate A to return after caller cancellation")
	if !errors.Is(got, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", got)
	}
	if errors.Is(got, ErrSuperseded) {
		t.Errorf("external cancellation must not be reported as ErrSuperseded: %v", got)
	}
}

// TestServiceTranslateSupersededRequestKeepsProviderError proves only the
// cancellation caused by supersession is reclassified: a provider that fails
// with a real error while being superseded keeps its own error.
func TestServiceTranslateSupersededRequestKeepsProviderError(t *testing.T) {
	startedA := make(chan struct{})
	releaseA := make(chan struct{})
	providerErr := errors.New("provider: upstream unavailable")

	svc := NewService(translateFunc(func(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
		if req.Text != "A" {
			return translator.TranslationResult{Translation: "B result", Provider: "fake"}, nil
		}
		close(startedA)
		// A ignores its context: it is a genuine provider failure in flight,
		// not a context cancellation.
		<-releaseA
		return translator.TranslationResult{}, providerErr
	}))

	errA := make(chan error, 1)
	go func() {
		_, err := svc.Translate(context.Background(), translator.TranslationRequest{
			Text:       "A",
			SourceLang: "auto",
			TargetLang: "auto",
		})
		errA <- err
	}()
	waitSignal(t, startedA, "translate A to start")

	// B supersedes A while A's provider error is still pending.
	if _, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text:       "B",
		SourceLang: "auto",
		TargetLang: "auto",
	}); err != nil {
		t.Fatalf("translate B: unexpected error: %v", err)
	}

	close(releaseA)
	got := waitErr(t, errA, "translate A to return")
	if !errors.Is(got, providerErr) {
		t.Errorf("error = %v, want provider error %v", got, providerErr)
	}
	if errors.Is(got, ErrSuperseded) {
		t.Errorf("a provider failure must not be reclassified as superseded: %v", got)
	}
}
