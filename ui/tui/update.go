package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/y0n1d/trans-tui/internal/core"
)

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.terminalWidth = msg.Width
	m.terminalHeight = msg.Height
	if !m.ready {
		m.viewport = newViewport(msg.Width, msg.Height-m.staticHeight())
		m.ready = true
		return m
	}
	m.viewport.Width = msg.Width
	m.viewport.Height = msg.Height - m.staticHeight()
	inputWidth := msg.Width - recordBorderPadding - 4
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.textInput.Width = inputWidth
	return m
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		return m.handleDismissError()
	case "up", "k":
		m.viewport.LineUp(1)
	case "down", "j":
		m.viewport.LineDown(1)
	case "pgup", "b":
		m.viewport.HalfViewUp()
	case "pgdown", "f":
		m.viewport.HalfViewDown()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	case "r":
		return m.handleRetry()
	case "i":
		return m.enterInputMode()
	}
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
	m.buildSemanticMap(m.Records, m.viewport.Width)
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
	m.textInput.Reset()
	m.textInput.SetValue("")
	m.textInput.Focus()
	m = m.recalcViewportHeight()
	return m, textinput.Blink
}

func (m Model) exitInputMode() Model {
	m.inputMode = false
	m.textInput.Blur()
	m.textInput.SetValue("")
	m = m.recalcViewportHeight()
	return m
}

func (m Model) handleInputKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.exitInputMode(), nil
	case "enter":
		text := strings.TrimSpace(m.textInput.Value())
		if text == "" {
			return m, nil
		}
		m.inputMode = false
		m.textInput.Blur()
		m.textInput.SetValue("")
		m.Loading = true
		m = m.recalcViewportHeight()
		cmd := m.translateText(text, m.sourceLang, m.targetLang)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}
