package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	textArea       textarea.Model
	inputInitial   bool
	displayMode    bool
	keyMap         KeyMap
}

func New(initial core.AppState, service *core.Service, text, sourceLang, targetLang string, inputInitial, displayMode bool, km KeyMap) Model {
	ta := textarea.New()
	ta.Placeholder = "Type text to translate..."
	ta.ShowLineNumbers = false
	ta.SetVirtualCursor(false) // Use real terminal cursor
	ta.SetWidth(60)

	// Dynamic height: textarea grows/shrinks with content.
	// MinHeight=1 ensures empty input shows exactly 1 line.
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.SetHeight(1) // initial height for empty input

	// Prompt: only the first visual line gets "> ".
	// Continuation lines (soft-wrapped or subsequent logical lines) get "  ".
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "> "
		}
		return "  "
	})

	m := Model{
		AppState:     initial,
		service:      service,
		lastText:     text,
		sourceLang:   sourceLang,
		targetLang:   targetLang,
		clipboard:    osc52ClipboardWrite,
		textArea:     ta,
		inputInitial: inputInitial,
		displayMode:  displayMode,
		keyMap:       km,
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
		m = m.recalcViewportHeight() // re-check after textarea resize may change height
		m.buildSemanticMap(m.Records, m.viewport.Width())
		m.viewport.SetContent(m.renderRecordsWithHighlight())
		return m, nil

	case tea.KeyPressMsg:
		if m.inputMode {
			return m.handleInputKeyPress(msg)
		}
		return m.handleKeyPress(msg)

	case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg, tea.MouseWheelMsg:
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

	case core.DisplayTextMsg:
		return m.handleDisplayText(msg)

	case clipboardErrorMsg:
		return m.handleClipboardError(msg)
	}

	return m, nil
}

func (m Model) View() tea.View {
	v := tea.NewView(m.renderView())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	// Set real cursor position from textarea when in input mode.
	// textarea.Cursor() returns coordinates relative to the textarea viewport.
	// We add the absolute screen position of the textarea:
	//   Y = header + history viewport + input panel border top
	//   X = input panel border left
	// Note: textarea.Cursor() already includes prompt width, textarea
	// internal padding/border — we do NOT add those again.
	if m.inputMode {
		c := m.textArea.Cursor()
		if c != nil {
			v.Cursor = c
			v.Cursor.Position.Y += m.headerHeight() + m.viewport.Height() +
				InputPanelStyle.GetBorderTopSize()
			v.Cursor.Position.X += InputPanelStyle.GetBorderLeftSize()
		}
	}

	return v
}

func (m Model) handleInitialTranslation() (Model, tea.Cmd) {
	if m.lastText == "" {
		return m, nil
	}
	if m.displayMode {
		record := core.TranslationRecord{
			ID:         "display-initial",
			Source:     m.lastText,
			SourceLang: "OCR",
		}
		m.Records = append(m.Records, record)
		m = m.recalcViewportHeight()
		m.buildSemanticMap(m.Records, m.viewport.Width())
		m.viewport.SetContent(m.renderRecordsWithHighlight())
		m.viewport.GotoBottom()
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
	return viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
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
	// textarea lines + top border + bottom border
	return m.textArea.Height() + 2
}

// recalcViewportHeight keeps the total TUI height exactly equal to the
// terminal height whenever the error panel or loading indicator changes.
func (m Model) recalcViewportHeight() Model {
	if m.ready {
		h := m.terminalHeight - m.staticHeight()
		if h < 0 {
			h = 0
		}
		m.viewport.SetHeight(h)
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
