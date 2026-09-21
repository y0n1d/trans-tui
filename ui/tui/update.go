package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/core"
)

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.terminalWidth = msg.Width
	m.terminalHeight = msg.Height

	// Always update textarea width when in input mode — even on the first
	// WindowSizeMsg. SetWidth MUST be called after SetPromptFunc (which
	// happens in New()), so it is safe here. Skipping it leaves the
	// textarea at the default width from New() and causes wrong wrapping.
	if m.inputMode {
		m.textArea.SetWidth(m.inputPanelWidth())
	}

	if !m.ready {
		m.viewport = newViewport(msg.Width, msg.Height-m.staticHeight())
		m.ready = true
		return m
	}
	m.viewport.SetWidth(msg.Width)
	m.viewport.SetHeight(msg.Height - m.staticHeight())
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
// error panel. The selection has already been cleared by the release handler,
// so this never leaves a broken selection or viewport behind.
func (m Model) handleClipboardError(msg clipboardErrorMsg) (Model, tea.Cmd) {
	m.Error = fmt.Sprintf("clipboard copy failed: %v", msg.err)
	m = m.recalcViewportHeight()
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	return m, nil
}

func (m Model) enterInputMode() (Model, tea.Cmd) {
	if m.inputMode {
		return m, nil
	}
	m.inputMode = true
	m.textArea.Reset()
	m.textArea.SetValue("")
	m.textArea.SetHeight(1) // reset to minimum for fresh input
	m.textArea.SetWidth(m.inputPanelWidth())
	m.textArea.Focus()
	m = m.recalcViewportHeight()
	return m, textarea.Blink
}

func (m Model) exitInputMode() Model {
	m.inputMode = false
	m.textArea.Blur()
	m.textArea.SetValue("")
	m.textArea.SetHeight(1) // reset for next entry
	m = m.recalcViewportHeight()
	return m
}

func (m Model) handleInputKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
