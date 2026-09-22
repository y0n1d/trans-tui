package tui

import (
	"context"
	"math"
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
	inputSelecting bool
	inputInitial   bool
	displayMode    bool
	keyMap         KeyMap
}

func New(initial core.AppState, service *core.Service, text, sourceLang, targetLang string, inputInitial, displayMode bool, km KeyMap) Model {
	ta := newInputTextArea()

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
		// Must focus the textarea so textarea.Update() processes keys.
		// Without this, textarea.Update() returns early on every key.
		m.textArea.Focus()
	}

	return m
}

// newInputTextArea builds the single input component used by the TUI. Its
// width is replaced with the real panel content width on WindowSizeMsg.
func newInputTextArea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type text to translate..."
	ta.ShowLineNumbers = false
	ta.SetVirtualCursor(false) // Use the real terminal cursor.

	// DynamicHeight grows and shrinks with the textarea's own soft-wrapped
	// visual rows. MinHeight=1 keeps an empty input to one row.
	ta.DynamicHeight = true
	ta.MinHeight = 1
	// MaxHeight limits only the visible textarea viewport. Set an explicit,
	// effectively unbounded content limit so Bubbles does not apply its legacy
	// MaxHeight-as-logical-line-limit behavior when the visible viewport fills.
	ta.MaxContentHeight = math.MaxInt
	ta.SetHeight(1)

	// PromptInfo.LineNumber is the visual display-row index: textarea.View
	// increments it for every soft-wrapped segment. Reserve two cells for both
	// the first prompt and the continuation indentation before setting width.
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "> "
		}
		return "  "
	})
	ta.SetWidth(60) // Safe fallback until the first WindowSizeMsg.
	return ta
}

// Init returns nil by design: Init runs before any message is handled, so at
// that point the viewport does not exist yet and the initial translation must
// not start. That trigger lives in the WindowSizeMsg handler
// (scheduleInitialTranslation), after layout initialization.
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		firstLayout := !m.ready
		m = m.handleWindowSize(msg)
		// refreshHistory re-checks the viewport height after the textarea
		// resize may have changed it, then rebuilds the semantic map for the
		// new width and re-renders the history content.
		m = m.refreshHistory(false)
		if firstLayout {
			// The first WindowSizeMsg is the observable end of layout
			// initialization: handleWindowSize has just created the viewport
			// and set m.ready. Scheduling the initial translation from that
			// boundary — instead of a fixed startup delay in the runtime —
			// guarantees it is handled exactly once, after the viewport
			// exists, with no message-ordering race against this resize.
			return m, scheduleInitialTranslation
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.inputMode {
			return m.handleInputKeyPress(msg)
		}
		return m.handleKeyPress(msg)

	case tea.PasteMsg:
		if m.inputMode {
			return m.handleInputPaste(msg)
		}
		return m, nil

	case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg, tea.MouseWheelMsg:
		if m.inputMode {
			return m.handleInputMouse(msg)
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
	//   Y = header + history viewport + input panel margin/border/padding
	//   X/Y = input panel margin + border + padding before its content
	// Note: textarea.Cursor() already includes prompt width, textarea
	// internal padding/border — we do NOT add those again.
	if m.inputMode {
		c := m.textArea.Cursor()
		if c != nil {
			v.Cursor = c
			x, y := m.inputTextAreaOrigin()
			v.Cursor.Position.X += x
			v.Cursor.Position.Y += y
		}
	}

	return v
}

// scheduleInitialTranslation makes the "layout initialized" boundary
// observable at the message layer: it is returned by the first WindowSizeMsg
// handler and emits InitialTranslationMsg, so the initial translation is
// always handled after the viewport exists. It replaces the fixed 200ms
// startup sleep that used to guess this condition from the clock.
func scheduleInitialTranslation() tea.Msg {
	return InitialTranslationMsg{}
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
		return m.refreshHistory(true), nil
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
	h := m.fixedHeightWithoutInput()
	if m.inputMode {
		h += m.inputPanelHeight()
	}
	return h
}

// fixedHeightWithoutInput is the part of the layout that never depends on
// textarea.Height. Keeping it separate lets maxInputTextAreaHeight derive an
// input limit directly from terminal height without a height feedback loop.
func (m Model) fixedHeightWithoutInput() int {
	h := m.headerHeight() + m.statusBarHeight()
	if m.Loading {
		h++
	}
	if m.Error != "" {
		h += lipgloss.Height(m.renderErrorPanel(m.terminalWidth))
	}
	return h
}

func (m Model) statusBarHeight() int {
	return 1
}

func (m Model) inputPanelHeight() int {
	if !m.inputMode {
		return 0
	}
	return m.textArea.Height() + InputPanelStyle.GetVerticalFrameSize()
}

const (
	minInputTextAreaHeight   = 1
	minHistoryViewportHeight = 1
)

// maxInputTextAreaHeight reserves one history row when the terminal has room
// for it. It deliberately depends only on terminal height, fixed UI, and the
// panel frame -- never on textarea.Height or the current history height.
func (m Model) maxInputTextAreaHeight() int {
	h := m.terminalHeight -
		m.fixedHeightWithoutInput() -
		InputPanelStyle.GetVerticalFrameSize() -
		minHistoryViewportHeight
	if h < minInputTextAreaHeight {
		return minInputTextAreaHeight
	}
	return h
}

// syncInputLayout applies the one-way input layout constraints. SetWidth
// triggers Bubbles' DynamicHeight recalculation after the new MaxHeight is in
// place, so textarea.Height is always its content height clamped to this cap.
func (m Model) syncInputLayout() Model {
	if !m.inputMode {
		return m
	}
	m.textArea.MaxHeight = m.maxInputTextAreaHeight()
	m.textArea.MaxContentHeight = math.MaxInt
	m.textArea.SetWidth(m.inputPanelContentWidth())
	return m
}

// inputPanelOuterWidth is the width passed to InputPanelStyle.Width. Lip Gloss
// defines that width as the complete block width before margins, including the
// panel's border and padding.
func (m Model) inputPanelOuterWidth() int {
	w := m.terminalWidth - InputPanelStyle.GetHorizontalMargins()
	if w < 1 {
		return 1
	}
	return w
}

// inputPanelContentWidth returns the width available inside the input panel's
// border and padding. This is the exact width passed to textarea.SetWidth.
//
// Layout per row (left to right):
//
//	outer margin │ border/padding │ textarea total width │ border/padding │ outer margin
//
// terminal width = panel margins + panel block width
// panel block width = border/padding + textarea total width
// textareaTotalWidth = promptWidth(2) + textContentWidth
//
// textarea.SetWidth receives textareaTotalWidth; it subtracts promptWidth
// internally to get textContentWidth. We do NOT subtract prompt again.
func (m Model) inputPanelContentWidth() int {
	w := m.inputPanelOuterWidth() -
		InputPanelStyle.GetHorizontalBorderSize() -
		InputPanelStyle.GetHorizontalPadding()
	if w < 1 {
		return 1
	}
	return w
}

// recalcViewportHeight keeps the total TUI height exactly equal to the
// terminal height whenever the error panel or loading indicator changes.
func (m Model) recalcViewportHeight() Model {
	if m.ready {
		m.viewport.SetHeight(m.historyViewportHeight())
	}
	return m
}

// refreshHistory is the single refresh path for the history panel. Every
// handler that changes history records or layout calls it, so exactly one
// authoritative semantic map exists: it is rebuilt here (never implicitly
// inside the renderer), the viewport height is recalculated, and the viewport
// content is re-rendered from that same map. scrollToBottom pins the view to
// the newest record; callers that only re-render without appending (resize,
// error changes) pass false so the existing scroll position is preserved.
func (m Model) refreshHistory(scrollToBottom bool) Model {
	m = m.recalcViewportHeight()
	m.buildSemanticMap(m.Records, m.historyContentWidth())
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	if scrollToBottom {
		m.viewport.GotoBottom()
	}
	return m
}

// historyContentWidth is the one width used to both build the semantic map
// and render the history cards, so wrapped semantic rows and rendered rows
// can never disagree about where a line breaks.
func (m Model) historyContentWidth() int {
	if w := m.viewport.Width(); w > 0 {
		return w
	}
	return 80
}

func (m Model) historyViewportHeight() int {
	h := m.terminalHeight - m.staticHeight()
	if h < 0 {
		return 0
	}
	return h
}

func (m Model) headerHeight() int {
	return 1
}

// inputTextAreaOrigin is the terminal-cell location of textarea.View(). The
// textarea itself has no project-added frame; its origin is the content origin
// inside InputPanelStyle. Cursor placement and mouse selection share this
// calculation so they cannot drift apart.
func (m Model) inputTextAreaOrigin() (x, y int) {
	x = InputPanelStyle.GetMarginLeft() +
		InputPanelStyle.GetBorderLeftSize() +
		InputPanelStyle.GetPaddingLeft()
	y = m.headerHeight() + m.viewport.Height() +
		InputPanelStyle.GetMarginTop() +
		InputPanelStyle.GetBorderTopSize() +
		InputPanelStyle.GetPaddingTop()
	return x, y
}

func (m Model) inputTextAreaMousePosition(mouse tea.Mouse) (x, y int) {
	originX, originY := m.inputTextAreaOrigin()
	return mouse.X - originX, mouse.Y - originY
}

func (m Model) mouseInInputTextArea(mouse tea.Mouse) bool {
	x, y := m.inputTextAreaMousePosition(mouse)
	return x >= 0 && x < m.inputPanelContentWidth() &&
		y >= 0 && y < m.textArea.Height()
}

func (m Model) mouseInHistoryViewport(mouse tea.Mouse) bool {
	return mouse.X >= 0 && mouse.X < m.viewport.Width() &&
		mouse.Y >= m.headerHeight() &&
		mouse.Y < m.headerHeight()+m.viewport.Height()
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
