package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"my-trans/internal/core"
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

			card := model.renderRecordHighlighted(tc.record, tc.viewportWidth)
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
			{LineID: 0, StartChar: 0, EndChar: 5, ScreenX0: 2, ScreenX1: 7},
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
