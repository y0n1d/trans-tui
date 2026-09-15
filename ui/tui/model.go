package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"my-trans/internal/core"
)

type Model struct {
	core.AppState
	viewport viewport.Model
	ready    bool
	err      error
}

func New(initial core.AppState) Model {
	return Model{AppState: initial}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m = m.handleWindowSize(msg)
		m.viewport.SetContent(m.renderRecords())
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case core.TranslationResultMsg:
		return m.handleTranslationResult(msg)

	case core.TranslationErrorMsg:
		return m.handleTranslationError(msg)
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.renderView()
}

func newViewport(width, height int) viewport.Model {
	return viewport.New(width, height)
}
