package config

import (
	"strings"
	"testing"
)

// --- Appearance defaults ----------------------------------------------------

// TestDefaultAppearanceMatchesOriginalStyles pins every [appearance] default
// to the values the TUI hard-coded before the section existed. A config file
// without [appearance] must render exactly like the original program.
func TestDefaultAppearanceMatchesOriginalStyles(t *testing.T) {
	a := DefaultAppearance()

	if a.TransparentBackground {
		t.Error("transparent_background default = true, want false")
	}

	header := a.Header
	if !header.Enabled {
		t.Error("header.enabled default = false, want true")
	}
	if header.Title != "trans-tui" {
		t.Errorf("header.title = %q, want %q", header.Title, "trans-tui")
	}
	if !header.ShowRecordCount {
		t.Error("header.show_record_count default = false, want true")
	}
	if header.Foreground != "229" {
		t.Errorf("header.foreground = %q, want %q", header.Foreground, "229")
	}
	if header.Background != "" {
		t.Errorf("header.background = %q, want empty (original header has no background)", header.Background)
	}
	if !header.Bold {
		t.Error("header.bold default = false, want true")
	}
	if header.PaddingLeft != 1 || header.PaddingRight != 1 {
		t.Errorf("header padding = (%d,%d), want (1,1)", header.PaddingLeft, header.PaddingRight)
	}

	sb := a.StatusBar
	if !sb.Enabled {
		t.Error("status_bar.enabled default = false, want true")
	}
	for _, f := range []struct {
		name string
		got  bool
	}{
		{"show_record_count", sb.ShowRecordCount},
		{"show_scroll_position", sb.ShowScrollPosition},
		{"show_input_hint", sb.ShowInputHint},
		{"show_quit_hint", sb.ShowQuitHint},
		{"show_cancel_hint", sb.ShowCancelHint},
	} {
		if !f.got {
			t.Errorf("status_bar.%s default = false, want true", f.name)
		}
	}
	if sb.Foreground != "241" {
		t.Errorf("status_bar.foreground = %q, want %q", sb.Foreground, "241")
	}
	if sb.Background != "235" {
		t.Errorf("status_bar.background = %q, want %q", sb.Background, "235")
	}
	if sb.PaddingLeft != 1 || sb.PaddingRight != 1 {
		t.Errorf("status_bar padding = (%d,%d), want (1,1)", sb.PaddingLeft, sb.PaddingRight)
	}

	if a.Record.BorderForeground != "62" {
		t.Errorf("record.border_foreground = %q, want %q", a.Record.BorderForeground, "62")
	}
	if a.Record.Background != "" {
		t.Errorf("record.background = %q, want empty (original cards have no background)", a.Record.Background)
	}
	if a.Source.Foreground != "241" || !a.Source.Bold {
		t.Errorf("source = (%q, bold=%v), want (%q, bold=true)", a.Source.Foreground, a.Source.Bold, "241")
	}
	if a.Translation.Foreground != "86" {
		t.Errorf("translation.foreground = %q, want %q", a.Translation.Foreground, "86")
	}
	if a.Error.Foreground != "196" || !a.Error.Bold {
		t.Errorf("error = (%q, bold=%v), want (%q, bold=true)", a.Error.Foreground, a.Error.Bold, "196")
	}
	if a.ErrorPanel.BorderForeground != "196" || a.ErrorPanel.Foreground != "196" || a.ErrorPanel.Background != "" {
		t.Errorf("error_panel = (%q,%q,%q), want (%q,%q,empty)",
			a.ErrorPanel.BorderForeground, a.ErrorPanel.Foreground, a.ErrorPanel.Background, "196", "196")
	}
	if a.Loading.Foreground != "205" || !a.Loading.Bold || a.Loading.Text != "Translating..." {
		t.Errorf("loading = (%q, bold=%v, %q), want (%q, bold=true, %q)",
			a.Loading.Foreground, a.Loading.Bold, a.Loading.Text, "205", "Translating...")
	}
	if a.Input.BorderForeground != "62" || a.Input.Background != "" {
		t.Errorf("input = (%q,%q), want (%q,empty)", a.Input.BorderForeground, a.Input.Background, "62")
	}
	if a.Input.PromptForeground != "86" || !a.Input.PromptBold {
		t.Errorf("input prompt = (%q, bold=%v), want (%q, bold=true)", a.Input.PromptForeground, a.Input.PromptBold, "86")
	}
	if a.Selection.Background != "240" || a.Selection.Foreground != "15" {
		t.Errorf("selection = (%q,%q), want (%q,%q)", a.Selection.Background, a.Selection.Foreground, "240", "15")
	}
}

// --- Key binding defaults ---------------------------------------------------

// TestDefaultKeyBindingsPinAllActions pins all 15 configurable actions to
// their documented defaults.
func TestDefaultKeyBindingsPinAllActions(t *testing.T) {
	kb := DefaultKeyBindings()
	want := map[string][]string{
		"quit":            {"q", "ctrl+c", "esc"},
		"manual_input":    {",", "，"},
		"scroll_up":       {"up", "k"},
		"scroll_down":     {"down", "j"},
		"page_up":         {"pgup", "b"},
		"page_down":       {"pgdown", "f"},
		"goto_top":        {"home", "g"},
		"goto_bottom":     {"end", "G"},
		"previous_record": {"h"},
		"next_record":     {"l"},
		"retry":           {"r"},
		"dismiss_error":   {"esc"},
		"cancel_input":    {"esc"},
		"submit_input":    {"enter"},
		"copy_selection":  {"ctrl+shift+c"},
	}

	got := map[string][]string{
		"quit":            kb.Quit,
		"manual_input":    kb.ManualInput,
		"scroll_up":       kb.ScrollUp,
		"scroll_down":     kb.ScrollDown,
		"page_up":         kb.PageUp,
		"page_down":       kb.PageDown,
		"goto_top":        kb.GotoTop,
		"goto_bottom":     kb.GotoBottom,
		"previous_record": kb.PreviousRecord,
		"next_record":     kb.NextRecord,
		"retry":           kb.Retry,
		"dismiss_error":   kb.DismissError,
		"cancel_input":    kb.CancelInput,
		"submit_input":    kb.SubmitInput,
		"copy_selection":  kb.CopySelection,
	}

	if len(got) != 15 {
		t.Fatalf("configurable actions = %d, want 15", len(got))
	}
	for name, wantKeys := range want {
		gotKeys := got[name]
		if len(gotKeys) != len(wantKeys) {
			t.Errorf("%s = %v, want %v", name, gotKeys, wantKeys)
			continue
		}
		for i := range wantKeys {
			if gotKeys[i] != wantKeys[i] {
				t.Errorf("%s = %v, want %v", name, gotKeys, wantKeys)
				break
			}
		}
	}
}

// --- Loading: omitted and partial sections ----------------------------------

func TestLoadWithoutAppearanceSectionKeepsDefaults(t *testing.T) {
	path := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Appearance != DefaultAppearance() {
		t.Errorf("appearance without [appearance] section = %+v, want defaults", cfg.Appearance)
	}
}

func TestLoadPartialAppearanceMergesOverDefaults(t *testing.T) {
	path := writeTempConfig(t, `[provider]
type = "openai-compatible"
api_key_env = "TEST_KEY"

[appearance]
transparent_background = true

[appearance.header]
enabled = false

[appearance.status_bar]
foreground = "250"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Appearance.TransparentBackground {
		t.Error("explicit transparent_background = false, want true")
	}
	if cfg.Appearance.Header.Enabled {
		t.Error("explicit header.enabled = true, want false")
	}
	if cfg.Appearance.StatusBar.Foreground != "250" {
		t.Errorf("explicit status_bar.foreground = %q, want %q", cfg.Appearance.StatusBar.Foreground, "250")
	}

	// Untouched fields keep their defaults.
	if cfg.Appearance.Header.Title != "trans-tui" {
		t.Errorf("untouched header.title = %q, want %q (default)", cfg.Appearance.Header.Title, "trans-tui")
	}
	if cfg.Appearance.StatusBar.Background != "235" {
		t.Errorf("untouched status_bar.background = %q, want %q (default)", cfg.Appearance.StatusBar.Background, "235")
	}
	if cfg.Appearance.Record.BorderForeground != "62" {
		t.Errorf("untouched record.border_foreground = %q, want %q (default)", cfg.Appearance.Record.BorderForeground, "62")
	}
	if cfg.Appearance.Translation.Foreground != "86" {
		t.Errorf("untouched translation.foreground = %q, want %q (default)", cfg.Appearance.Translation.Foreground, "86")
	}
}

func TestLoadPartialKeybindingsFallsBackToDefaults(t *testing.T) {
	path := writeTempConfig(t, `[provider]
type = "openai-compatible"
api_key_env = "TEST_KEY"

[keybindings]
quit = ["x"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.KeyBindings.Quit) != 1 || cfg.KeyBindings.Quit[0] != "x" {
		t.Errorf("quit = %v, want [x]", cfg.KeyBindings.Quit)
	}

	defaults := DefaultKeyBindings()
	// ResolveKeyBindings is what the runtime hands to the TUI: the omitted
	// actions must come back as their defaults.
	resolved := cfg.ResolveKeyBindings()
	if len(resolved.ManualInput) != len(defaults.ManualInput) || resolved.ManualInput[0] != defaults.ManualInput[0] {
		t.Errorf("manual_input resolved = %v, want default %v", resolved.ManualInput, defaults.ManualInput)
	}
	if resolved.ScrollUp[0] != "up" {
		t.Errorf("scroll_up resolved = %v, want default %v", resolved.ScrollUp, defaults.ScrollUp)
	}
	if resolved.CopySelection[0] != "ctrl+shift+c" {
		t.Errorf("copy_selection resolved = %v, want default %v", resolved.CopySelection, defaults.CopySelection)
	}
	if resolved.PreviousRecord[0] != "h" {
		t.Errorf("previous_record resolved = %v, want default %v", resolved.PreviousRecord, defaults.PreviousRecord)
	}
}

// --- Validation -------------------------------------------------------------

// TestKeyBindingsValidateRejectsEachEmptyAction proves that every one of the
// 15 actions rejects an empty binding list, with the action named in the
// error, while cross-action overlaps stay legal.
func TestKeyBindingsValidateRejectsEachEmptyAction(t *testing.T) {
	cases := []struct {
		name  string
		clear func(*KeyBindingsConfig)
	}{
		{"quit", func(kb *KeyBindingsConfig) { kb.Quit = nil }},
		{"manual_input", func(kb *KeyBindingsConfig) { kb.ManualInput = nil }},
		{"scroll_up", func(kb *KeyBindingsConfig) { kb.ScrollUp = []string{} }},
		{"scroll_down", func(kb *KeyBindingsConfig) { kb.ScrollDown = nil }},
		{"page_up", func(kb *KeyBindingsConfig) { kb.PageUp = nil }},
		{"page_down", func(kb *KeyBindingsConfig) { kb.PageDown = nil }},
		{"goto_top", func(kb *KeyBindingsConfig) { kb.GotoTop = nil }},
		{"goto_bottom", func(kb *KeyBindingsConfig) { kb.GotoBottom = nil }},
		{"previous_record", func(kb *KeyBindingsConfig) { kb.PreviousRecord = nil }},
		{"next_record", func(kb *KeyBindingsConfig) { kb.NextRecord = nil }},
		{"retry", func(kb *KeyBindingsConfig) { kb.Retry = nil }},
		{"dismiss_error", func(kb *KeyBindingsConfig) { kb.DismissError = nil }},
		{"cancel_input", func(kb *KeyBindingsConfig) { kb.CancelInput = nil }},
		{"submit_input", func(kb *KeyBindingsConfig) { kb.SubmitInput = nil }},
		{"copy_selection", func(kb *KeyBindingsConfig) { kb.CopySelection = nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kb := DefaultKeyBindings()
			tc.clear(&kb)
			err := kb.Validate()
			if err == nil {
				t.Fatalf("Validate() with empty %s = nil, want error", tc.name)
			}
			if want := "keybindings." + tc.name; !strings.Contains(err.Error(), want) {
				t.Errorf("Validate() error = %q, want it to mention %q", err, want)
			}
		})
	}
}

// TestKeyBindingsValidateAllowsCrossActionOverlap documents that overlapping
// keys (esc on quit/dismiss/cancel, for example) are legal: the TUI resolves
// them by dispatch precedence, not by rejecting the config.
func TestKeyBindingsValidateAllowsCrossActionOverlap(t *testing.T) {
	kb := DefaultKeyBindings() // esc already on quit + dismiss_error + cancel_input
	if err := kb.Validate(); err != nil {
		t.Errorf("Validate() with overlapping esc bindings = %v, want nil", err)
	}
}

// --- Fingerprint ------------------------------------------------------------

// TestFingerprintCoversAppearanceAndKeyBindings pins the fingerprint rules:
// same config → same hash; appearance or keybinding changes → different hash;
// translation languages → same hash; provider changes → different hash.
func TestFingerprintCoversAppearanceAndKeyBindings(t *testing.T) {
	base := DefaultConfig()

	t.Run("same config has same fingerprint", func(t *testing.T) {
		if base.Fingerprint() != DefaultConfig().Fingerprint() {
			t.Error("two identical default configs have different fingerprints")
		}
	})

	t.Run("appearance difference changes fingerprint", func(t *testing.T) {
		other := DefaultConfig()
		other.Appearance.Header.Enabled = false
		if base.Fingerprint() == other.Fingerprint() {
			t.Error("appearance change did not change the fingerprint")
		}
	})

	t.Run("transparent background changes fingerprint", func(t *testing.T) {
		other := DefaultConfig()
		other.Appearance.TransparentBackground = true
		if base.Fingerprint() == other.Fingerprint() {
			t.Error("transparent_background change did not change the fingerprint")
		}
	})

	t.Run("keybinding difference changes fingerprint", func(t *testing.T) {
		other := DefaultConfig()
		other.KeyBindings.Quit = []string{"Q"}
		if base.Fingerprint() == other.Fingerprint() {
			t.Error("keybinding change did not change the fingerprint")
		}
	})

	t.Run("translation languages do not change fingerprint", func(t *testing.T) {
		other := DefaultConfig()
		other.Translation.SourceLang = "ja"
		other.Translation.TargetLang = "en"
		if base.Fingerprint() != other.Fingerprint() {
			t.Error("translation language change changed the fingerprint")
		}
	})

	t.Run("provider difference changes fingerprint", func(t *testing.T) {
		other := DefaultConfig()
		other.Provider.OpenAI.Model = "other-model"
		if base.Fingerprint() == other.Fingerprint() {
			t.Error("provider change did not change the fingerprint")
		}
	})
}

// TestCreatedDefaultConfigRoundTripsNewSections loads the auto-created
// template and checks that its active [appearance] and [keybindings] sections
// decode back to exactly the defaults they document.
func TestCreatedDefaultConfigRoundTripsNewSections(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Appearance != DefaultAppearance() {
		t.Errorf("template appearance = %+v, want defaults", cfg.Appearance)
	}
	if err := cfg.KeyBindings.Validate(); err != nil {
		t.Errorf("template keybindings invalid: %v", err)
	}
	if got := cfg.ResolveKeyBindings(); got.Quit[0] != "q" || got.CopySelection[0] != "ctrl+shift+c" {
		t.Errorf("template keybindings resolve to %v / %v, want defaults", got.Quit, got.CopySelection)
	}
}
