package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/y0n1d/trans-tui/internal/core"
)

func viewportForTest(w, h int) viewport.Model {
	return viewport.New(w, h)
}

func TestRecordCardWidthInvariant(t *testing.T) {
	testCases := []struct {
		name          string
		record        core.TranslationRecord
		viewportWidth int
	}{
		{
			"80 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			80,
		},
		{
			"60 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			60,
		},
		{
			"40 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			40,
		},
		{
			"25 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hi", TargetLang: "ja", Translation: "你好"},
			25,
		},
		{
			"80 long CJK source",
			core.TranslationRecord{SourceLang: "zh", Source: "这是一个很长的翻译测试文本，需要确保在正确的宽度内换行不会溢出视口边界", TargetLang: "en", Translation: "This is a long translation test text that must wrap correctly within the viewport"},
			80,
		},
		{
			"40 long CJK source",
			core.TranslationRecord{SourceLang: "zh", Source: "这是一个很长的翻译测试文本需要换行", TargetLang: "en", Translation: "Long translation text"},
			40,
		},
		{
			"with provider",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは", Provider: "google", Model: "nmt"},
			60,
		},
		{
			"very narrow 25",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは世界"},
			25,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{
				viewport: viewportForTest(tc.viewportWidth, 20),
			}
			model.Records = append(model.Records, tc.record)
			model.buildSemanticMap(model.Records, tc.viewportWidth)

			card := model.renderRecordHighlighted(tc.record, tc.viewportWidth, 0)
			cardWidth := lipgloss.Width(card)

			if cardWidth != tc.viewportWidth {
				t.Errorf("viewport=%d card=%d (expected %d)", tc.viewportWidth, cardWidth, tc.viewportWidth)
			}
		})
	}
}

func TestErrorPanelBorderWidth(t *testing.T) {
	widths := []int{40, 60, 80, 120}
	for _, w := range widths {
		t.Run(func() string { return string(rune('0'+w/10)) + string(rune('0'+w%10)) }(), func(t *testing.T) {
			model := Model{
				terminalWidth:  w,
				terminalHeight: 24,
				viewport:       viewportForTest(w, 18),
				ready:          true,
			}
			model.Error = "Translation failed"

			panel := model.renderErrorPanel(w)
			panelWidth := lipgloss.Width(panel)

			if panelWidth != w {
				t.Errorf("terminal=%d panel=%d (expected %d)\npanel output:\n%s", w, panelWidth, w, panel)
			}

			if len(panel) == 0 {
				t.Errorf("panel is empty")
			}
		})
	}
}

func TestErrorPanelContentWidthInvariant(t *testing.T) {
	errors := []string{
		"short",
		"this is a moderately long error message that should still render correctly",
		"API key not set: environment variable DEEPSEEK_API_KEY is empty or missing",
	}

	for _, errMsg := range errors {
		t.Run(errMsg[:min(20, len(errMsg))], func(t *testing.T) {
			model := Model{
				terminalWidth:  80,
				terminalHeight: 24,
				viewport:       viewportForTest(80, 18),
				ready:          true,
			}
			model.Error = errMsg

			panel := model.renderErrorPanel(80)
			panelWidth := lipgloss.Width(panel)

			if panelWidth != 80 {
				t.Errorf("terminal=80 panel=%d for error %q", panelWidth, errMsg)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestErrorPanelDismissedLayout(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 18),
		ready:          true,
	}
	model.Error = "some error"

	panel := model.renderErrorPanel(80)
	if lipgloss.Width(panel) != 80 {
		t.Errorf("error panel width = %d, want 80", lipgloss.Width(panel))
	}

	model.Error = ""
	panel = model.renderErrorPanel(80)
	if panel != "" {
		t.Errorf("empty panel should return empty string, got %q", panel)
	}
}

func TestMouseEventsReachHandler(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 18),
		ready:          true,
		semRows: []semanticRow{
			{LineID: 0, StartChar: 0, EndChar: 5, ScreenX0: 2, ScreenX1: 7, Selectable: true},
		},
		semLines: []semanticLine{
			{text: []rune("hello")},
		},
	}

	wheelUp := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
		X:      40,
		Y:      10,
	}
	result, _ := model.Update(wheelUp)
	m := result.(Model)
	if m.viewport.YOffset != 0 {
		t.Errorf("wheel up should scroll, YOffset=%d", m.viewport.YOffset)
	}

	wheelDown := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
		X:      40,
		Y:      10,
	}
	result, _ = model.Update(wheelDown)
	m = result.(Model)
	_ = m

	leftClick := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      4,
		Y:      1,
	}
	result, _ = model.Update(leftClick)
	m = result.(Model)
	if !m.sel.selecting {
		t.Error("left click should start selection")
	}

	release := tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      4,
		Y:      1,
	}
	result, _ = model.Update(release)
	m = result.(Model)
	if m.sel.selecting {
		t.Error("release should end selection")
	}
}

func TestSemanticMapPersistedAfterTranslationResult(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 18),
		ready:          true,
	}

	if len(model.semRows) != 0 {
		t.Fatalf("initial semRows should be empty, got %d", len(model.semRows))
	}

	msg := core.TranslationResultMsg{
		RequestID:   "test-001",
		Source:      "Hello",
		Translation: "こんにちは",
		SourceLang:  "en",
		TargetLang:  "ja",
		Provider:    "google",
		Model:       "nmt",
	}
	result, _ := model.Update(msg)
	m := result.(Model)

	if len(m.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(m.Records))
	}
	if len(m.semRows) == 0 {
		t.Fatal("semRows should be non-empty after TranslationResult, got 0")
	}
	if len(m.semLines) == 0 {
		t.Fatal("semLines should be non-empty after TranslationResult, got 0")
	}

	rowIdx := firstSelectableRow(m)
	if rowIdx < 0 {
		t.Fatal("no selectable semantic row after TranslationResult")
	}
	row := m.semRows[rowIdx]
	if row.ScreenX0 != 2 {
		t.Errorf("first selectable row ScreenX0 = %d, want 2", row.ScreenX0)
	}
	if row.ScreenX1 <= row.ScreenX0 {
		t.Errorf("first selectable row ScreenX1 (%d) must be > ScreenX0 (%d)", row.ScreenX1, row.ScreenX0)
	}
}

func TestSemanticMapUpdatedAfterWindowSize(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 18),
		ready:          true,
	}
	longText := "The quick brown fox jumps over the lazy dog near the river bank"
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: longText,
		TargetLang: "ja", Translation: "翻訳テキスト",
	})

	model.buildSemanticMap(model.Records, 80)
	rowsAt80 := len(model.semRows)

	result := model.handleWindowSize(tea.WindowSizeMsg{Width: 40, Height: 24})
	result.buildSemanticMap(result.Records, result.viewport.Width)

	if len(result.semRows) == 0 {
		t.Fatal("semRows should be non-empty after WindowSize + buildSemanticMap")
	}
	if len(result.semRows) <= rowsAt80 {
		t.Errorf("semRows count should increase when viewport shrinks: at80=%d at40=%d", rowsAt80, len(result.semRows))
	}
}

func TestScreenToSelectionPointAfterTranslationResult(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 18),
		ready:          true,
	}

	msg := core.TranslationResultMsg{
		RequestID:   "test-002",
		Source:      "Hello",
		Translation: "こんにちは",
		SourceLang:  "en",
		TargetLang:  "ja",
	}
	result, _ := model.Update(msg)
	m := result.(Model)

	if len(m.semRows) == 0 {
		t.Fatal("semRows must be non-empty for screenToSelectionPoint to work")
	}

	pt := m.screenToSelectionPoint(4, 2)
	if pt == nil {
		t.Fatal("screenToSelectionPoint returned nil after TranslationResult")
	}
	if pt.VisualRow != firstSelectableRow(m) {
		t.Errorf("VisualRow = %d, want %d", pt.VisualRow, firstSelectableRow(m))
	}
}

func TestMultipleRecordsBuildSemanticMap(t *testing.T) {
	model := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 18),
		ready:          true,
	}

	for i, src := range []string{"Hello", "World", "Test"} {
		msg := core.TranslationResultMsg{
			RequestID:   string(rune('a' + i)),
			Source:      src,
			Translation: "翻訳",
			SourceLang:  "en",
			TargetLang:  "ja",
		}
		result, _ := model.Update(msg)
		model = result.(Model)
	}

	if len(model.Records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(model.Records))
	}
	if len(model.semRows) == 0 {
		t.Fatal("semRows should be non-empty after 3 translations")
	}

	expectedLines := 6
	if len(model.semLines) != expectedLines {
		t.Errorf("semLines = %d, want %d (2 per record)", len(model.semLines), expectedLines)
	}

	for i, row := range model.semRows {
		if !row.Selectable {
			continue
		}
		if row.ScreenX0 >= row.ScreenX1 {
			t.Errorf("selectable row %d: ScreenX0=%d >= ScreenX1=%d", i, row.ScreenX0, row.ScreenX1)
		}
	}
}

func TestSemanticMapWrappingMatchesLipgloss(t *testing.T) {
	viewportWidth := 80

	model := Model{
		terminalWidth:  viewportWidth,
		terminalHeight: 24,
		viewport:       viewportForTest(viewportWidth, 22),
		ready:          true,
	}

	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang:  "en",
		Source:      "The quick brown fox jumps over the lazy dog near the river bank and continues running",
		TargetLang:  "ja",
		Translation: "茶色の狐は川の近くで寝ている犬を飛び越えて走り続けます",
	})

	model.buildSemanticMap(model.Records, viewportWidth)

	// Count semantic rows for source text (LineID=0)
	sourceRows := 0
	for _, row := range model.semRows {
		if row.LineID == 0 {
			sourceRows++
		}
	}

	// Count rendered content lines for source text, excluding border lines
	style := RecordStyle.Width(viewportWidth - recordBorderPadding)
	srcText := "[en] The quick brown fox jumps over the lazy dog near the river bank and continues running"
	rendered := style.Render(srcText)
	renderedLines := 0
	for _, l := range strings.Split(rendered, "\n") {
		if lipgloss.Width(l) > 0 {
			renderedLines++
		}
	}
	borderLines := style.GetVerticalBorderSize()
	renderedContentLines := renderedLines - borderLines

	if sourceRows != renderedContentLines {
		t.Errorf("semantic source rows (%d) != rendered content lines (%d, total=%d border=%d)",
			sourceRows, renderedContentLines, renderedLines, borderLines)
	}

	// Verify screenToSelectionPoint works at each source row, accounting for
	// the record top-border placeholder that precedes the source rows.
	firstSourceRow := -1
	for i, row := range model.semRows {
		if row.LineID == 0 {
			firstSourceRow = i
			break
		}
	}
	if firstSourceRow < 0 {
		t.Fatal("no source semantic row found")
	}
	for i := 0; i < sourceRows; i++ {
		screenY := firstSourceRow + i + 1 // +1 for header row
		pt := model.screenToSelectionPoint(10, screenY)
		if pt == nil {
			t.Errorf("screenToSelectionPoint(10, %d) returned nil, expected valid point", screenY)
		}
	}
}
