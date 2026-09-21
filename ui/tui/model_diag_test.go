package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/y0n1d/trans-tui/internal/core"
)

// TestModelInputSimulate sends real keypresses through the Model.Update path
// and inspects what textarea.View() produces after each keystroke.
func TestModelInputSimulate(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	m.terminalWidth = 80
	m.terminalHeight = 24
	m.ready = true

	// Simulate first WindowSizeMsg to set textarea width properly.
	m = m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = m.recalcViewportHeight()

	// After handleWindowSize: inputPanelContentWidth() = 80 - 2 = 78
	// textarea.SetWidth(78) → content width = 78 - 2(prompt) = 76
	panelW := m.inputPanelContentWidth()
	contentW := panelW - 2 // prompt is 2
	t.Logf("panelW=%d contentW=%d textarea.Width()=%d", panelW, contentW, m.textArea.Width())
	if m.textArea.Width() != contentW {
		t.Fatalf("textarea.Width()=%d, want %d", m.textArea.Width(), contentW)
	}

	// Type 80 characters through the real Update path.
	input := strings.Repeat("1234", 20)
	for i, ch := range input {
		r, _ := m.Update(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
		m = r.(Model)
		_ = i
	}

	// After typing 80 chars, content should wrap.
	// 76 chars fit on line 0, remaining 4 on line 1.
	ta := m.textArea
	if ta.Height() != 2 {
		t.Errorf("Height()=%d, want 2 (80 chars with contentW=%d)", ta.Height(), contentW)
	}

	view := ta.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("view lines=%d, want 2", len(lines))
	}

	// Each view line should be exactly panelW wide (prompt + content + padding).
	// But we can't use len() on stripped strings because box-drawing chars
	// are multi-byte. Instead, just verify the content is correct:
	// line 0: "> " + 76 chars of "1234..."
	// line 1: "  " + 4 chars of "1234" + padding
	stripped0 := stripAnsi(lines[0])
	stripped1 := stripAnsi(lines[1])

	if !strings.HasPrefix(stripped0, "> ") {
		t.Errorf("line 0 should start with '> ', got %q", stripped0[:min(10, len(stripped0))])
	}
	if !strings.HasPrefix(stripped1, "  ") {
		t.Errorf("line 1 should start with '  ', got %q", stripped1[:min(10, len(stripped1))])
	}

	// Content on line 0 should be 76 chars
	content0 := strings.TrimPrefix(stripped0, "> ")
	content1 := strings.TrimPrefix(stripped1, "  ")
	// Trim trailing spaces (padding)
	content0 = strings.TrimRight(content0, " ")
	content1 = strings.TrimRight(content1, " ")

	if len(content0) != contentW {
		t.Errorf("line 0 content length=%d, want %d", len(content0), contentW)
	}
	if len(content1) != 4 {
		t.Errorf("line 1 content length=%d, want 4 (80-%d=%d)", len(content1), contentW, 80-contentW)
	}

	t.Logf("PASS: line0=%d chars, line1=%d chars", len(content0), len(content1))
}

// TestModelInputModeFocusOnStartup verifies that -i startup properly focuses textarea.
func TestModelInputModeFocusOnStartup(t *testing.T) {
	service := &core.Service{}
	m := New(core.AppState{}, service, "", "auto", "auto", true, false, DefaultKeyMap())

	if !m.inputMode {
		t.Fatal("inputInitial=true should set inputMode=true")
	}
	if !m.textArea.Focused() {
		t.Fatal("inputInitial=true should focus textarea")
	}
}

// TestModelInputModeEntersCorrectly verifies the comma-key enter path.
func TestModelInputModeEntersCorrectly(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m := r.(Model)
	if !m.inputMode {
		t.Fatal("comma should enter input mode")
	}
	if !m.textArea.Focused() {
		t.Fatal("textarea should be focused after entering input mode")
	}
}

// TestInputPanelWidthConsistency verifies the width calculation is correct.
func TestInputPanelWidthConsistency(t *testing.T) {
	service := &core.Service{}
	m := New(core.AppState{}, service, "", "auto", "auto", false, false, DefaultKeyMap())

	for _, termW := range []int{40, 60, 80, 120} {
		m.terminalWidth = termW
		m.inputMode = true

		outerW := m.inputPanelOuterWidth()
		panelW := m.inputPanelContentWidth()
		// outerW = termW - InputPanelStyle.GetHorizontalMargins()
		// panelW = outerW - InputPanelStyle.GetHorizontalBorderSize()
		//              - InputPanelStyle.GetHorizontalPadding()
		// InputPanelStyle has RoundedBorder, no padding/margins → frame size = 2
		expected := termW - 2
		if outerW != termW {
			t.Errorf("termW=%d: inputPanelOuterWidth()=%d, want %d", termW, outerW, termW)
			continue
		}
		if panelW != expected {
			t.Errorf("termW=%d: inputPanelContentWidth()=%d, want %d", termW, panelW, expected)
			continue
		}

		// Set width the same way handleWindowSize/enterInputMode does.
		m.textArea.SetWidth(panelW)
		contentW := m.textArea.Width()
		// contentW = panelW - promptWidth(2) = panelW - 2
		if contentW != panelW-2 {
			t.Errorf("termW=%d: textarea.Width()=%d, want %d (panelW=%d - 2)",
				termW, contentW, panelW-2, panelW)
		}
	}
}

// TestInputPanelPreservesTextareaVisualRows verifies the full composition
// contract: the panel's content box must be exactly as wide as textarea.View.
// If it is narrower, Lip Gloss wraps textarea's already-wrapped rows again.
func TestInputPanelPreservesTextareaVisualRows(t *testing.T) {
	inputs := []struct {
		name  string
		value string
	}{
		{"ascii", strings.Repeat("1234", 40)},
		{"cjk", strings.Repeat("这是中文软换行测试", 12)},
		{"emoji", strings.Repeat("😀", 80)},
		{"mixed", strings.Repeat("你好😀世界🌏测试🚀", 10)},
	}

	for _, input := range inputs {
		for _, terminalWidth := range []int{20, 30, 40, 60, 76, 80} {
			t.Run(input.name+"/width="+strconv.Itoa(terminalWidth), func(t *testing.T) {
				m := newDynamicTestModel(t)
				m.inputMode = true
				m.ready = true
				m = m.handleWindowSize(tea.WindowSizeMsg{Width: terminalWidth, Height: 40})

				// This deliberately spans several soft-wrapped rows at every width.
				m.textArea.SetValue(input.value)

				textareaRows := strings.Split(strings.TrimSuffix(m.textArea.View(), "\n"), "\n")
				if got, want := len(textareaRows), m.textArea.Height(); got != want {
					t.Fatalf("textarea rows = %d, want height %d", got, want)
				}
				for row, line := range textareaRows {
					if got, want := xansi.StringWidth(line), m.inputPanelContentWidth(); got != want {
						t.Fatalf("textarea row %d width = %d, want content box width %d", row, got, want)
					}
				}

				panelRows := strings.Split(m.renderInputPanel(), "\n")
				if got, want := len(panelRows), m.textArea.Height()+InputPanelStyle.GetVerticalFrameSize(); got != want {
					t.Fatalf("panel rows = %d, want textarea height + panel frame = %d", got, want)
				}
				for row, line := range panelRows {
					if got := xansi.StringWidth(line); got != terminalWidth {
						t.Fatalf("panel row %d width = %d, want terminal width %d", row, got, terminalWidth)
					}
				}
			})
		}
	}
}

// TestConfiguredTextareaOwnsSoftWrap verifies the production textarea setup
// before it is placed in any Lip Gloss container. It exercises grapheme-aware
// wrapping and records the textarea layout data used by the real cursor.
func TestConfiguredTextareaOwnsSoftWrap(t *testing.T) {
	inputs := []struct {
		name  string
		value string
	}{
		{"ascii", strings.Repeat("1234", 40)},
		{"cjk", strings.Repeat("这是中文软换行测试", 12)},
		{"emoji", strings.Repeat("😀", 30)},
		{"mixed", strings.Repeat("你好😀世界🌏测试🚀", 10)},
	}

	for _, input := range inputs {
		for _, width := range []int{20, 30, 40, 60, 76, 80} {
			t.Run(input.name+"/width="+strconv.Itoa(width), func(t *testing.T) {
				ta := newInputTextArea()
				ta.SetWidth(width)
				_ = ta.Focus()
				ta.SetValue(input.value)
				ta.MoveToEnd()

				rows := strings.Split(strings.TrimSuffix(ta.View(), "\n"), "\n")
				if got, want := len(rows), ta.Height(); got != want {
					t.Fatalf("textarea rows = %d, want height %d", got, want)
				}
				for row, line := range rows {
					if got := xansi.StringWidth(line); got != width {
						t.Fatalf("textarea row %d width = %d, want declared width %d", row, got, width)
					}
					prompt := "> "
					if row > 0 {
						prompt = "  "
					}
					if visible := xansi.Strip(line); !strings.HasPrefix(visible, prompt) {
						t.Fatalf("textarea row %d = %q, want prompt %q", row, visible, prompt)
					}
				}

				lineInfo := ta.LineInfo()
				if got, want := lineInfo.Height, ta.Height(); got != want {
					t.Fatalf("LineInfo.Height = %d, want textarea height %d", got, want)
				}
				if cursor := ta.Cursor(); cursor == nil {
					t.Fatal("focused textarea with a real cursor must return Cursor")
				}
				t.Logf("width=%d height=%d line=%d column=%d lineInfo=%+v", width, ta.Height(), ta.Line(), ta.Column(), lineInfo)
			})
		}
	}
}
