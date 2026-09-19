package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/y0n1d/trans-tui/internal/core"
)

// clipboardRecorder is a fake clipboardWrite that records what was copied.
type clipboardRecorder struct {
	writes []string
	err    error
}

func (c *clipboardRecorder) write(text string) error {
	c.writes = append(c.writes, text)
	return c.err
}

func clipboardModel(rec *clipboardRecorder, records []core.TranslationRecord, width int) Model {
	model := Model{
		terminalWidth:  width,
		terminalHeight: 24,
		viewport:       viewportForTest(width, 20),
		ready:          true,
		clipboard:      rec.write,
	}
	model.Records = append(model.Records, records...)
	model.buildSemanticMap(model.Records, width)
	model.viewport.SetContent(model.renderRecordsWithHighlight())
	return model
}

// dragSelect performs press -> motion -> release and executes the command
// returned by the release, returning the resulting model and the message the
// command produced (nil on success).
func dragSelect(t *testing.T, model Model, x0, y0, x1, y1 int) (Model, tea.Msg) {
	t.Helper()
	r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x0, Y: y0})
	m := r.(Model)
	r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: x1, Y: y1})
	m = r.(Model)
	r, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: x1, Y: y1})
	m = r.(Model)
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

func TestReleaseCopiesAndClearsSelection(t *testing.T) {
	rec := &clipboardRecorder{}
	model := clipboardModel(rec, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
	}, 80)

	screenY := firstSelectableRow(model) + 1
	before := model.viewport.View()

	m, _ := dragSelect(t, model, 2, screenY, 7, screenY) // cells 0..5 -> "[en] "

	if len(rec.writes) != 1 {
		t.Fatalf("expected exactly 1 clipboard write, got %d (%q)", len(rec.writes), rec.writes)
	}
	if rec.writes[0] != "[en] " {
		t.Errorf("copied %q, want %q", rec.writes[0], "[en] ")
	}
	if m.sel != (selection{}) {
		t.Errorf("selection should be cleared after release, got %+v", m.sel)
	}
	if m.viewport.View() != before {
		t.Error("viewport should return to the unselected state after release")
	}
}

func TestEmptySelectionDoesNotCopy(t *testing.T) {
	rec := &clipboardRecorder{}
	model := clipboardModel(rec, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 80)

	screenY := firstSelectableRow(model) + 1

	// Plain click: press and release at the same point.
	r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 4, Y: screenY})
	m := r.(Model)
	r, cmd := m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 4, Y: screenY})
	m = r.(Model)

	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Errorf("empty selection produced a message: %#v", msg)
		}
	}
	if len(rec.writes) != 0 {
		t.Errorf("empty selection must not write to clipboard, got %q", rec.writes)
	}
	if m.sel != (selection{}) {
		t.Errorf("selection should be cleared after click, got %+v", m.sel)
	}
}

func TestReverseSelectionAutoCopy(t *testing.T) {
	records := []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
	}

	for _, tc := range []struct {
		name       string
		x0, x1     int
		wantCopied string
	}{
		{"forward", 2, 7, "[en] "},
		{"reverse", 7, 2, "[en] "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &clipboardRecorder{}
			model := clipboardModel(rec, records, 80)
			screenY := firstSelectableRow(model) + 1

			m, _ := dragSelect(t, model, tc.x0, screenY, tc.x1, screenY)

			if len(rec.writes) != 1 || rec.writes[0] != tc.wantCopied {
				t.Errorf("copied %q, want %q", rec.writes, tc.wantCopied)
			}
			if m.sel != (selection{}) {
				t.Errorf("selection should be cleared, got %+v", m.sel)
			}
		})
	}
}

func TestWrappedSelectionAutoCopy(t *testing.T) {
	const width = 30
	text := "abcdefghijklmnopqrstuvwxyz0123456789"
	rec := &clipboardRecorder{}
	model := clipboardModel(rec, []core.TranslationRecord{
		{SourceLang: "en", Source: text, TargetLang: "ja", Translation: "翻訳"},
	}, width)

	var srcRows []int
	for i, row := range model.semRows {
		if row.Selectable && row.LineID == 0 {
			srcRows = append(srcRows, i)
		}
	}
	if len(srcRows) < 2 {
		t.Fatalf("expected wrapped source rows, got %d", len(srcRows))
	}

	first := srcRows[0]
	last := srcRows[len(srcRows)-1]
	pressX := model.semRows[first].ScreenX0
	pressY := first + 1
	// Release past the end of the last row; clampDragPoint clamps to the row.
	releaseX := model.semRows[last].ScreenX1 + 5
	releaseY := last + 1

	m, _ := dragSelect(t, model, pressX, pressY, releaseX, releaseY)

	if len(rec.writes) != 1 {
		t.Fatalf("expected 1 clipboard write, got %d (%q)", len(rec.writes), rec.writes)
	}
	want := "[en] " + text
	if rec.writes[0] != want {
		t.Errorf("copied %q, want %q", rec.writes[0], want)
	}
	if m.sel != (selection{}) {
		t.Errorf("selection should be cleared, got %+v", m.sel)
	}
}

func TestCJKSelectionAutoCopy(t *testing.T) {
	rec := &clipboardRecorder{}
	model := clipboardModel(rec, []core.TranslationRecord{
		{SourceLang: "zh", Source: "你好世界", TargetLang: "en", Translation: "Hello world"},
	}, 80)

	screenY := firstSelectableRow(model) + 1
	// Source is "[zh] 你好世界": cells 5..8 are 你 and 好.
	m, _ := dragSelect(t, model, 7, screenY, 11, screenY)

	if len(rec.writes) != 1 {
		t.Fatalf("expected 1 clipboard write, got %d (%q)", len(rec.writes), rec.writes)
	}
	if rec.writes[0] != "你好" {
		t.Errorf("copied %q, want %q", rec.writes[0], "你好")
	}
	if m.sel != (selection{}) {
		t.Errorf("selection should be cleared, got %+v", m.sel)
	}
}

func TestCopyFailureDoesNotBreakUI(t *testing.T) {
	rec := &clipboardRecorder{err: errors.New("stderr closed")}
	model := clipboardModel(rec, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
	}, 80)

	screenY := firstSelectableRow(model) + 1

	m, msg := dragSelect(t, model, 2, screenY, 7, screenY)
	if len(rec.writes) != 1 {
		t.Fatalf("copy should be attempted once, got %d", len(rec.writes))
	}
	if msg == nil {
		t.Fatal("expected a clipboard error message")
	}
	errMsg, ok := msg.(clipboardErrorMsg)
	if !ok {
		t.Fatalf("expected clipboardErrorMsg, got %T", msg)
	}

	// The error must be handled without disturbing the cleared selection.
	r, _ := m.Update(errMsg)
	m = r.(Model)

	if m.sel != (selection{}) {
		t.Errorf("selection should stay cleared after copy failure, got %+v", m.sel)
	}
	if !strings.Contains(m.Error, "clipboard copy failed") {
		t.Errorf("expected a copy error in the error panel, got %q", m.Error)
	}
	if got := lipgloss.Height(m.renderView()); got != m.terminalHeight {
		t.Errorf("view height = %d, want %d (UI must stay consistent)", got, m.terminalHeight)
	}
}

func TestCopySuccessProducesNoMessage(t *testing.T) {
	rec := &clipboardRecorder{}
	model := clipboardModel(rec, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
	}, 80)

	screenY := firstSelectableRow(model) + 1
	if _, msg := dragSelect(t, model, 2, screenY, 7, screenY); msg != nil {
		t.Errorf("successful copy should not emit a message, got %#v", msg)
	}
}
