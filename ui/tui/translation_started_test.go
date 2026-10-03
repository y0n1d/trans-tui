package tui

import (
	"strings"
	"testing"

	"github.com/y0n1d/trans-tui/internal/core"
)

// ---------------------------------------------------------------------------
// TranslationStartedMsg: the IPC-initiated translation lifecycle raises and
// clears the existing Loading indicator — no second loading state.
// ---------------------------------------------------------------------------

// TestTranslationStartedMsgSetsLoading pins edge 1+2: the started message
// raises Loading, the view renders the configured loading text at the bottom,
// and the loading row is accounted for in the layout height.
func TestTranslationStartedMsgSetsLoading(t *testing.T) {
	m := newTestModel(t)

	r, _ := m.Update(core.TranslationStartedMsg{RequestID: "req-started-1"})
	next := r.(Model)

	if !next.Loading {
		t.Fatal("TranslationStartedMsg should set Loading")
	}
	view := next.renderView()
	if !strings.Contains(view, next.themeOr().LoadingText) {
		t.Errorf("loading text missing from view: %q", view)
	}
	assertRenderHeight(t, next, "after TranslationStartedMsg")
}

// TestTranslationResultMsgClearsLoadingAfterStarted pins edge 3: the outcome
// message emitted by the IPC handler after the translation finishes clears
// the indicator the started message raised.
func TestTranslationResultMsgClearsLoadingAfterStarted(t *testing.T) {
	m := newTestModel(t)
	r, _ := m.Update(core.TranslationStartedMsg{RequestID: "req-started-2"})
	m = r.(Model)
	if !m.Loading {
		t.Fatal("precondition: TranslationStartedMsg should set Loading")
	}

	r, _ = m.Update(core.TranslationResultMsg{
		RequestID:   "req-started-2",
		Source:      "Hello",
		Translation: "你好",
		SourceLang:  "en",
		TargetLang:  "zh",
	})
	next := r.(Model)

	if next.Loading {
		t.Error("TranslationResultMsg should clear Loading after started")
	}
	if len(next.Records) != 1 {
		t.Errorf("records = %d, want 1", len(next.Records))
	}
}

// TestTranslationErrorMsgClearsLoadingAfterStarted pins edge 4: a failed
// translation also clears the indicator instead of leaving it stuck on.
func TestTranslationErrorMsgClearsLoadingAfterStarted(t *testing.T) {
	m := newTestModel(t)
	r, _ := m.Update(core.TranslationStartedMsg{RequestID: "req-started-3"})
	m = r.(Model)
	if !m.Loading {
		t.Fatal("precondition: TranslationStartedMsg should set Loading")
	}

	r, _ = m.Update(core.TranslationErrorMsg{
		RequestID: "req-started-3",
		Source:    "Hello",
		Error:     "provider boom",
	})
	next := r.(Model)

	if next.Loading {
		t.Error("TranslationErrorMsg should clear Loading after started")
	}
	if next.Error == "" {
		t.Error("TranslationErrorMsg should surface the error")
	}
}
