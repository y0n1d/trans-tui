package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const recordBorderPadding = 2

func (m Model) renderView() string {
	if !m.ready {
		return "Initializing..."
	}

	var sections []string

	sections = append(sections, m.renderHeader())
	sections = append(sections, m.viewport.View())

	if m.inputMode {
		sections = append(sections, m.renderInputPanel())
	}

	if m.Error != "" {
		sections = append(sections, m.renderErrorPanel(m.terminalWidth))
	}

	sections = append(sections, m.renderStatusBar())

	if m.Loading {
		sections = append(sections, m.renderLoading())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	title := HeaderStyle.Render("trans-tui")
	recordCount := StatusBarStyle.Render(fmt.Sprintf("Records: %d", len(m.Records)))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, " ", recordCount)
}

func (m Model) renderStatusBar() string {
	recordCount := fmt.Sprintf("Records: %d", len(m.Records))
	scrollPos := fmt.Sprintf("Scroll: %d (%.0f%%)", m.viewport.YOffset, m.viewport.ScrollPercent()*100)
	if m.inputMode {
		return StatusBarStyle.Render(fmt.Sprintf("%s | %s | esc: cancel", recordCount, scrollPos))
	}
	inputHint := firstKey(m.keyMap.InputMode) + ": input"
	quitHint := firstKey(m.keyMap.Quit) + ": quit"
	return StatusBarStyle.Render(fmt.Sprintf("%s | %s | %s | %s", recordCount, scrollPos, inputHint, quitHint))
}

func (m Model) renderErrorPanel(width int) string {
	if m.Error == "" {
		return ""
	}
	content := fmt.Sprintf("\u26a0 %s", m.Error)
	contentWidth := width - recordBorderPadding
	if contentWidth < 1 {
		contentWidth = 1
	}
	return ErrorPanelStyle.Width(contentWidth).Render(content)
}

func (m Model) renderLoading() string {
	return LoadingStyle.Render("Translating...")
}

func (m Model) renderInputPanel() string {
	prompt := InputPromptStyle.Render("> ")
	input := m.textInput.View()
	contentWidth := m.terminalWidth - recordBorderPadding
	if contentWidth < 1 {
		contentWidth = 1
	}
	content := prompt + input
	return InputPanelStyle.Width(contentWidth).Render(content)
}
