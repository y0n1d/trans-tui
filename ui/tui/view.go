package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"my-trans/internal/core"
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

func (m Model) renderRecords() string {
	w := m.viewport.Width
	if w <= 0 {
		w = 80
	}

	if len(m.Records) == 0 {
		return StatusBarStyle.Render("No translations yet. Type text to translate.")
	}

	var records []string
	for _, record := range m.Records {
		records = append(records, m.renderRecord(record, w))
	}
	return lipgloss.JoinVertical(lipgloss.Left, records...)
}

func (m Model) renderRecord(record core.TranslationRecord, viewportWidth int) string {
	contentWidth := viewportWidth - recordBorderPadding
	if contentWidth < 1 {
		contentWidth = 1
	}

	style := RecordStyle.Width(contentWidth)

	var parts []string

	source := SourceStyle.Render(fmt.Sprintf("[%s] %s", record.SourceLang, record.Source))
	parts = append(parts, source)

	if record.Error != "" {
		errorText := ErrorStyle.Render(fmt.Sprintf("Error: %s", record.Error))
		parts = append(parts, errorText)
	} else if record.Translation != "" {
		translation := TranslationStyle.Render(fmt.Sprintf("[%s] %s", record.TargetLang, record.Translation))
		parts = append(parts, translation)
	}

	if record.Provider != "" && record.Model != "" {
		provider := StatusBarStyle.Render(fmt.Sprintf("via %s/%s", record.Provider, record.Model))
		parts = append(parts, provider)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return style.Render(content)
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
