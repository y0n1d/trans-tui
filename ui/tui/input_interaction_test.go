package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func newSizedInputModel(t *testing.T, width, height int) Model {
	t.Helper()
	m := newDynamicTestModel(t)
	m.inputMode = true
	m = m.handleWindowSize(tea.WindowSizeMsg{Width: width, Height: height})
	m = m.recalcViewportHeight()
	return m
}

func updateInputModel(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func dragInputSelection(t *testing.T, m Model, startX, startY, endX, endY int) Model {
	t.Helper()
	m = updateInputModel(t, m, tea.MouseClickMsg{Button: tea.MouseLeft, X: startX, Y: startY})
	m = updateInputModel(t, m, tea.MouseMotionMsg{Button: tea.MouseLeft, X: endX, Y: endY})
	return updateInputModel(t, m, tea.MouseReleaseMsg{Button: tea.MouseLeft, X: endX, Y: endY})
}

func inputTextCell(m Model, localX, localY int) (int, int) {
	x, y := m.inputTextAreaOrigin()
	return x + localX, y + localY
}

func TestInputDynamicHeightStopsAtLayoutCap(t *testing.T) {
	m := newSizedInputModel(t, 40, 12)
	maxHeight := m.maxInputTextAreaHeight()
	if maxHeight != 7 {
		t.Fatalf("max input height = %d, want 7 for the 40x12 layout", maxHeight)
	}

	m.textArea.SetValue(strings.Repeat("x", 800))
	if got := m.textArea.Height(); got != maxHeight {
		t.Fatalf("textarea height = %d, want cap %d", got, maxHeight)
	}
	if got := m.historyViewportHeight(); got != minHistoryViewportHeight {
		t.Fatalf("history height = %d, want reserved minimum %d", got, minHistoryViewportHeight)
	}

	// The visible cap must not become a content cap. A further keypress still
	// reaches the textarea while its height stays fixed.
	m = updateInputModel(t, m, tea.KeyPressMsg{Text: "z", Code: 'z'})
	if !strings.HasSuffix(m.textArea.Value(), "z") {
		t.Fatalf("input at visual cap rejected extra text: %q", m.textArea.Value())
	}
	if got := m.textArea.Height(); got != maxHeight {
		t.Fatalf("textarea height after extra input = %d, want cap %d", got, maxHeight)
	}
}

func TestInputViewportKeepsFinalCursorRowVisible(t *testing.T) {
	m := newSizedInputModel(t, 40, 12)
	m = updateInputModel(t, m, tea.KeyPressMsg{
		Code: 'x',
		Text: strings.Repeat("x", 800) + "final-input-marker",
	})

	if got, want := m.textArea.Height(), m.maxInputTextAreaHeight(); got != want {
		t.Fatalf("textarea height = %d, want cap %d", got, want)
	}
	cursor := m.textArea.Cursor()
	if cursor == nil {
		t.Fatal("focused textarea must expose a real cursor")
	}
	if cursor.Position.Y < 0 || cursor.Position.Y >= m.textArea.Height() {
		t.Fatalf("cursor relative y = %d outside visible textarea height %d", cursor.Position.Y, m.textArea.Height())
	}
	if !strings.Contains(m.textArea.View(), "final-input-marker") {
		t.Fatal("textarea viewport does not render the final cursor row")
	}
}

func TestInputHeightCapRecalculatesOnResize(t *testing.T) {
	m := newSizedInputModel(t, 40, 12)
	m.textArea.SetValue(strings.Join(make([]string, 24), "\n"))
	if got, want := m.textArea.Height(), 7; got != want {
		t.Fatalf("initial textarea height = %d, want %d", got, want)
	}

	m = m.handleWindowSize(tea.WindowSizeMsg{Width: 40, Height: 16})
	m = m.recalcViewportHeight()
	if got, want := m.maxInputTextAreaHeight(), 11; got != want {
		t.Fatalf("resized max input height = %d, want %d", got, want)
	}
	if got, want := m.textArea.Height(), 11; got != want {
		t.Fatalf("resized textarea height = %d, want %d", got, want)
	}
	if got, want := m.viewport.Height(), minHistoryViewportHeight; got != want {
		t.Fatalf("resized history height = %d, want %d", got, want)
	}
}

func TestInputHeightCapIsSafeInTinyTerminal(t *testing.T) {
	for _, height := range []int{0, 1, 2, 3, 4, 5} {
		t.Run("height="+string(rune('0'+height)), func(t *testing.T) {
			m := newSizedInputModel(t, 1, height)
			m.textArea.SetValue(strings.Repeat("x", 100))
			if got := m.maxInputTextAreaHeight(); got != minInputTextAreaHeight {
				t.Fatalf("max input height = %d, want minimum %d", got, minInputTextAreaHeight)
			}
			if got := m.textArea.Height(); got != minInputTextAreaHeight {
				t.Fatalf("textarea height = %d, want minimum %d", got, minInputTextAreaHeight)
			}
			if got := m.viewport.Height(); got < 0 {
				t.Fatalf("history height = %d, must not be negative", got)
			}
		})
	}
}

func TestInputModeWheelRoutesHistoryAndTextareaSeparately(t *testing.T) {
	m := newSizedInputModel(t, 40, 12)
	m.viewport.SetContent(strings.Repeat("history\n", 30))
	m = updateInputModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 4, Y: m.headerHeight()})
	if got := m.viewport.YOffset(); got == 0 {
		t.Fatal("wheel over history did not scroll history viewport")
	}

	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "input row"
	}
	m.textArea.SetValue(strings.Join(lines, "\n"))
	m.textArea.MoveToBegin()
	for range 4 {
		m.textArea.CursorDown()
	}
	inputX, inputY := inputTextCell(m, 2, 0)
	m = updateInputModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: inputX, Y: inputY})
	if got := m.textArea.ScrollYOffset(); got == 0 {
		t.Fatal("wheel over input did not scroll textarea viewport")
	}
	historyOffset := m.viewport.YOffset()
	m = updateInputModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: inputX, Y: inputY})
	if got := m.viewport.YOffset(); got != historyOffset {
		t.Fatalf("input wheel changed history offset: got %d, want %d", got, historyOffset)
	}
}

func TestInputMouseSelectionCopiesThroughProjectClipboard(t *testing.T) {
	clipboard := &clipboardRecorder{}
	m := newSizedInputModel(t, 40, 12)
	m.clipboard = clipboard.write
	m.textArea.SetValue("hello world")
	before := m.textArea.View()
	startX, startY := inputTextCell(m, 2, 0)
	endX, endY := inputTextCell(m, 7, 0)
	m = dragInputSelection(t, m, startX, startY, endX, endY)

	if !m.textArea.HasSelection() {
		t.Fatal("input drag did not retain textarea selection")
	}
	if got, want := m.textArea.SelectedText(), "hello"; got != want {
		t.Fatalf("selected text = %q, want %q", got, want)
	}
	if m.textArea.View() == before {
		t.Fatal("textarea view did not render selection highlight")
	}

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl | tea.ModShift})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Ctrl+Shift+C did not return project clipboard command")
	}
	if msg := cmd(); msg != nil {
		t.Fatalf("clipboard command returned unexpected message %T", msg)
	}
	if got := clipboard.writes; len(got) != 1 || got[0] != "hello" {
		t.Fatalf("clipboard writes = %q, want [hello]", got)
	}
	if !m.textArea.HasSelection() {
		t.Fatal("copy should preserve textarea selection")
	}
}

func TestInputCopyShortcutDoesNotClaimCtrlCOrCtrlV(t *testing.T) {
	clipboard := &clipboardRecorder{}
	m := newSizedInputModel(t, 40, 12)
	m.clipboard = clipboard.write
	m.textArea.SetValue("hello world")
	startX, startY := inputTextCell(m, 2, 0)
	endX, endY := inputTextCell(m, 7, 0)
	m = dragInputSelection(t, m, startX, startY, endX, endY)

	updated, ctrlCCmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = updated.(Model)
	if ctrlCCmd != nil {
		t.Fatal("plain Ctrl+C must not be treated as input copy")
	}
	if len(clipboard.writes) != 0 {
		t.Fatalf("plain Ctrl+C wrote clipboard data: %q", clipboard.writes)
	}
	if !m.textArea.HasSelection() {
		t.Fatal("plain Ctrl+C must not disturb textarea selection")
	}

	_, ctrlVCmd := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	if ctrlVCmd == nil {
		t.Fatal("Ctrl+V must remain delegated to textarea paste handling")
	}
}

func TestInputMouseSelectionUsesCJKAndGraphemeBoundaries(t *testing.T) {
	t.Run("CJK", func(t *testing.T) {
		m := newSizedInputModel(t, 40, 12)
		m.textArea.SetValue("你好世界")
		startX, startY := inputTextCell(m, 2, 0)
		endX, endY := inputTextCell(m, 6, 0)
		m = dragInputSelection(t, m, startX, startY, endX, endY)
		if got, want := m.textArea.SelectedText(), "你好"; got != want {
			t.Fatalf("CJK selected text = %q, want %q", got, want)
		}
	})

	t.Run("ZWJ emoji", func(t *testing.T) {
		m := newSizedInputModel(t, 40, 12)
		m.textArea.SetValue("👩‍💻x")
		startX, startY := inputTextCell(m, 2, 0)
		endX, endY := inputTextCell(m, 4, 0)
		m = dragInputSelection(t, m, startX, startY, endX, endY)
		if got, want := m.textArea.SelectedText(), "👩‍💻"; got != want {
			t.Fatalf("emoji selected text = %q, want full grapheme %q", got, want)
		}
	})
}

func TestInputMouseSelectionRespectsSoftWrapAndScrollOffset(t *testing.T) {
	t.Run("soft wrap", func(t *testing.T) {
		m := newSizedInputModel(t, 14, 12)
		m.textArea.SetValue("aaaaabbbbbccccc")
		startX, startY := inputTextCell(m, 2, 1)
		endX, endY := inputTextCell(m, 7, 1)
		m = dragInputSelection(t, m, startX, startY, endX, endY)
		if got, want := m.textArea.SelectedText(), "ccccc"; got != want {
			t.Fatalf("soft-wrapped selected text = %q, want %q", got, want)
		}
	})

	t.Run("scrolled textarea", func(t *testing.T) {
		m := newSizedInputModel(t, 40, 12)
		lines := make([]string, 30)
		for i := range lines {
			lines[i] = "line-00"
		}
		m.textArea.SetValue(strings.Join(lines, "\n"))
		m.textArea.MoveToBegin()
		for range 4 {
			m.textArea.CursorDown()
		}
		wheelX, wheelY := inputTextCell(m, 2, 0)
		m = updateInputModel(t, m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: wheelX, Y: wheelY})
		if got := m.textArea.ScrollYOffset(); got == 0 {
			t.Fatal("textarea did not scroll before selection")
		}

		startX, startY := inputTextCell(m, 2, 0)
		endX, endY := inputTextCell(m, 6, 0)
		m = dragInputSelection(t, m, startX, startY, endX, endY)
		if got, want := m.textArea.SelectedText(), "line"; got != want {
			t.Fatalf("scrolled selected text = %q, want %q", got, want)
		}
	})
}

func TestInputHeightCapStillRendersPanelAtTextareaHeight(t *testing.T) {
	m := newSizedInputModel(t, 40, 12)
	m.textArea.SetValue(strings.Repeat("x", 800))
	if got, want := lipgloss.Height(m.renderInputPanel()), m.textArea.Height()+InputPanelStyle.GetVerticalFrameSize(); got != want {
		t.Fatalf("rendered input panel height = %d, want %d", got, want)
	}
}
