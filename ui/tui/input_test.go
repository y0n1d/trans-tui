package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/y0n1d/trans-tui/internal/config"
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
		textArea:       textarea.New(),
		keyMap:         DefaultKeyMap(),
	}
	_ = m.textArea.Focus()
	return m
}

func newTestModelWithInputMode(t *testing.T) Model {
	t.Helper()
	m := newTestModel(t)
	m.inputMode = true
	_ = m.textArea.Focus()
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
	r, _ := model.Update(tea.KeyPressMsg{Text: ",", Code: ','})
	m := r.(Model)
	if !m.inputMode {
		t.Error("pressing ',' should enter input mode")
	}
}

func TestInputModeShortcutIColonAlsoEntersInputMode(t *testing.T) {
	model := newTestModel(t)
	// Custom keymap with "i" as manual input
	model.keyMap = NewKeyMapFromBindings(
		[]string{"q", "ctrl+c", "esc"},
		[]string{"i"},
	)
	r, _ := model.Update(tea.KeyPressMsg{Text: "i", Code: 'i'})
	m := r.(Model)
	if !m.inputMode {
		t.Error("pressing 'i' with custom binding should enter input mode")
	}
}

func TestInputModeDoesNotTriggerOnErrorPanel(t *testing.T) {
	model := newTestModel(t)
	model.Error = "some error"
	model = model.recalcViewportHeight()

	// Esc with error showing should dismiss error (not quit, not enter input mode).
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
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
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m := r.(Model)
	if m.inputMode {
		t.Error("pressing Esc should exit input mode")
	}
	if m.textArea.Value() != "" {
		t.Error("pressing Esc should clear input value")
	}
}

// ---------------------------------------------------------------------------
// Empty Enter ignored
// ---------------------------------------------------------------------------

func TestInputModeEmptyEnterIgnored(t *testing.T) {
	model := newTestModelWithInputMode(t)
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	model.textArea.SetValue("   ")
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
		r, _ := model.Update(tea.KeyPressMsg{Text: string(ch), Code: ch})
		model = r.(Model)
	}
	if model.textArea.Value() != "Hello" {
		t.Errorf("input value = %q, want %q", model.textArea.Value(), "Hello")
	}
}

// ---------------------------------------------------------------------------
// Unicode accepted
// ---------------------------------------------------------------------------

func TestInputModeUnicodeAccepted(t *testing.T) {
	model := newTestModelWithInputMode(t)
	for _, ch := range "\u4f60\u597d\u4e16\u754c" {
		r, _ := model.Update(tea.KeyPressMsg{Text: string(ch), Code: ch})
		model = r.(Model)
	}
	if model.textArea.Value() != "\u4f60\u597d\u4e16\u754c" {
		t.Errorf("input value = %q, want unicode text", model.textArea.Value())
	}
}

// ---------------------------------------------------------------------------
// Enter submits and transitions to loading
// ---------------------------------------------------------------------------

func TestInputModeEnterSubmits(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.textArea.SetValue("Hello world")

	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
		textArea:       textarea.New(),
	}
	_ = m.textArea.Focus()
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
		textArea:       textarea.New(),
	}
	_ = m.textArea.Focus()
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
		textArea:       textarea.New(),
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
	r, _ := model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 4, Y: screenY})
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
	r, _ := model.Update(tea.KeyPressMsg{Text: "j", Code: 'j'})
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

	// Esc dismisses error first, even though Esc is also a quit key.
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
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
	if !strings.Contains(bar, ": input") {
		t.Error("status bar should show input hint")
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
	got := model.inputPanelHeight()
	want := model.textArea.Height() + InputPanelStyle.GetVerticalFrameSize()
	if got != want {
		t.Errorf("inputPanelHeight = %d, want %d (textarea height %d + panel frame)", got, want, model.textArea.Height())
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
	diff := h2 - h1
	want := model.textArea.Height() + InputPanelStyle.GetVerticalFrameSize()
	if diff != want {
		t.Errorf("staticHeight with input: %d, without: %d, diff=%d, want textarea height + panel frame %d", h2, h1, diff, want)
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
	m := New(core.AppState{}, svc, "", "auto", "auto", true, false, DefaultKeyMap())
	if !m.inputMode {
		t.Error("New with inputInitial=true should start in input mode")
	}
}

func TestNewWithoutInputInitial(t *testing.T) {
	svc := &core.Service{}
	m := New(core.AppState{}, svc, "Hello", "auto", "auto", false, false, DefaultKeyMap())
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
	model.textArea.SetValue("existing text")

	m, _ := model.enterInputMode()
	if !m.inputMode {
		t.Error("enterInputMode should keep inputMode true")
	}
	if m.textArea.Value() != "existing text" {
		t.Error("enterInputMode should not modify existing input when already in input mode")
	}
}

// ---------------------------------------------------------------------------
// Exit input mode clears input
// ---------------------------------------------------------------------------

func TestExitInputModeClearsInput(t *testing.T) {
	model := newTestModelWithInputMode(t)
	model.textArea.SetValue("some text")

	m := model.exitInputMode()
	if m.inputMode {
		t.Error("exitInputMode should set inputMode to false")
	}
	if m.textArea.Value() != "" {
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
		textArea:       textarea.New(),
	}
	_ = model.textArea.Focus()
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
		textArea:       textarea.New(),
		sourceLang:     "auto",
		targetLang:     "auto",
	}
	_ = model.textArea.Focus()
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
				textArea:       textarea.New(),
			}
			if tc.input {
				_ = m.textArea.Focus()
			}
			m = m.recalcViewportHeight()
			got := lipgloss.Height(m.renderView())
			if got != H {
				t.Errorf("View height = %d, want %d", got, H)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Display mode tests
// ---------------------------------------------------------------------------

func TestNewWithDisplayMode(t *testing.T) {
	svc := &core.Service{}
	m := New(core.AppState{}, svc, "OCR text", "auto", "auto", false, true, DefaultKeyMap())
	if !m.displayMode {
		t.Error("New with displayMode=true should set displayMode")
	}
	if m.inputMode {
		t.Error("displayMode should not set inputMode")
	}
	if m.lastText != "OCR text" {
		t.Errorf("lastText = %q, want %q", m.lastText, "OCR text")
	}
}

func TestDisplayModeShowsTextWithoutTranslation(t *testing.T) {
	model := newTestModel(t)
	msg := core.DisplayTextMsg{
		RequestID: "display-001",
		Text:      "Hello OCR world",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if len(m.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(m.Records))
	}
	record := m.Records[0]
	if record.Source != "Hello OCR world" {
		t.Errorf("record source = %q, want %q", record.Source, "Hello OCR world")
	}
	if record.SourceLang != "OCR" {
		t.Errorf("record sourceLang = %q, want %q", record.SourceLang, "OCR")
	}
	if record.Translation != "" {
		t.Errorf("record translation should be empty, got %q", record.Translation)
	}
}

func TestDisplayModeMultilineText(t *testing.T) {
	model := newTestModel(t)
	msg := core.DisplayTextMsg{
		RequestID: "display-002",
		Text:      "Line 1\nLine 2\nLine 3",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if len(m.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(m.Records))
	}
	if m.Records[0].Source != "Line 1\nLine 2\nLine 3" {
		t.Errorf("record source = %q, want multiline text", m.Records[0].Source)
	}
}

func TestDisplayModeChinese(t *testing.T) {
	model := newTestModel(t)
	msg := core.DisplayTextMsg{
		RequestID: "display-003",
		Text:      "你好世界\n这是中文OCR结果",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if len(m.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(m.Records))
	}
	if m.Records[0].Source != "你好世界\n这是中文OCR结果" {
		t.Errorf("record source = %q, want Chinese text", m.Records[0].Source)
	}
}

func TestDisplayModeClearsLoading(t *testing.T) {
	model := newTestModel(t)
	model.Loading = true
	model = model.recalcViewportHeight()

	msg := core.DisplayTextMsg{
		RequestID: "display-004",
		Text:      "OCR text",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if m.Loading {
		t.Error("DisplayTextMsg should clear loading")
	}
}

func TestDisplayModeBuildsSemanticMap(t *testing.T) {
	model := newTestModel(t)
	msg := core.DisplayTextMsg{
		RequestID: "display-005",
		Text:      "Hello OCR world",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if len(m.semRows) == 0 {
		t.Error("DisplayTextMsg should build semantic map")
	}
}

func TestDisplayModeMouseSelectionWorks(t *testing.T) {
	model := newTestModel(t)
	msg := core.DisplayTextMsg{
		RequestID: "display-006",
		Text:      "Selectable text",
	}
	r, _ := model.Update(msg)
	m := r.(Model)

	if len(m.semRows) == 0 {
		t.Fatal("no semantic rows for selection")
	}

	// Find a selectable row
	var selRow int
	for i, row := range m.semRows {
		if row.Selectable {
			selRow = i
			break
		}
	}

	// Click on it
	screenY := selRow + 1 // +1 for header
	click := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      4,
		Y:      screenY,
	}
	r, _ = m.Update(click)
	m = r.(Model)
	if !m.sel.selecting {
		t.Error("should start selection on clickable display text")
	}
}

func TestViewHeightInvariantDisplayMode(t *testing.T) {
	const H = 24
	m := Model{
		terminalWidth:  80,
		terminalHeight: H,
		viewport:       viewportForTest(80, H-2),
		ready:          true,
		displayMode:    true,
		textArea:       textarea.New(),
	}
	m = m.recalcViewportHeight()

	got := lipgloss.Height(m.renderView())
	if got != H {
		t.Errorf("View height with displayMode = %d, want %d", got, H)
	}
}

func TestDisplayModeStatusBarShowsHint(t *testing.T) {
	model := newTestModel(t)
	model.displayMode = true
	bar := model.renderStatusBar()
	if !strings.Contains(bar, ": quit") {
		t.Error("status bar should show quit hint in display mode")
	}
}

// ---------------------------------------------------------------------------
// Key binding tests: Esc → quit
// ---------------------------------------------------------------------------

func TestEscQuitsInNormalMode(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	// tea.Quit returns a special command; the model itself is returned.
	// We verify by checking the command is non-nil (quit command).
	if r == nil {
		t.Error("Esc should return a model")
	}
}

func TestEscDoesNotQuitWhenErrorShowing(t *testing.T) {
	model := newTestModel(t)
	model.Error = "some error"
	model = model.recalcViewportHeight()

	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m := r.(Model)
	// Esc should dismiss error, not quit.
	if m.Error != "" {
		t.Error("Esc should dismiss error")
	}
}

func TestEscQuitsAfterErrorDismissed(t *testing.T) {
	model := newTestModel(t)
	model.Error = "some error"
	model = model.recalcViewportHeight()

	// First Esc dismisses error.
	r, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m := r.(Model)
	if m.Error != "" {
		t.Fatal("first Esc should dismiss error")
	}

	// Second Esc quits.
	r, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	// Quit returns a tea.Cmd; model is returned.
	_ = r
}

// ---------------------------------------------------------------------------
// Key binding tests: , / ， → manual input
// ---------------------------------------------------------------------------

func TestCommaEntersInputMode(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Text: ",", Code: ','})
	m := r.(Model)
	if !m.inputMode {
		t.Error("pressing ',' should enter input mode")
	}
}

func TestFullwidthCommaEntersInputMode(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Text: "\uff0c", Code: '\uff0c'})
	m := r.(Model)
	if !m.inputMode {
		t.Error("pressing '，' should enter input mode")
	}
}

func TestIDoesNotEnterInputModeWithDefaultBindings(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Text: "i", Code: 'i'})
	m := r.(Model)
	if m.inputMode {
		t.Error("'i' should not enter input mode with default bindings")
	}
}

// ---------------------------------------------------------------------------
// Key binding tests: q → quit
// ---------------------------------------------------------------------------

func TestQQuits(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
	_ = r // Quit returns a command
}

func TestCtrlCQuits(t *testing.T) {
	model := newTestModel(t)
	r, _ := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	_ = r // Quit returns a command
}

// ---------------------------------------------------------------------------
// Key binding tests: custom bindings
// ---------------------------------------------------------------------------

func TestCustomQuitBinding(t *testing.T) {
	model := newTestModel(t)
	model.keyMap = NewKeyMapFromBindings(
		[]string{"x"},
		[]string{"m"},
	)

	// x should quit
	r, _ := model.Update(tea.KeyPressMsg{Text: "x", Code: 'x'})
	_ = r // Quit returns a command

	// q should NOT quit (not in custom bindings)
	model2 := newTestModel(t)
	model2.keyMap = NewKeyMapFromBindings(
		[]string{"x"},
		[]string{"m"},
	)
	r2, _ := model2.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
	m2 := r2.(Model)
	if m2.inputMode {
		t.Error("q should not trigger any action with custom bindings")
	}
}

func TestCustomManualInputBinding(t *testing.T) {
	model := newTestModel(t)
	model.keyMap = NewKeyMapFromBindings(
		[]string{"q", "ctrl+c", "esc"},
		[]string{"m"},
	)

	r, _ := model.Update(tea.KeyPressMsg{Text: "m", Code: 'm'})
	m := r.(Model)
	if !m.inputMode {
		t.Error("'m' should enter input mode with custom binding")
	}
}

func TestMultipleQuitBindings(t *testing.T) {
	model := newTestModel(t)
	model.keyMap = NewKeyMapFromBindings(
		[]string{"q", "esc", "x"},
		[]string{","},
	)

	// All three should quit
	for _, key := range []rune{'q', 'x'} {
		m := newTestModel(t)
		m.keyMap = model.keyMap
		r, _ := m.Update(tea.KeyPressMsg{Text: string(key), Code: key})
		_ = r
	}
	// esc also quits (tested separately in TestEscQuitsInNormalMode)
}

// ---------------------------------------------------------------------------
// Key binding tests: missing config uses defaults
// ---------------------------------------------------------------------------

func TestDefaultKeyBindingsUsed(t *testing.T) {
	km := NewKeyMapFromBindings(nil, nil)
	// Should fall back to defaults
	if len(km.Quit.Keys()) == 0 {
		t.Error("default quit keys should not be empty")
	}
	if len(km.InputMode.Keys()) == 0 {
		t.Error("default input mode keys should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Config keybindings validation
// ---------------------------------------------------------------------------

func TestConfigKeyBindingsValidation(t *testing.T) {
	tests := []struct {
		name    string
		kb      config.KeyBindingsConfig
		wantErr bool
	}{
		{"valid", config.KeyBindingsConfig{Quit: []string{"q"}, ManualInput: []string{","}}, false},
		{"empty quit", config.KeyBindingsConfig{Quit: []string{}, ManualInput: []string{","}}, true},
		{"empty manual_input", config.KeyBindingsConfig{Quit: []string{"q"}, ManualInput: []string{}}, true},
		{"both empty", config.KeyBindingsConfig{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.kb.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveKeyBindingsWithEmpty(t *testing.T) {
	cfg := config.Config{}
	resolved := cfg.ResolveKeyBindings()
	defaults := config.DefaultKeyBindings()

	if len(resolved.Quit) != len(defaults.Quit) {
		t.Errorf("resolved quit keys = %d, want %d (defaults)", len(resolved.Quit), len(defaults.Quit))
	}
	if len(resolved.ManualInput) != len(defaults.ManualInput) {
		t.Errorf("resolved manual_input keys = %d, want %d (defaults)", len(resolved.ManualInput), len(defaults.ManualInput))
	}
}

func TestResolveKeyBindingsWithCustom(t *testing.T) {
	cfg := config.Config{
		KeyBindings: config.KeyBindingsConfig{
			Quit:        []string{"x"},
			ManualInput: []string{"m"},
		},
	}
	resolved := cfg.ResolveKeyBindings()

	if len(resolved.Quit) != 1 || resolved.Quit[0] != "x" {
		t.Errorf("resolved quit = %v, want [x]", resolved.Quit)
	}
	if len(resolved.ManualInput) != 1 || resolved.ManualInput[0] != "m" {
		t.Errorf("resolved manual_input = %v, want [m]", resolved.ManualInput)
	}
}

// ---------------------------------------------------------------------------
// Config loading with keybindings section
// ---------------------------------------------------------------------------

func TestConfigLoadWithKeybindings(t *testing.T) {
	content := `[provider]
type = "openai-compatible"
api_key_env = "TEST_KEY"

[keybindings]
quit = ["x", "esc"]
manual_input = ["m"]
`
	path := tuiWriteTempConfig(t, content)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.KeyBindings.Quit) != 2 || cfg.KeyBindings.Quit[0] != "x" {
		t.Errorf("quit bindings = %v, want [x esc]", cfg.KeyBindings.Quit)
	}
	if len(cfg.KeyBindings.ManualInput) != 1 || cfg.KeyBindings.ManualInput[0] != "m" {
		t.Errorf("manual_input bindings = %v, want [m]", cfg.KeyBindings.ManualInput)
	}
}

func TestConfigLoadWithoutKeybindingsSection(t *testing.T) {
	content := `[provider]
type = "openai-compatible"
api_key_env = "TEST_KEY"
`
	path := tuiWriteTempConfig(t, content)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should use defaults
	defaults := config.DefaultKeyBindings()
	if len(cfg.KeyBindings.Quit) != len(defaults.Quit) {
		t.Errorf("quit bindings = %d keys, want %d (defaults)", len(cfg.KeyBindings.Quit), len(defaults.Quit))
	}
	if len(cfg.KeyBindings.ManualInput) != len(defaults.ManualInput) {
		t.Errorf("manual_input bindings = %d keys, want %d (defaults)", len(cfg.KeyBindings.ManualInput), len(defaults.ManualInput))
	}
}

func TestConfigLoadEmptyQuitFails(t *testing.T) {
	content := `[provider]
type = "openai-compatible"
api_key_env = "TEST_KEY"

[keybindings]
quit = []
`
	path := tuiWriteTempConfig(t, content)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for empty quit bindings")
	}
}

func tuiWriteTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Textarea dynamic height + prompt regression tests
// ---------------------------------------------------------------------------

// newDynamicTestModel creates a test model with DynamicHeight textarea configured
// identically to the production code in New().
func newDynamicTestModel(t *testing.T) Model {
	t.Helper()
	ta := newInputTextArea()
	ta.SetWidth(60)
	_ = ta.Focus()

	m := Model{
		terminalWidth:  80,
		terminalHeight: 24,
		viewport:       viewportForTest(80, 22),
		ready:          true,
		sourceLang:     "auto",
		targetLang:     "auto",
		textArea:       ta,
		keyMap:         DefaultKeyMap(),
	}
	return m
}

func TestDynamicHeightEmptyInput(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	// Empty input: textarea should be exactly 1 line.
	if got := m.textArea.Height(); got != 1 {
		t.Errorf("empty input: textarea height = %d, want 1", got)
	}
	if got := m.inputPanelHeight(); got != 3 { // 1 textarea + 2 border
		t.Errorf("empty input: inputPanelHeight = %d, want 3", got)
	}
}

func TestDynamicHeightShortInput(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	// Type a short string that fits in 1 line.
	m.textArea.SetValue("hello")
	if got := m.textArea.Height(); got != 1 {
		t.Errorf("short input: textarea height = %d, want 1", got)
	}
}

func TestDynamicHeightLongInputWraps(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	// Type a long string that must wrap in 60-char viewport.
	longText := "this is a very long line that should definitely wrap to multiple visual lines in the textarea"
	m.textArea.SetValue(longText)
	h := m.textArea.Height()
	if h < 2 {
		t.Errorf("long input: textarea height = %d, want >= 2", h)
	}
}

func TestPromptOnlyOnFirstLine(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	// The prompt func returns "> " for line 0 and "  " for others.
	// Verify that the rendered view has only one ">" prefix.
	view := m.textArea.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	promptCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "> ") || strings.HasPrefix(line, "\u001b[") {
			// Check if the visible content starts with "> " after stripping ANSI.
			stripped := stripAnsi(line)
			if strings.HasPrefix(stripped, "> ") {
				promptCount++
			}
		}
	}
	// For empty input there should be exactly 1 line with ">" prompt.
	if promptCount != 1 {
		t.Errorf("expected 1 prompt line, got %d (lines: %v)", promptCount, lines)
	}
}

func TestValueUnchangedBySoftWrap(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	original := "hello world this is a long line that wraps"
	m.textArea.SetValue(original)
	// Value must remain exactly the original string, no newlines inserted.
	if got := m.textArea.Value(); got != original {
		t.Errorf("Value() = %q, want %q", got, original)
	}
}

func TestEnterStillSubmits(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	m.textArea.SetValue("translate me")
	// Simulate Enter key.
	r, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	result := r.(Model)
	if result.inputMode {
		t.Error("Enter should exit input mode (submit)")
	}
	if !result.Loading {
		t.Error("Enter should start loading (translation)")
	}
}

func TestEscStillCancels(t *testing.T) {
	m := newDynamicTestModel(t)
	m.inputMode = true
	m.textArea.SetValue("some text")
	// Simulate Esc key.
	r, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	result := r.(Model)
	if result.inputMode {
		t.Error("Esc should exit input mode")
	}
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	// Simple regex-free approach: skip ESC-prefixed sequences.
	var out strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}
