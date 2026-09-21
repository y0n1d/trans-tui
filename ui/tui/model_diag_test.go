package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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

	// After handleWindowSize: inputPanelWidth() = 80 - 2 = 78
	// textarea.SetWidth(78) → content width = 78 - 2(prompt) = 76
	panelW := m.inputPanelWidth()
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

		panelW := m.inputPanelWidth()
		// panelW = termW - InputPanelStyle.GetHorizontalFrameSize()
		// InputPanelStyle has RoundedBorder, no padding/margins → frame size = 2
		expected := termW - 2
		if panelW != expected {
			t.Errorf("termW=%d: inputPanelWidth()=%d, want %d", termW, panelW, expected)
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
