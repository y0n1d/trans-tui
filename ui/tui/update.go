package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/core"
)

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.terminalWidth = msg.Width
	m.terminalHeight = msg.Height

	// Always update the input layout on resize — including the first resize.
	// syncInputLayout sets the width only after the prompt is configured and
	// recalculates the DynamicHeight cap from the new terminal height.
	if m.inputMode {
		m = m.syncInputLayout()
	}

	if !m.ready {
		m.viewport = newViewport(msg.Width, m.historyViewportHeight())
		m.ready = true
		return m
	}
	m.viewport.SetWidth(msg.Width)
	m.viewport.SetHeight(m.historyViewportHeight())
	return m
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// Esc has dual behavior: dismiss error if visible, otherwise quit.
	if msg.String() == "esc" {
		if m.Error != "" {
			return m.handleDismissError()
		}
		if m.keyMap.Matches(msg, m.keyMap.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}

	if m.keyMap.Matches(msg, m.keyMap.Quit) {
		return m, tea.Quit
	}
	if m.keyMap.Matches(msg, m.keyMap.InputMode) {
		return m.enterInputMode()
	}

	switch msg.String() {
	case "up", "k":
		m.viewport.ScrollUp(1)
	case "down", "j":
		m.viewport.ScrollDown(1)
	case "pgup", "b":
		m.viewport.HalfPageUp()
	case "pgdown", "f":
		m.viewport.HalfPageDown()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	case "r":
		return m.handleRetry()
	}
	return m, nil
}

func (m Model) handleDisplayText(msg core.DisplayTextMsg) (Model, tea.Cmd) {
	record := core.TranslationRecord{
		ID:         msg.RequestID,
		Source:     msg.Text,
		SourceLang: "OCR",
	}
	m.Records = append(m.Records, record)
	m.Loading = false
	m.Error = ""

	m = m.recalcViewportHeight()
	m.buildSemanticMap(m.Records, m.viewport.Width())
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	m.viewport.GotoBottom()
	return m, nil
}

func (m Model) handleTranslationResult(msg core.TranslationResultMsg) (Model, tea.Cmd) {
	record := core.TranslationRecord{
		ID:          msg.RequestID,
		Source:      msg.Source,
		Translation: msg.Translation,
		SourceLang:  msg.SourceLang,
		TargetLang:  msg.TargetLang,
		Provider:    msg.Provider,
		Model:       msg.Model,
	}
	m.Records = append(m.Records, record)
	m.Loading = false
	m.Error = ""
	m.LastFailed = nil

	m = m.recalcViewportHeight()
	m.buildSemanticMap(m.Records, m.viewport.Width())
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	m.viewport.GotoBottom()
	return m, nil
}

func (m Model) handleTranslationError(msg core.TranslationErrorMsg) (Model, tea.Cmd) {
	m.Loading = false
	m.Error = msg.Error
	m.LastFailed = &core.TranslationRecord{
		ID:         msg.RequestID,
		Source:     msg.Source,
		Error:      msg.Error,
		SourceLang: msg.SourceLang,
		TargetLang: msg.TargetLang,
	}

	m = m.recalcViewportHeight()
	m.buildSemanticMap(m.Records, m.viewport.Width())
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	return m, nil
}

func (m Model) handleRetry() (Model, tea.Cmd) {
	if m.LastFailed == nil {
		return m, nil
	}
	m.Loading = true
	m.Error = ""
	m = m.recalcViewportHeight()
	last := *m.LastFailed
	return m, m.translateText(last.Source, last.SourceLang, last.TargetLang)
}

func (m Model) handleDismissError() (Model, tea.Cmd) {
	if m.Error == "" {
		return m, nil
	}
	m.Error = ""
	m = m.recalcViewportHeight()
	return m, nil
}

// handleClipboardError surfaces a failed clipboard write through the existing
// error panel. The selection remains owned by its component: history clears on
// release, while an input textarea keeps its Bubbles selection after copying.
func (m Model) handleClipboardError(msg clipboardErrorMsg) (Model, tea.Cmd) {
	m.Error = fmt.Sprintf("clipboard copy failed: %v", msg.err)
	m = m.syncInputLayout()
	m = m.recalcViewportHeight()
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	return m, nil
}

func (m Model) enterInputMode() (Model, tea.Cmd) {
	if m.inputMode {
		return m, nil
	}
	m.inputMode = true
	m.inputSelecting = false
	m.textArea.Reset()
	m.textArea.SetValue("")
	m.textArea.SetHeight(minInputTextAreaHeight)
	m = m.syncInputLayout()
	m.textArea.Focus()
	m = m.recalcViewportHeight()
	return m, textarea.Blink
}

func (m Model) exitInputMode() Model {
	m.inputMode = false
	m.inputSelecting = false
	m.textArea.Blur()
	m.textArea.SetValue("")
	m.textArea.SetHeight(minInputTextAreaHeight) // reset for next entry
	m = m.recalcViewportHeight()
	return m
}

func (m Model) handleInputKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// textarea.CopySelection writes through atotto/clipboard. Intercept its
	// documented shortcut and use the application's OSC52-backed abstraction
	// instead, while retaining textarea's own selection state and rendering.
	if key.Matches(msg, m.textArea.KeyMap.CopySelection) {
		if !m.textArea.HasSelection() {
			return m, nil
		}
		return m, m.copyToClipboard(m.textArea.SelectedText())
	}

	switch msg.String() {
	case "esc":
		return m.exitInputMode(), nil
	case "enter":
		text := strings.TrimSpace(m.textArea.Value())
		if text == "" {
			return m, nil
		}
		m.inputMode = false
		m.textArea.Blur()
		m.textArea.SetValue("")
		m.Loading = true
		m = m.recalcViewportHeight()
		cmd := m.translateText(text, m.sourceLang, m.targetLang)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.textArea, cmd = m.textArea.Update(msg)
		// DynamicHeight may have changed textarea.Height() — recalculate
		// viewport height so the total TUI fits the terminal.
		m = m.recalcViewportHeight()
		return m, cmd
	}
}

// handleInputMouse routes only the input-mode interactions that have a clear
// owner. History wheel events continue to drive the history viewport; textarea
// wheel events are delegated to Bubbles' own private textarea viewport. Click
// and drag selection remain entirely inside the textarea.
func (m Model) handleInputMouse(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		if m.mouseInInputTextArea(mouse) {
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(msg)
			return m, cmd
		}
		if m.mouseInHistoryViewport(mouse) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft || !m.mouseInInputTextArea(mouse) {
			return m, nil
		}
		x, y := m.inputTextAreaSelectionCoordinates(mouse)
		m.textArea.BeginSelection(x, y)
		m.inputSelecting = true
		return m, nil

	case tea.MouseMotionMsg:
		if !m.inputSelecting {
			return m, nil
		}
		x, y := m.inputTextAreaSelectionCoordinates(msg.Mouse())
		m.textArea.ExtendSelection(x, y)
		return m, nil

	case tea.MouseReleaseMsg:
		mouse := msg.Mouse()
		if mouse.Button != tea.MouseLeft || !m.inputSelecting {
			return m, nil
		}
		x, y := m.inputTextAreaSelectionCoordinates(mouse)
		m.textArea.ExtendSelection(x, y)
		m.textArea.EndSelection()
		m.inputSelecting = false
		return m, nil
	}
	return m, nil
}

// inputTextAreaSelectionCoordinates maps terminal cells to the coordinates
// expected by textarea.BeginSelection and ExtendSelection. PositionAt owns
// soft-wrap and viewport-offset handling. The small grapheme-boundary adapter
// only advances an endpoint when Bubbles' rune-index mapping lands inside a
// multi-rune grapheme (for example a ZWJ emoji); it never computes byte offsets
// or reimplements wrapping.
func (m Model) inputTextAreaSelectionCoordinates(mouse tea.Mouse) (x, y int) {
	x, y = m.inputTextAreaMousePosition(mouse)
	pos := m.textArea.PositionAt(x, y)
	lines := strings.Split(m.textArea.Value(), "\n")
	if pos.Row < 0 || pos.Row >= len(lines) {
		return x, y
	}

	end := graphemeEndContaining([]rune(lines[pos.Row]), pos.Col)
	if end == pos.Col {
		return x, y
	}

	// PositionAt clamps a far-right x to the end of this visual row, so once
	// the target boundary is in the row, monotonically increasing x reaches it.
	// Bound the probe to the visible textarea width: a pathological wrapped
	// segment must not turn a malformed terminal coordinate into an unbounded
	// loop.
	for adjustedX := x; adjustedX <= m.inputPanelContentWidth(); adjustedX++ {
		adjusted := m.textArea.PositionAt(adjustedX, y)
		if adjusted.Row != pos.Row || adjusted.Col >= end {
			return adjustedX, y
		}
	}
	return x, y
}

// graphemeEndContaining returns column unchanged when it is already a
// grapheme boundary, otherwise it returns the end of the containing cluster.
func graphemeEndContaining(line []rune, column int) int {
	if column <= 0 || column >= len(line) {
		return column
	}
	for start := 0; start < len(line); {
		runeCount, _ := firstGraphemeWidth(line[start:])
		if runeCount < 1 {
			runeCount = 1
		}
		end := start + runeCount
		if column == start || column == end {
			return column
		}
		if column > start && column < end {
			return end
		}
		start = end
	}
	return column
}
