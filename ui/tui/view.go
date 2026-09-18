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
	sections = append(sections, m.renderStatusBar())

	if m.Error != "" {
		sections = append(sections, m.renderError())
	}

	if m.Loading {
		sections = append(sections, m.renderLoading())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	title := HeaderStyle.Render("my-trans")
	recordCount := StatusBarStyle.Render(fmt.Sprintf("Records: %d", len(m.Records)))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, " ", recordCount)
}

func (m Model) renderStatusBar() string {
	recordCount := fmt.Sprintf("Records: %d", len(m.Records))
	scrollPos := fmt.Sprintf("Scroll: %d (%.0f%%)", m.viewport.YOffset, m.viewport.ScrollPercent()*100)
	return StatusBarStyle.Render(fmt.Sprintf("%s | %s | q: quit", recordCount, scrollPos))
}

func (m Model) renderError() string {
	return ErrorStyle.Render(fmt.Sprintf("Error: %s", m.Error))
}

func (m Model) renderLoading() string {
	return LoadingStyle.Render("Translating...")
}
