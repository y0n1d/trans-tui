package tui

import "github.com/charmbracelet/lipgloss"

var (
	// RecordStyle is the style for a translation record.
	RecordStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	// SourceStyle is the style for the source text.
	SourceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Bold(true)

	// TranslationStyle is the style for the translation text.
	TranslationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86"))

	// ErrorStyle is the style for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// StatusBarStyle is the style for the status bar.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	// HeaderStyle is the style for the header.
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Bold(true).
			Padding(0, 1)

	// LoadingStyle is the style for loading indicator.
	LoadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
)