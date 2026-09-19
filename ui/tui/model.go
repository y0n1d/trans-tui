package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/y0n1d/trans-tui/internal/core"
	"github.com/y0n1d/trans-tui/internal/translator"
)

type InitialTranslationMsg = core.InitialTranslationMsg
type EnterInputModeMsg = core.EnterInputModeMsg

type Model struct {
	core.AppState
	viewport       viewport.Model
	ready          bool
	err            error
	service        *core.Service
	lastText       string
	sourceLang     string
	targetLang     string
	terminalWidth  int
	terminalHeight int
	sel            selection
	semLines       []semanticLine
	semRows        []semanticRow
	clipboard      clipboardWrite
	inputMode      bool
	textInput      textinput.Model
	inputInitial   bool
}

func New(initial core.AppState, service *core.Service, text, sourceLang, targetLang string, inputInitial bool) Model {
	ti := textinput.New()
	ti.Placeholder = "Type text to translate..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 60

	m := Model{
		AppState:     initial,
		service:      service,
		lastText:     text,
		sourceLang:   sourceLang,
		targetLang:   targetLang,
		clipboard:    osc52ClipboardWrite,
		textInput:    ti,
		inputInitial: inputInitial,
	}

	if inputInitial {
		m.inputMode = true
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m = m.handleWindowSize(msg)
		m.buildSemanticMap(m.Records, m.viewport.Width)
		m.viewport.SetContent(m.renderRecordsWithHighlight())
		return m, nil

	case tea.KeyMsg:
		if m.inputMode {
			return m.handleInputKeyMsg(msg)
		}
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		if m.inputMode {
			return m, nil
		}
		return m.handleMouse(msg)

	case EnterInputModeMsg:
		return m.enterInputMode()

	case InitialTranslationMsg:
		return m.handleInitialTranslation()

	case core.TranslationResultMsg:
		return m.handleTranslationResult(msg)

	case core.TranslationErrorMsg:
		return m.handleTranslationError(msg)

	case clipboardErrorMsg:
		return m.handleClipboardError(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.renderView()
}

func (m Model) handleInitialTranslation() (Model, tea.Cmd) {
	if m.lastText == "" {
		return m, nil
	}
	m.Loading = true
	m = m.recalcViewportHeight()
	text := m.lastText
	srcLang := m.sourceLang
	tgtLang := m.targetLang
	return m, func() tea.Msg {
		ctx := context.Background()
		result, err := m.service.Translate(ctx, translator.TranslationRequest{
			Text:       text,
			SourceLang: srcLang,
			TargetLang: tgtLang,
		})
		if err != nil {
			return core.TranslationErrorMsg{
				Source:     text,
				Error:      err.Error(),
				SourceLang: srcLang,
				TargetLang: tgtLang,
			}
		}
		return core.TranslationResultMsg{
			Source:      text,
			Translation: result.Translation,
			SourceLang:  srcLang,
			TargetLang:  result.TargetLang,
			Provider:    result.Provider,
			Model:       result.Model,
		}
	}
}

func newViewport(width, height int) viewport.Model {
	return viewport.New(width, height)
}

func (m Model) staticHeight() int {
	h := 2 // header + status bar
	if m.inputMode {
		h += m.inputPanelHeight()
	}
	if m.Loading {
		h++
	}
	if m.Error != "" {
		h += lipgloss.Height(m.renderErrorPanel(m.terminalWidth))
	}
	return h
}

func (m Model) inputPanelHeight() int {
	if !m.inputMode {
		return 0
	}
	return 3 // top border + input line + bottom border
}

// recalcViewportHeight keeps the total TUI height exactly equal to the
// terminal height whenever the error panel or loading indicator changes.
func (m Model) recalcViewportHeight() Model {
	if m.ready {
		h := m.terminalHeight - m.staticHeight()
		if h < 0 {
			h = 0
		}
		m.viewport.Height = h
	}
	return m
}

func (m Model) headerHeight() int {
	return 1
}

func (m Model) translateText(text, srcLang, tgtLang string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.service.Translate(ctx, translator.TranslationRequest{
			Text:       text,
			SourceLang: srcLang,
			TargetLang: tgtLang,
		})
		if err != nil {
			return core.TranslationErrorMsg{
				Source:     text,
				Error:      err.Error(),
				SourceLang: srcLang,
				TargetLang: tgtLang,
			}
		}
		return core.TranslationResultMsg{
			Source:      text,
			Translation: result.Translation,
			SourceLang:  srcLang,
			TargetLang:  result.TargetLang,
			Provider:    result.Provider,
			Model:       result.Model,
			RequestID:   time.Now().Format("20060102150405.000000000"),
		}
	}
}
