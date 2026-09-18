package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"my-trans/internal/core"
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

	m.viewport.Height = m.terminalHeight - m.staticHeight()
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

	m.viewport.Height = m.terminalHeight - m.staticHeight()
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	return m, nil
}

func (m Model) handleRetry() (Model, tea.Cmd) {
	if m.LastFailed == nil {
		return m, nil
	}
	m.Loading = true
	m.Error = ""
	last := *m.LastFailed
	return m, m.translateText(last.Source, last.SourceLang, last.TargetLang)
}

func (m Model) handleDismissError() (Model, tea.Cmd) {
	if m.Error == "" {
		return m, nil
	}
	m.Error = ""
	m.viewport.Height = m.terminalHeight - m.staticHeight()
	return m, nil
}
