package tui

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/core"
	"github.com/y0n1d/trans-tui/internal/translator"
)

// stubTranslator is a deterministic translator: it records every request and
// replays a canned outcome. No network, no timing.
type stubTranslator struct {
	mu       sync.Mutex
	requests []translator.TranslationRequest
	result   translator.TranslationResult
	err      error
}

func (s *stubTranslator) Translate(_ context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	s.mu.Unlock()
	return s.result, s.err
}

func (s *stubTranslator) recorded() []translator.TranslationRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]translator.TranslationRequest(nil), s.requests...)
}

// mustUpdate drives one message through Update and requires the model back as
// a Model, so a message-type regression fails loudly instead of panicking
// later.
func mustUpdate(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// TestInitialTranslationScheduledByFirstWindowSizeMsg pins the synchronization
// contract that replaced the fixed 200ms startup sleep: the initial
// translation is scheduled exactly once, from the first WindowSizeMsg — the
// message that creates the viewport and sets m.ready — and never from a later
// resize. The trigger is derived from model state, so it needs no delay.
func TestInitialTranslationScheduledByFirstWindowSizeMsg(t *testing.T) {
	stub := &stubTranslator{}
	m := New(core.AppState{}, core.NewService(stub), "hello", "auto", "zh-CN", false, false, DefaultKeyMap(), DefaultTheme())

	if m.ready {
		t.Fatal("model must start with uninitialized layout")
	}

	// Before layout initialization no message schedules the initial
	// translation: there is nothing to wait for yet.
	m, cmd := mustUpdate(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd != nil {
		t.Fatalf("pre-layout messages must not schedule the initial translation, got cmd producing %v", cmd())
	}

	// The first WindowSizeMsg is the observable end of layout initialization:
	// it creates the viewport, sets ready, and schedules exactly one
	// InitialTranslationMsg.
	m, cmd = mustUpdate(t, m, tea.WindowSizeMsg{Width: 40, Height: 6})
	if !m.ready {
		t.Fatal("first WindowSizeMsg must initialize layout")
	}
	if cmd == nil {
		t.Fatal("first WindowSizeMsg must schedule the initial translation")
	}
	scheduled := cmd()
	if _, ok := scheduled.(InitialTranslationMsg); !ok {
		t.Fatalf("scheduled message = %T, want InitialTranslationMsg", scheduled)
	}

	// Later resizes must not re-trigger it.
	m, cmd = mustUpdate(t, m, tea.WindowSizeMsg{Width: 60, Height: 12})
	if cmd != nil {
		t.Fatalf("later WindowSizeMsg must not schedule another initial translation, got cmd producing %v", cmd())
	}
}

// TestInitialTranslationFlowThroughMessageSequence drives the exact message
// sequence Bubble Tea produces — first WindowSizeMsg → scheduled
// InitialTranslationMsg → translation command → TranslationResultMsg —
// executing each returned Cmd directly against the stub translator. The
// assertions cover ordering (no request before InitialTranslationMsg is
// handled), request contents, history, loading and error state. Everything is
// synchronous: there is no sleep and no timing guess anywhere.
func TestInitialTranslationFlowThroughMessageSequence(t *testing.T) {
	stub := &stubTranslator{result: translator.TranslationResult{
		Translation: "你好世界",
		Provider:    "stub",
		Model:       "stub-1",
	}}
	m := New(core.AppState{}, core.NewService(stub), "hello world", "auto", "zh-CN", false, false, DefaultKeyMap(), DefaultTheme())

	// 1. Layout initialization schedules the initial translation.
	m, cmd := mustUpdate(t, m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if cmd == nil {
		t.Fatal("first WindowSizeMsg must schedule the initial translation")
	}

	// 2. The scheduled command yields InitialTranslationMsg; handling it turns
	// on loading and returns the translation command. Nothing has been sent
	// to the provider yet.
	msg := cmd()
	if _, ok := msg.(InitialTranslationMsg); !ok {
		t.Fatalf("scheduled message = %T, want InitialTranslationMsg", msg)
	}
	m, cmd = mustUpdate(t, m, msg)
	if !m.Loading {
		t.Fatal("initial translation must show the loading state")
	}
	if len(stub.recorded()) != 0 {
		t.Fatal("provider must not be called before InitialTranslationMsg is handled")
	}
	if cmd == nil {
		t.Fatal("InitialTranslationMsg must return the translation command")
	}

	// 3. The translation command produces the result message.
	msg = cmd()
	if _, ok := msg.(core.TranslationResultMsg); !ok {
		t.Fatalf("translation command produced %T, want core.TranslationResultMsg", msg)
	}

	// 4. Delivering the result completes the flow with unchanged UI state.
	m, cmd = mustUpdate(t, m, msg)
	if cmd != nil {
		t.Fatal("TranslationResultMsg must not schedule further commands")
	}
	if len(m.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(m.Records))
	}
	rec := m.Records[0]
	if rec.Source != "hello world" || rec.Translation != "你好世界" {
		t.Errorf("record = %+v, want source %q translated to %q", rec, "hello world", "你好世界")
	}
	if m.Loading {
		t.Error("loading must clear after the result")
	}
	if m.Error != "" {
		t.Errorf("error = %q, want empty", m.Error)
	}
	if m.LastFailed != nil {
		t.Error("LastFailed must stay nil on success")
	}

	reqs := stub.recorded()
	if len(reqs) != 1 {
		t.Fatalf("provider requests = %d, want exactly 1 (no duplicates, no drops)", len(reqs))
	}
	if reqs[0].Text != "hello world" || reqs[0].SourceLang != "auto" || reqs[0].TargetLang != "zh-CN" {
		t.Errorf("provider request = %+v, want the startup text and configured languages", reqs[0])
	}
}

// TestInitialTranslationErrorFlowKeepsStateContract covers the failure branch
// of the same sequence: a provider error must surface through the error panel,
// clear loading, remember the retry candidate, and append no record.
func TestInitialTranslationErrorFlowKeepsStateContract(t *testing.T) {
	stub := &stubTranslator{err: errors.New("provider exploded")}
	m := New(core.AppState{}, core.NewService(stub), "hello world", "auto", "zh-CN", false, false, DefaultKeyMap(), DefaultTheme())

	m, cmd := mustUpdate(t, m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if cmd == nil {
		t.Fatal("first WindowSizeMsg must schedule the initial translation")
	}
	m, cmd = mustUpdate(t, m, cmd())
	if cmd == nil {
		t.Fatal("InitialTranslationMsg must return the translation command")
	}
	msg := cmd()
	if _, ok := msg.(core.TranslationErrorMsg); !ok {
		t.Fatalf("translation command produced %T, want core.TranslationErrorMsg", msg)
	}
	m, _ = mustUpdate(t, m, msg)

	if m.Error != "provider exploded" {
		t.Errorf("error = %q, want the provider error", m.Error)
	}
	if m.Loading {
		t.Error("loading must clear after an error")
	}
	if m.LastFailed == nil {
		t.Fatal("LastFailed must remember the failed attempt for retry")
	}
	if m.LastFailed.Source != "hello world" {
		t.Errorf("LastFailed.Source = %q, want the startup text", m.LastFailed.Source)
	}
	if len(m.Records) != 0 {
		t.Errorf("records = %d, want 0: an error must not append a history record", len(m.Records))
	}
}

// TestInitialTranslationOrderingAffectsHistoryScroll documents the race the
// fixed 200ms sleep was papering over. The runtime used to inject
// InitialTranslationMsg from its own goroutine while Bubble Tea delivered the
// program's initial WindowSizeMsg from another goroutine — both onto the same
// unbuffered Program.Send channel — so which one the model handled first was
// up to the scheduler; the sleep only made one order likely, not guaranteed.
// Driving both orders deterministically shows they are not equivalent: handled
// before the first WindowSizeMsg, the newest record is rendered against a
// viewport that does not exist yet, and the WindowSizeMsg handler then creates
// a fresh, top-aligned viewport — the record ends up below the fold. The new
// scheduleInitialTranslation trigger makes the safe order the only order the
// program can produce.
func TestInitialTranslationOrderingAffectsHistoryScroll(t *testing.T) {
	newModel := func(t *testing.T) Model {
		t.Helper()
		// displayMode appends the record synchronously inside
		// handleInitialTranslation, and the tiny terminal guarantees the card
		// overflows the history viewport so the scroll assertion is real.
		return New(core.AppState{}, &core.Service{}, "OCRed text", "auto", "auto", false, true, DefaultKeyMap(), DefaultTheme())
	}

	t.Run("after layout initialization (production order)", func(t *testing.T) {
		m := newModel(t)

		m, cmd := mustUpdate(t, m, tea.WindowSizeMsg{Width: 40, Height: 4})
		if cmd == nil {
			t.Fatal("first WindowSizeMsg must schedule the initial translation")
		}
		m, _ = mustUpdate(t, m, cmd())

		if len(m.Records) != 1 || m.Records[0].ID != "display-initial" {
			t.Fatalf("records = %+v, want the display-initial record", m.Records)
		}
		if total, visible := m.viewport.TotalLineCount(), m.viewport.VisibleLineCount(); total <= visible {
			t.Fatalf("test terminal must overflow the card (content %d lines, viewport %d), otherwise the scroll assertion is vacuous", total, visible)
		}
		if !m.viewport.AtBottom() {
			t.Errorf("history must show the newest record: YOffset = %d, want bottom", m.viewport.YOffset())
		}
	})

	t.Run("before layout initialization (the old race order)", func(t *testing.T) {
		m := newModel(t)

		// InitialTranslationMsg handled with no viewport yet...
		m, _ = mustUpdate(t, m, InitialTranslationMsg{})
		if len(m.Records) != 1 {
			t.Fatalf("records = %+v, want the display-initial record", m.Records)
		}
		// ...then the first WindowSizeMsg arrives, as Bubble Tea's own
		// startup goroutine could once race it to the model.
		m, _ = mustUpdate(t, m, tea.WindowSizeMsg{Width: 40, Height: 4})

		if m.viewport.AtBottom() {
			t.Fatal("this documents the old hazard: the record was expected below the fold after a pre-layout handling order")
		}
	})
}

// observeModel wraps the production Model without changing its behavior; it
// only reports when a TranslationResultMsg has actually been handled, which is
// the deterministic hand-off point for the test below.
type observeModel struct {
	Model
	resultSeen chan<- struct{}
}

func (m observeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.Model.Update(msg)
	next := observeModel{Model: inner.(Model), resultSeen: m.resultSeen}
	if _, ok := msg.(core.TranslationResultMsg); ok {
		select {
		case m.resultSeen <- struct{}{}:
		default:
		}
	}
	return next, cmd
}

// TestInitialTranslationDeliveredByRealProgram runs the production TUI inside
// a real Bubble Tea program with no fixed delay anywhere: Bubble Tea delivers
// its own initial WindowSizeMsg, the model schedules the initial translation
// from that boundary, and the stub provider's result reaches the model. The
// resultSeen channel is the success condition; the time.After guards are
// fail-fast watchdogs only and can never make the test pass.
func TestInitialTranslationDeliveredByRealProgram(t *testing.T) {
	stub := &stubTranslator{result: translator.TranslationResult{
		Translation: "你好世界",
		Provider:    "stub",
		Model:       "stub-1",
	}}
	m := New(core.AppState{}, core.NewService(stub), "hello world", "auto", "zh-CN", false, false, DefaultKeyMap(), DefaultTheme())

	resultSeen := make(chan struct{}, 1)
	p := tea.NewProgram(
		observeModel{Model: m, resultSeen: resultSeen},
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
	)

	runDone := make(chan error, 1)
	var final tea.Model
	go func() {
		var err error
		final, err = p.Run()
		runDone <- err
	}()

	select {
	case <-resultSeen:
		// The initial translation was scheduled, sent, and its result handled
		// — all through real Bubble Tea message scheduling, with no startup
		// timer in the runtime.
	case <-time.After(5 * time.Second):
		p.Kill()
		<-runDone
		t.Fatal("watchdog: the initial translation result never reached the model")
	}
	p.Quit()

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("program run: %v", err)
		}
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("watchdog: program did not quit after the result")
	}

	finalModel := final.(observeModel).Model
	if len(finalModel.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(finalModel.Records))
	}
	rec := finalModel.Records[0]
	if rec.Source != "hello world" || rec.Translation != "你好世界" {
		t.Errorf("record = %+v, want source %q translated to %q", rec, "hello world", "你好世界")
	}
	if finalModel.Loading {
		t.Error("loading must clear after the result")
	}
	if finalModel.Error != "" {
		t.Errorf("error = %q, want empty", finalModel.Error)
	}
	reqs := stub.recorded()
	if len(reqs) != 1 {
		t.Fatalf("provider requests = %d, want exactly 1", len(reqs))
	}
	if reqs[0].Text != "hello world" {
		t.Errorf("provider request text = %q, want the startup text", reqs[0].Text)
	}
}
