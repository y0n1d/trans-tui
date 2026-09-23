package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
)

// remappedModel returns a normal-mode model whose keymap starts from the
// defaults with the given actions rebound through the production constructor.
func remappedModel(t *testing.T, mutate func(*config.KeyBindingsConfig)) Model {
	t.Helper()
	m := newTestModel(t)
	kb := config.DefaultKeyBindings()
	mutate(&kb)
	m.keyMap = NewKeyMapFromBindings(kb)
	return m
}

// remappedScrollModel adds a history several viewports tall (same fixture as
// the default-scroll test) on top of a remapped keymap.
func remappedScrollModel(t *testing.T, mutate func(*config.KeyBindingsConfig)) Model {
	t.Helper()
	m := remappedModel(t, mutate)
	m.viewport.SetContent(strings.Repeat("row\n", 4*m.viewport.Height()))
	return m
}

// ---------------------------------------------------------------------------
// Normal-mode actions are configurable
// ---------------------------------------------------------------------------

// TestConfigurableNormalModeKeys rebinds every navigation/scroll action to a
// fresh key and proves both halves of the contract: the new key drives the
// action and the old default key no longer does.
func TestConfigurableNormalModeKeys(t *testing.T) {
	type position int
	const (
		atTop position = iota
		atBottom
	)
	cases := []struct {
		name   string
		mutate func(*config.KeyBindingsConfig)
		custom tea.KeyPressMsg
		legacy tea.KeyPressMsg // the default key that must stop working
		start  position
		want   string // "up", "down", "top" or "bottom"
	}{
		{
			name:   "scroll_up",
			mutate: func(kb *config.KeyBindingsConfig) { kb.ScrollUp = []string{"u"} },
			custom: tea.KeyPressMsg{Text: "u", Code: 'u'},
			legacy: tea.KeyPressMsg{Text: "k", Code: 'k'},
			start:  atBottom,
			want:   "up",
		},
		{
			name:   "scroll_down",
			mutate: func(kb *config.KeyBindingsConfig) { kb.ScrollDown = []string{"n"} },
			custom: tea.KeyPressMsg{Text: "n", Code: 'n'},
			legacy: tea.KeyPressMsg{Text: "j", Code: 'j'},
			start:  atTop,
			want:   "down",
		},
		{
			name:   "page_up",
			mutate: func(kb *config.KeyBindingsConfig) { kb.PageUp = []string{"1"} },
			custom: tea.KeyPressMsg{Text: "1", Code: '1'},
			legacy: tea.KeyPressMsg{Code: tea.KeyPgUp},
			start:  atBottom,
			want:   "up",
		},
		{
			name:   "page_down",
			mutate: func(kb *config.KeyBindingsConfig) { kb.PageDown = []string{"2"} },
			custom: tea.KeyPressMsg{Text: "2", Code: '2'},
			legacy: tea.KeyPressMsg{Code: tea.KeyPgDown},
			start:  atTop,
			want:   "down",
		},
		{
			name:   "goto_top",
			mutate: func(kb *config.KeyBindingsConfig) { kb.GotoTop = []string{"t"} },
			custom: tea.KeyPressMsg{Text: "t", Code: 't'},
			legacy: tea.KeyPressMsg{Text: "g", Code: 'g'},
			start:  atBottom,
			want:   "top",
		},
		{
			name:   "goto_bottom",
			mutate: func(kb *config.KeyBindingsConfig) { kb.GotoBottom = []string{"e"} },
			custom: tea.KeyPressMsg{Text: "e", Code: 'e'},
			legacy: tea.KeyPressMsg{Text: "G", Code: 'G'},
			start:  atTop,
			want:   "bottom",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bottom := bottomOffset(remappedScrollModel(t, tc.mutate))
			if bottom == 0 {
				t.Fatal("test history must be taller than the viewport")
			}
			startOffset := func() int {
				if tc.start == atBottom {
					return bottom
				}
				return 0
			}

			// Custom key performs the action.
			m := remappedScrollModel(t, tc.mutate)
			m.viewport.SetYOffset(startOffset())
			before := m.viewport.YOffset()
			m = pressKey(t, m, tc.custom)
			after := m.viewport.YOffset()
			switch tc.want {
			case "up":
				if after >= before {
					t.Errorf("custom key: YOffset = %d, want < %d", after, before)
				}
			case "down":
				if after <= before {
					t.Errorf("custom key: YOffset = %d, want > %d", after, before)
				}
			case "top":
				if after != 0 {
					t.Errorf("custom key: YOffset = %d, want 0", after)
				}
			case "bottom":
				if after != bottom {
					t.Errorf("custom key: YOffset = %d, want %d", after, bottom)
				}
			}

			// Legacy default key is inert after the remap.
			old := remappedScrollModel(t, tc.mutate)
			old.viewport.SetYOffset(startOffset())
			legacyBefore := old.viewport.YOffset()
			old = pressKey(t, old, tc.legacy)
			if got := old.viewport.YOffset(); got != legacyBefore {
				t.Errorf("legacy key still acts: YOffset = %d, want unchanged %d", got, legacyBefore)
			}
		})
	}
}

// TestConfigurableRetryKey rebinds retry and checks the legacy "r" stops
// retrying.
func TestConfigurableRetryKey(t *testing.T) {
	setup := func(m Model) Model {
		m.Error = "provider exploded"
		m.LastFailed = &core.TranslationRecord{
			ID: "1", Source: "hello", SourceLang: "auto", TargetLang: "zh-CN",
		}
		return m
	}

	m := setup(remappedModel(t, func(kb *config.KeyBindingsConfig) { kb.Retry = []string{"R"} }))
	r, cmd := m.Update(tea.KeyPressMsg{Text: "R", Code: 'R'})
	next := r.(Model)
	if !next.Loading {
		t.Error("custom retry key should start loading")
	}
	if next.Error != "" {
		t.Errorf("custom retry key should clear the error, got %q", next.Error)
	}
	if cmd == nil {
		t.Error("custom retry key should return the re-translate command")
	}

	old := setup(remappedModel(t, func(kb *config.KeyBindingsConfig) { kb.Retry = []string{"R"} }))
	r, cmd = old.Update(tea.KeyPressMsg{Text: "r", Code: 'r'})
	next = r.(Model)
	if next.Loading || cmd != nil {
		t.Error("legacy 'r' must be inert after retry was rebound")
	}
}

// TestConfigurableNavigationKeys rebinds previous/next record (the h/l
// actions) and checks the legacy keys stop navigating.
func TestConfigurableNavigationKeys(t *testing.T) {
	mutate := func(kb *config.KeyBindingsConfig) {
		kb.PreviousRecord = []string{"y"}
		kb.NextRecord = []string{"o"}
	}
	starts := func(m Model) []int {
		s := m.historyRecordStartRows()
		if len(s) < 3 {
			t.Fatalf("fixture needs >= 3 records, got %d starts", len(s))
		}
		return s
	}

	// Custom next ("o") moves to the following record.
	m := remappedModel(t, mutate)
	m.terminalHeight = 8
	m.Records = layoutRecords()[:3]
	m = m.refreshHistory(false)
	s := starts(m)
	m.viewport.SetYOffset(s[0])
	m = pressKey(t, m, tea.KeyPressMsg{Text: "o", Code: 'o'})
	if got := m.viewport.YOffset(); got != s[1] {
		t.Errorf("custom next: YOffset = %d, want %d (second record start)", got, s[1])
	}

	// Custom previous ("y") moves back.
	m = pressKey(t, m, tea.KeyPressMsg{Text: "y", Code: 'y'})
	if got := m.viewport.YOffset(); got != s[0] {
		t.Errorf("custom previous: YOffset = %d, want %d (first record start)", got, s[0])
	}

	// Legacy h/l are inert in normal mode after the remap.
	legacy := remappedModel(t, mutate)
	legacy.terminalHeight = 8
	legacy.Records = layoutRecords()[:3]
	legacy = legacy.refreshHistory(false)
	legacy.viewport.SetYOffset(s[0])
	legacy = pressKey(t, legacy, tea.KeyPressMsg{Text: "l", Code: 'l'})
	if got := legacy.viewport.YOffset(); got != s[0] {
		t.Errorf("legacy 'l' still navigates: YOffset = %d, want unchanged %d", got, s[0])
	}
	legacy = pressKey(t, legacy, tea.KeyPressMsg{Text: "h", Code: 'h'})
	if got := legacy.viewport.YOffset(); got != s[0] {
		t.Errorf("legacy 'h' still navigates: YOffset = %d, want unchanged %d", got, s[0])
	}
}

// ---------------------------------------------------------------------------
// Esc dual semantics
// ---------------------------------------------------------------------------

// TestEscDismissesErrorInsteadOfQuitting pins the default dual behavior:
// esc with a visible error dismisses it without quitting; esc without an
// error quits.
func TestEscDismissesErrorInsteadOfQuitting(t *testing.T) {
	m := newTestModel(t)
	m.Error = "boom"
	m = m.recalcViewportHeight()

	r, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	next := r.(Model)
	if next.Error != "" {
		t.Errorf("esc with error: Error = %q, want dismissed", next.Error)
	}
	assertNoQuit(t, cmd)

	clean := newTestModel(t)
	_, quitCmd := clean.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assertQuit(t, quitCmd)
}

// TestConfigurableDismissErrorKey documents the precedence when dismissal is
// rebound: the new key dismisses; esc (still bound to quit) quits even while
// the error is visible.
func TestConfigurableDismissErrorKey(t *testing.T) {
	mutate := func(kb *config.KeyBindingsConfig) { kb.DismissError = []string{"d"} }

	withError := func() Model {
		m := remappedModel(t, mutate)
		m.Error = "boom"
		return m.recalcViewportHeight()
	}

	// "d" dismisses, does not quit.
	r, cmd := withError().Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
	next := r.(Model)
	if next.Error != "" {
		t.Errorf("custom dismiss key: Error = %q, want dismissed", next.Error)
	}
	assertNoQuit(t, cmd)

	// esc no longer dismisses — it falls through to quit.
	r, cmd = withError().Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	next = r.(Model)
	if next.Error == "" {
		t.Error("esc must not dismiss once dismiss_error was rebound")
	}
	assertQuit(t, cmd)
}

// TestEscInInputModeCancelsInsteadOfQuitting pins input-mode esc: it leaves
// input mode through cancel_input and never returns quit, even though esc is
// also a default quit key.
func TestEscInInputModeCancelsInsteadOfQuitting(t *testing.T) {
	m := newTestModelWithInputMode(t)
	r, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	next := r.(Model)
	if next.inputMode {
		t.Error("esc should leave input mode")
	}
	assertNoQuit(t, cmd)
}

// ---------------------------------------------------------------------------
// Input-mode actions are configurable
// ---------------------------------------------------------------------------

// TestConfigurableCancelAndSubmitInputKeys rebinds cancel_input and
// submit_input, proving the new keys act and the legacy esc/enter do not.
func TestConfigurableCancelAndSubmitInputKeys(t *testing.T) {
	kb := config.DefaultKeyBindings()
	kb.CancelInput = []string{"c"}
	kb.SubmitInput = []string{"ctrl+j"}

	inputModel := func() Model {
		m := newTestModel(t)
		m.keyMap = NewKeyMapFromBindings(kb)
		next, _ := m.enterInputMode()
		next.textArea.SetValue("hello world")
		return next
	}

	// Custom cancel key leaves input mode.
	m := inputModel()
	r, cmd := m.Update(tea.KeyPressMsg{Text: "c", Code: 'c'})
	next := r.(Model)
	if next.inputMode {
		t.Error("custom cancel key should leave input mode")
	}
	assertNoQuit(t, cmd)

	// Legacy esc no longer cancels: input mode stays.
	m = inputModel()
	r, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	next = r.(Model)
	if !next.inputMode {
		t.Error("esc must stay in input mode after cancel_input was rebound")
	}

	// Custom submit key sends the text.
	m = inputModel()
	r, cmd = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	next = r.(Model)
	if next.inputMode {
		t.Error("custom submit key should leave input mode")
	}
	if !next.Loading {
		t.Error("custom submit key should start loading")
	}
	if cmd == nil {
		t.Fatal("custom submit key should return the translate command")
	}

	// Legacy enter no longer submits: it falls through to the textarea and
	// inserts a newline instead.
	m = inputModel()
	r, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	next = r.(Model)
	if !next.inputMode || next.Loading {
		t.Errorf("enter after remap: inputMode=%v Loading=%v, want still typing",
			next.inputMode, next.Loading)
	}
	if !strings.Contains(next.textArea.Value(), "\n") {
		t.Errorf("enter after remap should insert a newline, value = %q", next.textArea.Value())
	}
	assertNoQuit(t, cmd)
}

// TestNavigationKeysStillTypeInInputMode pins that remapping previous/next
// record never leaks into the textarea: h and l remain ordinary characters.
func TestNavigationKeysStillTypeInInputMode(t *testing.T) {
	m := remappedModel(t, func(kb *config.KeyBindingsConfig) {
		kb.PreviousRecord = []string{"y"}
		kb.NextRecord = []string{"o"}
	})
	next, _ := m.enterInputMode()
	m = next

	for _, k := range []tea.KeyPressMsg{
		{Text: "h", Code: 'h'},
		{Text: "l", Code: 'l'},
		{Text: "y", Code: 'y'},
	} {
		r, _ := m.Update(k)
		m = r.(Model)
	}
	if got := m.textArea.Value(); got != "hly" {
		t.Errorf("textarea value = %q, want %q (app keys must type)", got, "hly")
	}
	if !m.inputMode {
		t.Error("typing must not leave input mode")
	}
}

// TestConfigurableCopySelectionKey proves copy_selection flows through New
// into the textarea's own keymap: the custom key copies via the app's
// clipboard abstraction, the legacy ctrl+shift+c no longer does.
func TestConfigurableCopySelectionKey(t *testing.T) {
	kb := config.DefaultKeyBindings()
	kb.CopySelection = []string{"ctrl+y"}

	rec := &clipboardRecorder{}
	m := New(core.AppState{}, &core.Service{}, "", "auto", "auto", true, false,
		NewKeyMapFromBindings(kb), DefaultTheme())
	m.clipboard = rec.write

	if got := m.textArea.KeyMap.CopySelection.Keys(); len(got) != 1 || got[0] != "ctrl+y" {
		t.Fatalf("textarea CopySelection keys = %v, want [ctrl+y]", got)
	}
	if !m.inputMode {
		t.Fatal("inputInitial should start in input mode")
	}

	m.textArea.SetValue("hello world")
	m.textArea.BeginSelection(0, 0)
	m.textArea.ExtendSelection(5, 0)
	m.textArea.EndSelection()
	if !m.textArea.HasSelection() {
		t.Fatal("fixture must produce a textarea selection")
	}
	want := m.textArea.SelectedText()
	if want == "" {
		t.Fatal("fixture selection must not be empty")
	}

	// Custom key copies through the injected clipboard writer.
	r, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	m = r.(Model)
	if cmd == nil {
		t.Fatal("custom copy key should return a clipboard command")
	}
	_ = cmd()
	if len(rec.writes) != 1 || rec.writes[0] != want {
		t.Fatalf("clipboard writes = %v, want [%q]", rec.writes, want)
	}

	// Legacy ctrl+shift+c no longer copies (the textarea binding moved too).
	r, cmd = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl | tea.ModShift})
	m = r.(Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			t.Errorf("legacy copy key produced a clipboard message: %v", msg)
		}
	}
	if len(rec.writes) != 1 {
		t.Errorf("legacy copy key still wrote to the clipboard: %v", rec.writes)
	}
}

// ---------------------------------------------------------------------------
// Partial config → other actions keep their defaults
// ---------------------------------------------------------------------------

// TestPartialKeyMapKeepsOtherActionDefaults pins that building the keymap
// from a config that only rebinds quit leaves every other action at its
// default keys.
func TestPartialKeyMapKeepsOtherActionDefaults(t *testing.T) {
	km := NewKeyMapFromBindings(config.KeyBindingsConfig{Quit: []string{"Q"}})
	defaults := config.DefaultKeyBindings()

	if got := km.Quit.Keys(); len(got) != 1 || got[0] != "Q" {
		t.Errorf("quit keys = %v, want [Q]", got)
	}
	checks := []struct {
		name string
		got  []string
		want []string
	}{
		{"manual_input", km.ManualInput.Keys(), defaults.ManualInput},
		{"scroll_up", km.ScrollUp.Keys(), defaults.ScrollUp},
		{"page_down", km.PageDown.Keys(), defaults.PageDown},
		{"goto_top", km.GotoTop.Keys(), defaults.GotoTop},
		{"previous_record", km.PreviousRecord.Keys(), defaults.PreviousRecord},
		{"next_record", km.NextRecord.Keys(), defaults.NextRecord},
		{"retry", km.Retry.Keys(), defaults.Retry},
		{"dismiss_error", km.DismissError.Keys(), defaults.DismissError},
		{"cancel_input", km.CancelInput.Keys(), defaults.CancelInput},
		{"submit_input", km.SubmitInput.Keys(), defaults.SubmitInput},
		{"copy_selection", km.CopySelection.Keys(), defaults.CopySelection},
	}
	for _, c := range checks {
		if strings.Join(c.got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s keys = %v, want default %v", c.name, c.got, c.want)
		}
	}
}
