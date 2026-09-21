package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/y0n1d/trans-tui/internal/core"
)

// ---------------------------------------------------------------------------
// D1: semantic rows must align 1:1 with the rows actually rendered.
// ---------------------------------------------------------------------------

func TestSemanticRowsAlignWithRenderedRecord(t *testing.T) {
	cases := []struct {
		name   string
		record core.TranslationRecord
		width  int
	}{
		{
			"english no provider",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			80,
		},
		{
			"english with provider",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界", Provider: "deepseek", Model: "deepseek-flash"},
			80,
		},
		{
			"cjk wrapped",
			core.TranslationRecord{SourceLang: "zh", Source: "这是一个很长的测试文本用于检查换行是否正确", TargetLang: "en", Translation: "This is a long translation test text for wrapping checks"},
			40,
		},
		{
			"wrapped english",
			core.TranslationRecord{SourceLang: "en", Source: "The quick brown fox jumps over the lazy dog near the river bank and continues running", TargetLang: "ja", Translation: "茶色の狐は川の近くで寝ている犬を飛び越えて走り続けます"},
			40,
		},
		{
			"error record",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", Error: "network timeout while contacting upstream"},
			60,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{viewport: viewportForTest(tc.width, 40)}
			model.Records = append(model.Records, tc.record)
			model.buildSemanticMap(model.Records, tc.width)

			rendered := model.renderRecordHighlighted(tc.record, tc.width, 0)
			renderedRows := lipgloss.Height(rendered)

			if renderedRows != len(model.semRows) {
				t.Fatalf("rendered rows (%d) != semRows (%d)", renderedRows, len(model.semRows))
			}
			if model.semRows[0].Selectable {
				t.Error("top border row must not be selectable")
			}
			if model.semRows[len(model.semRows)-1].Selectable {
				t.Error("bottom border row must not be selectable")
			}
		})
	}
}

func TestSemanticRowsAlignWithRenderedRecords(t *testing.T) {
	records := []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
		{SourceLang: "en", Source: "Long source text that should wrap at a narrow width for testing", TargetLang: "ja", Translation: "長いテキスト", Provider: "deepseek", Model: "deepseek-flash"},
		{SourceLang: "zh", Source: "另一个测试", TargetLang: "en", Translation: "another test"},
	}
	const w = 50
	model := Model{viewport: viewportForTest(w, 40)}
	model.Records = append(model.Records, records...)
	model.buildSemanticMap(model.Records, w)

	rendered := model.renderRecordsWithHighlight()
	if got := lipgloss.Height(rendered); got != len(model.semRows) {
		t.Errorf("rendered lines (%d) != semRows (%d)", got, len(model.semRows))
	}
}

func TestSemanticRowSelectionPattern(t *testing.T) {
	model := Model{viewport: viewportForTest(80, 40)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好",
		Provider: "deepseek", Model: "deepseek-flash",
	})
	model.buildSemanticMap(model.Records, 80)

	if len(model.semRows) != 5 {
		t.Fatalf("expected 5 semantic rows (border/src/trans/provider/border), got %d", len(model.semRows))
	}

	want := []bool{false, true, true, false, false}
	for i, sel := range want {
		if model.semRows[i].Selectable != sel {
			t.Errorf("semRows[%d].Selectable = %v, want %v", i, model.semRows[i].Selectable, sel)
		}
	}
}

func TestBorderAndProviderRowsRejectSelectionStart(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 20),
		ready:          true,
	}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好",
		Provider: "deepseek", Model: "deepseek-flash",
	})
	model.buildSemanticMap(model.Records, 80)

	// top border is semRows[0] -> screen row 1 (after header), provider is row 3.
	for _, vr := range []int{0, 3} {
		screenY := vr + 1
		if pt := model.screenToSelectionPoint(4, screenY); pt != nil {
			t.Errorf("row %d (non-selectable) should reject selection start, got %+v", vr, pt)
		}
	}
	// source row must accept.
	if pt := model.screenToSelectionPoint(4, 1+1); pt == nil {
		t.Error("source row should accept selection start")
	}
}

func TestWrappedSelectionExtractsAcrossRows(t *testing.T) {
	const w = 30
	model := Model{viewport: viewportForTest(w, 40)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang:  "en",
		Source:      "abcdefghijklmnopqrstuvwxyz0123456789",
		TargetLang:  "ja",
		Translation: "翻訳",
	})
	model.buildSemanticMap(model.Records, w)

	var srcRows []int
	for i, row := range model.semRows {
		if row.Selectable && row.LineID == 0 {
			srcRows = append(srcRows, i)
		}
	}
	if len(srcRows) < 2 {
		t.Fatalf("expected source to wrap to >= 2 rows, got %d", len(srcRows))
	}

	last := model.semRows[srcRows[len(srcRows)-1]]
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRows[0], CellCol: 0},
		end:   SelectionPoint{VisualRow: srcRows[len(srcRows)-1], CellCol: last.ScreenX1 - last.ScreenX0},
	}
	got := model.extractSelectedText()
	want := "abcdefghijklmnopqrstuvwxyz0123456789"
	if got != want {
		t.Errorf("wrapped extraction = %q, want %q", got, want)
	}
}

func TestMouseDragSelectsExpectedText(t *testing.T) {
	newModel := func() Model {
		m := Model{
			terminalWidth:  80,
			terminalHeight: 24,
			viewport:       viewportForTest(80, 20),
			ready:          true,
		}
		m.Records = append(m.Records, core.TranslationRecord{
			SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界",
		})
		m.buildSemanticMap(m.Records, 80)
		m.viewport.SetContent(m.renderRecordsWithHighlight())
		return m
	}

	t.Run("forward", func(t *testing.T) {
		model := newModel()
		screenY := firstSelectableRow(model) + 1

		r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 2, Y: screenY})
		m := r.(Model)
		r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 9, Y: screenY})
		m = r.(Model)
		if got := m.extractSelectedText(); got != "Hello W" {
			t.Errorf("drag = %q, want %q", got, "Hello W")
		}
		r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 9, Y: screenY})
		m = r.(Model)
		if m.sel != (selection{}) {
			t.Errorf("selection should be cleared after release, got %+v", m.sel)
		}
	})

	t.Run("reverse", func(t *testing.T) {
		model := newModel()
		screenY := firstSelectableRow(model) + 1

		r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 9, Y: screenY})
		m := r.(Model)
		r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 2, Y: screenY})
		m = r.(Model)
		if got := m.extractSelectedText(); got != "Hello W" {
			t.Errorf("reverse drag = %q, want %q", got, "Hello W")
		}
		r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 2, Y: screenY})
		m = r.(Model)
		if m.sel != (selection{}) {
			t.Errorf("selection should be cleared after release, got %+v", m.sel)
		}
	})
}

// ---------------------------------------------------------------------------
// D2: selection state changes must refresh the viewport highlight.
// ---------------------------------------------------------------------------

func TestMouseSelectionRefreshesViewport(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(prev)

	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 20),
		ready:          true,
	}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界",
	})
	model.buildSemanticMap(model.Records, 80)
	model.viewport.SetContent(model.renderRecordsWithHighlight())

	srcRow := firstSelectableRow(model)
	screenY := srcRow + 1 // +1 for header row
	before := model.viewport.View()

	// press -> selection starts and viewport is refreshed with a cursor highlight
	r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 4, Y: screenY})
	m := r.(Model)
	if !m.sel.selecting {
		t.Fatal("press should start selection")
	}
	afterPress := m.viewport.View()
	if afterPress == before {
		t.Error("viewport content should change immediately after press")
	}

	// drag -> highlight follows endpoint
	r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 12, Y: screenY})
	m = r.(Model)
	afterDrag := m.viewport.View()
	if afterDrag == afterPress {
		t.Error("viewport content should change while dragging")
	}

	// release -> auto-copy, selection cleared, highlight removed
	r, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: 12, Y: screenY})
	m = r.(Model)
	if m.sel.selecting {
		t.Error("release should end selection")
	}
	if m.sel != (selection{}) {
		t.Errorf("selection should be cleared after release, got %+v", m.sel)
	}
	if m.viewport.View() != before {
		t.Error("highlight should be removed after release")
	}
}

func TestWheelDoesNotStartSelection(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 20),
		ready:          true,
	}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好",
	})
	model.buildSemanticMap(model.Records, 80)

	r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 4, Y: 2})
	m := r.(Model)
	if m.sel.selecting {
		t.Error("wheel must not start a selection")
	}
}

// ---------------------------------------------------------------------------
// D3: total view height must always equal terminal height.
// ---------------------------------------------------------------------------

func TestViewHeightInvariant(t *testing.T) {
	const H = 24
	cases := []struct {
		name    string
		err     string
		loading bool
	}{
		{"no error no loading", "", false},
		{"loading only", "", true},
		{"short error", "boom", false},
		{"long error wrapping", strings.Repeat("very long error message ", 30), false},
		{"error plus loading", strings.Repeat("another long error ", 25), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				terminalWidth:  80,
				terminalHeight: H,
				viewport:       viewportForTest(80, H-2),
				ready:          true,
				Error:          tc.err,
				Loading:        tc.loading,
			}
			m = m.recalcViewportHeight()
			m.viewport.SetContent(m.renderRecordsWithHighlight())

			if got := lipgloss.Height(m.renderView()); got != H {
				t.Errorf("View height = %d, want %d", got, H)
			}
		})
	}
}

func TestViewHeightInvariantAfterResize(t *testing.T) {
	m := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 22),
		ready:          true,
	}
	m.Records = append(m.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好",
	})
	m.viewport.SetContent(m.renderRecordsWithHighlight())

	for _, size := range []tea.WindowSizeMsg{
		{Width: 100, Height: 30},
		{Width: 60, Height: 18},
		{Width: 200, Height: 50},
	} {
		m = m.handleWindowSize(size)
		m.viewport.SetContent(m.renderRecordsWithHighlight())
		if got := lipgloss.Height(m.renderView()); got != size.Height {
			t.Errorf("after resize to %dx%d: View height = %d", size.Width, size.Height, got)
		}
	}
}

func TestViewHeightInvariantAfterErrorDismiss(t *testing.T) {
	const H = 24
	m := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-2),
		ready:          true,
	}
	m.Error = strings.Repeat("long error text ", 30)
	m = m.recalcViewportHeight()
	m.viewport.SetContent(m.renderRecordsWithHighlight())

	if got := lipgloss.Height(m.renderView()); got != H {
		t.Fatalf("with error: View height = %d, want %d", got, H)
	}

	r, _ := m.handleDismissError()
	m = r
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	if got := lipgloss.Height(m.renderView()); got != H {
		t.Errorf("after dismiss: View height = %d, want %d", got, H)
	}
}
