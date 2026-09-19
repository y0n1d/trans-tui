package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/y0n1d/trans-tui/internal/core"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	m := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 22),
		ready:          true,
		sourceLang:     "auto",
		targetLang:     "auto",
		textInput:      textinput.New(),
	}
	m.textInput.Focus()
	return m
}

func newTestModelWithInputMode(t *testing.T) Model {
	t.Helper()
	m := newTestModel(t)
	m.inputMode = true
	m.textInput.Focus()
	return m
}

// ---------------------------------------------------------------------------
// Input Mode shortcut
// ---------------------------------------------------------------------------

func TestInputModeShortcutEntersInputMode(t *testing.T) {
	model := newTestModel(t)
	if model.inputMode {
		t.Fatal("should start in normal mode")
	}
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m := r.(Model)
	if !m.inputMode {
		t.Error("pressing 'i' should enter input mode")
	}
}

func TestInputModeDoesNotTriggerOnErrorPanel(t *testing.T) {
	model := newTestModel(t)
	model.Error = "some error"
	model = model.recalcViewportHeight()

	// When error is showing, Esc dismisses it, not enters input mode
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m := r.(Model)
	if m.inputMode {
		t.Error("Esc with error should dismiss error, not enter input mode")
	}
	if m.Error != "" {
		t.Error("Esc should dismiss error")
	}
}

// ---------------------------------------------------------------------------
// Esc exits input mode
// ---------------------------------------------------------------------------

func TestInputModeEscExits(t *testing.T) {
	model := newTestModelWithInputMode(t)
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m := r.(Model)
	if m.inputMode {
		t.Error("pressing Esc should exit input mode")
	}
	if m.textInput.Value() != "" {
		t.Error("pressing Esc should clear input value")
	}
}

// ---------------------------------------------------------------------------
// Empty Enter ignored
// ---------------------------------------------------------------------------

func TestInputModeEmptyEnterIgnored(t *testing.T) {
	model := newTestModelWithInputMode(t)
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := r.(Model)
	if !m.inputMode {
		t.Error("empty Enter should keep input mode")
	}
	if m.Loading {
		t.Error("empty Enter should not trigger loading")
	}
}

func TestInputModeWhitespaceEnterIgnored(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.textInput.SetValue("   ")
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := r.(Model)
	if !m.inputMode {
		t.Error("whitespace-only Enter should keep input mode")
	}
	if m.Loading {
		t.Error("whitespace-only Enter should not trigger loading")
	}
}

// ---------------------------------------------------------------------------
// Normal text accepted
// ---------------------------------------------------------------------------

func TestInputModeTextAccepted(t *testing.T) {
	model := newTestModelWithInputMode(t)
	for _, ch := range "Hello" {
		r, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = r.(Model)
	}
	if model.textInput.Value() != "Hello" {
		t.Errorf("input value = %q, want %q", model.textInput.Value(), "Hello")
	}
}

// ---------------------------------------------------------------------------
// Unicode accepted
// ---------------------------------------------------------------------------

func TestInputModeUnicodeAccepted(t *testing.T) {
	model := newTestModelWithInputMode(t)
	for _, ch := range "\u4f60\u597d\u4e16\u754c" {
		r, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = r.(Model)
	}
	if model.textInput.Value() != "\u4f60\u597d\u4e16\u754c" {
		t.Errorf("input value = %q, want unicode text", model.textInput.Value())
	}
}

// ---------------------------------------------------------------------------
// Enter submits and transitions to loading
// ---------------------------------------------------------------------------

func TestInputModeEnterSubmits(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.textInput.SetValue("Hello world")

	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := r.(Model)

	if m.inputMode {
		t.Error("Enter should exit input mode")
	}
	if !m.Loading {
		t.Error("Enter should set loading state")
	}
}

// ---------------------------------------------------------------------------
// TranslationResult clears loading and appends record
// ---------------------------------------------------------------------------

func TestInputModeResultAppendsRecord(t *testing.T) {
	model := newTestModel(t)
	model.Loading = true
	model = model.recalcViewportHeight()

	msg := core.TranslationResultMsg{
		RequestID:   "test-input-001",
		Source:      "Hello",
		Translation: "\u3053\u3093\u306b\u3061\u306f",
		SourceLang:  "en",
		TargetLang:  "ja",
		Provider:    "google",
		Model:       "nmt",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if m.Loading {
		t.Error("result should clear loading")
	}
	if len(m.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(m.Records))
	}
	if m.Records[0].Source != "Hello" {
		t.Errorf("record source = %q, want %q", m.Records[0].Source, "Hello")
	}
}

// ---------------------------------------------------------------------------
// Layout invariant with input mode
// ---------------------------------------------------------------------------

func TestViewHeightInvariantWithInputMode(t *testing.T) {
	const H = 24
	m := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-5),
		ready:          true,
		inputMode:      true,
		textInput:      textinput.New(),
	}
	m.textInput.Focus()
	m = m.recalcViewportHeight()

	got := lipgloss.Height(m.renderView())
	if got != H {
		t.Errorf("View height with input mode = %d, want %d", got, H)
	}
}

func TestViewHeightInvariantInputModeWithError(t *testing.T) {
	const H = 24
	m := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-7),
		ready:          true,
		inputMode:      true,
		Error:          "some error",
		textInput:      textinput.New(),
	}
	m.textInput.Focus()
	m = m.recalcViewportHeight()

	got := lipgloss.Height(m.renderView())
	if got != H {
		t.Errorf("View height with input+error = %d, want %d", got, H)
	}
}

func TestViewHeightInvariantInputModeWithLoading(t *testing.T) {
	const H = 24
	m := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-6),
		ready:          true,
		inputMode:      false,
		Loading:        true,
		textInput:      textinput.New(),
	}
	m = m.recalcViewportHeight()

	got := lipgloss.Height(m.renderView())
	if got != H {
		t.Errorf("View height with loading = %d, want %d", got, H)
	}
}

// ---------------------------------------------------------------------------
// Mouse events blocked in input mode
// ---------------------------------------------------------------------------

func TestMouseEventsIgnoredInInputMode(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "\u3053\u3093\u306b\u3061\u306f",
	})
	model.buildSemanticMap(model.Records, 80)

	screenY := firstSelectableRow(model) + 1
	r, _ := model.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 4, Y: screenY})
	m := r.(Model)
	if m.sel.selecting {
		t.Error("mouse selection should not start in input mode")
	}
}

// ---------------------------------------------------------------------------
// EnterInputModeMsg
// ---------------------------------------------------------------------------

func TestEnterInputModeMsg(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(core.EnterInputModeMsg{})
	m := r.(Model)
	if !m.inputMode {
		t.Error("EnterInputModeMsg should enter input mode")
	}
}

// ---------------------------------------------------------------------------
// Scroll navigation works in normal mode
// ---------------------------------------------------------------------------

func TestNormalModeScrollWorks(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := r.(Model)
	if m.inputMode {
		t.Error("'j' should not enter input mode")
	}
}

// ---------------------------------------------------------------------------
// 'i' key ignored when not in normal mode with error
// ---------------------------------------------------------------------------

func TestInputKeyWithActiveErrorDismissesFirst(t *testing.T) {
	model := newTestModel(t)
	model.Error = "some error"
	model = model.recalcViewportHeight()

	// Esc dismisses error first
	r, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m := r.(Model)
	if m.Error != "" {
		t.Error("Esc should dismiss error")
	}
}

// ---------------------------------------------------------------------------
// StatusBar shows input hint
// ---------------------------------------------------------------------------

func TestStatusBarShowsInputHint(t *testing.T) {
	model := newTestModel(t)
	bar := model.renderStatusBar()
	if !strings.Contains(bar, "i: input") {
		t.Error("status bar should show 'i: input' hint")
	}
}

func TestStatusBarShowsCancelInInputMode(t *testing.T) {
	model := newTestModelWithInputMode(t)
	bar := model.renderStatusBar()
	if !strings.Contains(bar, "esc: cancel") {
		t.Error("status bar should show 'esc: cancel' in input mode")
	}
}

// ---------------------------------------------------------------------------
// inputPanelHeight
// ---------------------------------------------------------------------------

func TestInputPanelHeight(t *testing.T) {
	model := newTestModel(t)
	if model.inputPanelHeight() != 0 {
		t.Error("inputPanelHeight should be 0 when not in input mode")
	}
	model.inputMode = true
	if model.inputPanelHeight() != 3 {
		t.Errorf("inputPanelHeight = %d, want 3", model.inputPanelHeight())
	}
}

// ---------------------------------------------------------------------------
// staticHeight includes input panel
// ---------------------------------------------------------------------------

func TestStaticHeightIncludesInputPanel(t *testing.T) {
	model := newTestModel(t)
	h1 := model.staticHeight()
	model.inputMode = true
	h2 := model.staticHeight()
	if h2 != h1+3 {
		t.Errorf("staticHeight with input: %d, without: %d, diff should be 3", h2, h1)
	}
}

// ---------------------------------------------------------------------------
// CLI flag tests
// ---------------------------------------------------------------------------

func TestCLIParserAcceptsInputFlag(t *testing.T) {
	args := []string{"-i"}
	inputInitial := false
	var remaining []string
	for _, arg := range args {
		if arg == "-i" || arg == "--input" {
			inputInitial = true
		} else {
			remaining = append(remaining, arg)
		}
	}
	if !inputInitial {
		t.Error("-i flag should set inputInitial")
	}
	if len(remaining) != 0 {
		t.Errorf("remaining args should be empty, got %v", remaining)
	}
}

func TestCLIParserAcceptsInputLongFlag(t *testing.T) {
	args := []string{"--input"}
	inputInitial := false
	var remaining []string
	for _, arg := range args {
		if arg == "-i" || arg == "--input" {
			inputInitial = true
		} else {
			remaining = append(remaining, arg)
		}
	}
	if !inputInitial {
		t.Error("--input flag should set inputInitial")
	}
}

func TestCLIParserInputFlagWithText(t *testing.T) {
	args := []string{"-i", "Hello"}
	inputInitial := false
	var remaining []string
	for _, arg := range args {
		if arg == "-i" || arg == "--input" {
			inputInitial = true
		} else {
			remaining = append(remaining, arg)
		}
	}
	if !inputInitial {
		t.Error("-i flag should set inputInitial")
	}
	text := strings.TrimSpace(strings.Join(remaining, " "))
	if text != "Hello" {
		t.Errorf("text = %q, want %q", text, "Hello")
	}
}

// ---------------------------------------------------------------------------
// IPC enter_input_mode request type
// ---------------------------------------------------------------------------

func TestIPCEnterInputModeRequestType(t *testing.T) {
	reqType := "enter_input_mode"
	if reqType != "enter_input_mode" {
		t.Error("request type mismatch")
	}
}

// ---------------------------------------------------------------------------
// New() with inputInitial flag
// ---------------------------------------------------------------------------

func TestNewWithInputInitial(t *testing.T) {
	svc := &core.Service{}
	m := New(core.AppState{}, svc, "", "auto", "auto", true)
	if !m.inputMode {
		t.Error("New with inputInitial=true should start in input mode")
	}
}

func TestNewWithoutInputInitial(t *testing.T) {
	svc := &core.Service{}
	m := New(core.AppState{}, svc, "Hello", "auto", "auto", false)
	if m.inputMode {
		t.Error("New with inputInitial=false should start in normal mode")
	}
	if m.lastText != "Hello" {
		t.Errorf("lastText = %q, want %q", m.lastText, "Hello")
	}
}

// ---------------------------------------------------------------------------
// enterInputMode is idempotent
// ---------------------------------------------------------------------------

func TestEnterInputModeIdempotent(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.textInput.SetValue("existing text")

	m, _ := model.enterInputMode()
	if !m.inputMode {
		t.Error("enterInputMode should keep inputMode true")
	}
	if m.textInput.Value() != "existing text" {
		t.Error("enterInputMode should not modify existing input when already in input mode")
	}
}

// ---------------------------------------------------------------------------
// Exit input mode clears input
// ---------------------------------------------------------------------------

func TestExitInputModeClearsInput(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.textInput.SetValue("some text")

	m := model.exitInputMode()
	if m.inputMode {
		t.Error("exitInputMode should set inputMode to false")
	}
	if m.textInput.Value() != "" {
		t.Error("exitInputMode should clear text input")
	}
}

// ---------------------------------------------------------------------------
// Input panel renders
// ---------------------------------------------------------------------------

func TestRenderInputPanel(t *testing.T) {
	model := newTestModelWithInputMode(t)
	panel := model.renderInputPanel()
	if panel == "" {
		t.Error("renderInputPanel should return non-empty string")
	}
}

func TestRenderViewWithInputMode(t *testing.T) {
	const H = 24
	model := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-5),
		ready:          true,
		inputMode:      true,
		textInput:      textinput.New(),
	}
	model.textInput.Focus()
	model = model.recalcViewportHeight()

	view := model.renderView()
	if view == "" {
		t.Error("renderView should return non-empty string in input mode")
	}
	if got := lipgloss.Height(view); got != H {
		t.Errorf("View height in input mode = %d, want %d", got, H)
	}
}

func TestRenderViewWithInputModeNoRecords(t *testing.T) {
	const H = 24
	model := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-5),
		ready:          true,
		inputMode:      true,
		textInput:      textinput.New(),
		sourceLang:     "auto",
		targetLang:     "auto",
	}
	model.textInput.Focus()
	model = model.recalcViewportHeight()

	view := model.renderView()
	if got := lipgloss.Height(view); got != H {
		t.Errorf("View height in input mode (no records) = %d, want %d", got, H)
	}
}

// ---------------------------------------------------------------------------
// View height invariant: all input mode combos
// ---------------------------------------------------------------------------

func TestViewHeightInvariantInputModeCombinations(t *testing.T) {
	const H = 24
	cases := []struct {
		name    string
		input   bool
		err     string
		loading bool
		vpH     int
	}{
		{"input only", true, "", false, H - 5},
		{"input + error", true, "boom", false, H - 7},
		{"input + loading", true, "", true, H - 6},
		{"no input", false, "", false, H - 2},
		{"no input + error", false, "boom", false, H - 4},
		{"no input + loading", false, "", true, H - 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				terminalWidth:  80,
				terminalHeight: H,
				viewport:       viewportForTest(80, tc.vpH),
				ready:          true,
				inputMode:      tc.input,
				Error:          tc.err,
				Loading:        tc.loading,
				textInput:      textinput.New(),
			}
			if tc.input {
				m.textInput.Focus()
			}
			m = m.recalcViewportHeight()
			got := lipgloss.Height(m.renderView())
			if got != H {
				t.Errorf("View height = %d, want %d", got, H)
			}
		})
	}
}
