package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/yoned/my-trans-tui/internal/core"
)

// renderView renders the entire TUI view.
func (m Model) renderView() string {
	if !m.ready {
		return "Initializing..."
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content area (viewport)
	sections = append(sections, m.viewport.View())

	// Status bar
	sections = append(sections, m.renderStatusBar())

	// Error display (if any)
	if m.Error != "" {
		sections = append(sections, m.renderError())
	}

	// Loading indicator
	if m.Loading {
		sections = append(sections, m.renderLoading())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the header section.
func (m Model) renderHeader() string {
	title := HeaderStyle.Render("my-trans")
	recordCount := StatusBarStyle.Render(fmt.Sprintf("Records: %d", len(m.Records)))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, " ", recordCount)
}

// renderRecords renders all translation records as a single string for the viewport.
func (m Model) renderRecords() string {
	if len(m.Records) == 0 {
		return StatusBarStyle.Render("No translations yet. Type text to translate.")
	}

	var records []string
	for _, record := range m.Records {
		records = append(records, m.renderRecord(record))
	}
	return lipgloss.JoinVertical(lipgloss.Left, records...)
}

// renderRecord renders a single translation record.
func (m Model) renderRecord(record core.TranslationRecord) string {
	var parts []string

	// Source text
	source := SourceStyle.Render(fmt.Sprintf("[%s] %s", record.SourceLang, record.Source))
	parts = append(parts, source)

	// Translation or error
	if record.Error != "" {
		errorText := ErrorStyle.Render(fmt.Sprintf("Error: %s", record.Error))
		parts = append(parts, errorText)
	} else if record.Translation != "" {
		translation := TranslationStyle.Render(fmt.Sprintf("[%s] %s", record.TargetLang, record.Translation))
		parts = append(parts, translation)
	}

	// Provider/model info (if available)
	if record.Provider != "" && record.Model != "" {
		provider := StatusBarStyle.Render(fmt.Sprintf("via %s/%s", record.Provider, record.Model))
		parts = append(parts, provider)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return RecordStyle.Render(content)
}

// renderStatusBar renders the status bar at the bottom.
func (m Model) renderStatusBar() string {
	recordCount := fmt.Sprintf("Records: %d", len(m.Records))
	scrollPos := fmt.Sprintf("Scroll: %d (%.0f%%)", m.viewport.YOffset, m.viewport.ScrollPercent()*100)
	return StatusBarStyle.Render(fmt.Sprintf("%s | %s | q: quit", recordCount, scrollPos))
}

// renderError renders the error display.
func (m Model) renderError() string {
	return ErrorStyle.Render(fmt.Sprintf("Error: %s", m.Error))
}

// renderLoading renders the loading indicator.
func (m Model) renderLoading() string {
	return LoadingStyle.Render("Translating...")
}