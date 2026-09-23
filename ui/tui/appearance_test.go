package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
)

// ---------------------------------------------------------------------------
// Fixtures: models built through the production layout path.
// ---------------------------------------------------------------------------

// themeWith builds a default theme with the given [appearance] mutations.
func themeWith(mutate func(*config.AppearanceConfig)) Theme {
	a := config.DefaultAppearance()
	mutate(&a)
	return NewTheme(a)
}

// newAppearanceModel builds a model through the production WindowSizeMsg
// handler so viewport creation, semantic mapping and section heights all come
// from the real layout code. The returned model is ready and laid out.
func newAppearanceModel(t *testing.T, theme Theme, width, height int) Model {
	t.Helper()
	m := Model{
		sourceLang: "auto",
		targetLang: "auto",
		textArea:   newInputTextArea(theme),
		keyMap:     DefaultKeyMap(),
		theme:      theme,
	}
	r, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	next, ok := r.(Model)
	if !ok {
		t.Fatalf("Update(WindowSizeMsg) returned %T, want Model", r)
	}
	return next
}

// layoutRecords mixes long, CJK and wrapped records plus a failed record so
// the viewport renders every card shape while the layout is asserted.
func layoutRecords() []core.TranslationRecord {
	return []core.TranslationRecord{
		{SourceLang: "en", TargetLang: "ja", Source: "Hello world", Translation: "こんにちは世界"},
		{SourceLang: "zh", TargetLang: "en",
			Source:      strings.Repeat("这是一段很长的中文文本用于测试换行是否正确", 8),
			Translation: strings.Repeat("long english translation text ", 8)},
		{SourceLang: "en", TargetLang: "ja",
			Source:      strings.Repeat("wrap-me-", 40),
			Translation: strings.Repeat("long-token-", 40),
			Provider:    "deepseek", Model: "deepseek-flash"},
		{SourceLang: "en", Source: "failed request", Error: "network timeout"},
	}
}

func withLayoutRecords(m Model) Model {
	m.Records = append(m.Records, layoutRecords()...)
	return m.refreshHistory(false)
}

func assertRenderHeight(t *testing.T, m Model, context string) {
	t.Helper()
	if got, want := lipgloss.Height(m.renderView()), m.terminalHeight; got != want {
		t.Errorf("%s: renderView height = %d, want terminalHeight %d", context, got, want)
	}
}

// ---------------------------------------------------------------------------
// Section toggles
// ---------------------------------------------------------------------------

func TestDefaultLayoutKeepsHeaderAndStatusBar(t *testing.T) {
	m := newAppearanceModel(t, DefaultTheme(), 80, 24)

	if got := m.headerHeight(); got != 1 {
		t.Errorf("headerHeight() = %d, want 1", got)
	}
	if got := m.statusBarHeight(); got != 1 {
		t.Errorf("statusBarHeight() = %d, want 1", got)
	}
	if got := m.viewport.Height(); got != 22 {
		t.Errorf("viewport height = %d, want 22 (24 - header - status bar)", got)
	}
	view := m.renderView()
	if !strings.Contains(firstLine(view), "trans-tui") {
		t.Errorf("default view should start with the header title, got %q", firstLine(view))
	}
	if !strings.Contains(view, "Scroll:") {
		t.Error("default view should contain the status bar scroll indicator")
	}
	assertRenderHeight(t, m, "default layout")
}

func TestHeaderDisabledCollapsesLayout(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.Enabled = false
	}), 80, 24)

	if got := m.headerHeight(); got != 0 {
		t.Errorf("headerHeight() = %d, want 0 when header disabled", got)
	}
	if got := m.statusBarHeight(); got != 1 {
		t.Errorf("statusBarHeight() = %d, want 1 (status bar still enabled)", got)
	}
	if got := m.viewport.Height(); got != 23 {
		t.Errorf("viewport height = %d, want 23 (grew into the missing header row)", got)
	}
	view := m.renderView()
	if strings.Contains(firstLine(view), "trans-tui") {
		t.Errorf("disabled header still renders its title: %q", firstLine(view))
	}
	if !strings.Contains(view, "Scroll:") {
		t.Error("status bar should still render when only the header is disabled")
	}
	assertRenderHeight(t, m, "header disabled")
}

func TestStatusBarDisabledGrowsViewport(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.StatusBar.Enabled = false
	}), 80, 24)

	if got := m.statusBarHeight(); got != 0 {
		t.Errorf("statusBarHeight() = %d, want 0 when status bar disabled", got)
	}
	if got := m.headerHeight(); got != 1 {
		t.Errorf("headerHeight() = %d, want 1 (header still enabled)", got)
	}
	if got := m.viewport.Height(); got != 23 {
		t.Errorf("viewport height = %d, want 23 (grew into the missing status row)", got)
	}
	if strings.Contains(m.renderView(), "Scroll:") {
		t.Error("disabled status bar still renders its scroll indicator")
	}
	assertRenderHeight(t, m, "status bar disabled")
}

func TestBothSectionsDisabled(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.Enabled = false
		a.StatusBar.Enabled = false
	}), 80, 24)

	if m.headerHeight() != 0 || m.statusBarHeight() != 0 {
		t.Errorf("heights = (%d,%d), want (0,0)", m.headerHeight(), m.statusBarHeight())
	}
	if got := m.viewport.Height(); got != 24 {
		t.Errorf("viewport height = %d, want 24 (viewport owns the whole terminal)", got)
	}
	view := m.renderView()
	if strings.Contains(firstLine(view), "trans-tui") {
		t.Error("header title rendered while header disabled")
	}
	if strings.Contains(view, "Scroll:") {
		t.Error("status bar rendered while status bar disabled")
	}
	assertRenderHeight(t, m, "both sections disabled")
}

// ---------------------------------------------------------------------------
// Height invariant with sections disabled
// ---------------------------------------------------------------------------

// TestHeightInvariantWithSectionsDisabled walks loading/error/input states
// over short, wide and tall terminals with each section-toggle combination,
// including history full of wrapped/CJK records. The render must always fill
// exactly the terminal height.
func TestHeightInvariantWithSectionsDisabled(t *testing.T) {
	type appearanceCase struct {
		name   string
		mutate func(*config.AppearanceConfig)
	}
	themes := []appearanceCase{
		{"default", func(*config.AppearanceConfig) {}},
		{"header off", func(a *config.AppearanceConfig) { a.Header.Enabled = false }},
		{"status off", func(a *config.AppearanceConfig) { a.StatusBar.Enabled = false }},
		{"both off", func(a *config.AppearanceConfig) {
			a.Header.Enabled = false
			a.StatusBar.Enabled = false
		}},
	}
	sizes := []struct {
		name          string
		width, height int
	}{
		{"standard", 80, 24},
		{"medium", 40, 12},
		{"short", 24, 10},
		{"tall", 100, 40},
	}
	states := []struct {
		name  string
		apply func(t *testing.T, m Model) Model
	}{
		{"idle", func(t *testing.T, m Model) Model { return m }},
		{"loading", func(t *testing.T, m Model) Model {
			m.Loading = true
			return m.recalcViewportHeight()
		}},
		{"error", func(t *testing.T, m Model) Model {
			m.Error = "boom"
			return m.recalcViewportHeight()
		}},
		{"loading and error", func(t *testing.T, m Model) Model {
			m.Loading = true
			m.Error = "boom"
			return m.recalcViewportHeight()
		}},
		{"input", func(t *testing.T, m Model) Model {
			next, _ := m.enterInputMode()
			return next
		}},
		{"input with error", func(t *testing.T, m Model) Model {
			m.Error = "boom"
			m = m.recalcViewportHeight()
			next, _ := m.enterInputMode()
			return next
		}},
		{"input loading and error", func(t *testing.T, m Model) Model {
			m.Loading = true
			m.Error = "boom"
			m = m.recalcViewportHeight()
			next, _ := m.enterInputMode()
			return next
		}},
	}

	for _, th := range themes {
		for _, sz := range sizes {
			for _, st := range states {
				t.Run(th.name+"/"+sz.name+"/"+st.name, func(t *testing.T) {
					m := newAppearanceModel(t, themeWith(th.mutate), sz.width, sz.height)
					m = withLayoutRecords(m)
					m = st.apply(t, m)
					assertRenderHeight(t, m, th.name+"/"+sz.name+"/"+st.name)
				})
			}
		}
	}
}

func TestHeightInvariantWithSectionsDisabledAfterResize(t *testing.T) {
	resizes := []tea.WindowSizeMsg{
		{Width: 100, Height: 30},
		{Width: 24, Height: 10},
		{Width: 60, Height: 18},
		{Width: 200, Height: 50},
	}

	modes := []struct {
		name  string
		input bool
	}{
		{"normal mode", false},
		{"input mode", true},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
				a.Header.Enabled = false
				a.StatusBar.Enabled = false
			}), 80, 24)
			m = withLayoutRecords(m)
			if mode.input {
				next, _ := m.enterInputMode()
				m = next
			}
			for _, size := range resizes {
				r, _ := m.Update(size)
				m = r.(Model)
				assertRenderHeight(t, m, fmt.Sprintf("resize to %dx%d", size.Width, size.Height))
			}
		})
	}
}

// TestErrorDismissKeepsInvariantWithSectionsDisabled dismisses through the
// production handler with both sections off.
func TestErrorDismissKeepsInvariantWithSectionsDisabled(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.Enabled = false
		a.StatusBar.Enabled = false
	}), 80, 24)
	m = withLayoutRecords(m)
	m.Error = "provider exploded"
	m = m.recalcViewportHeight()
	assertRenderHeight(t, m, "with error")

	next, _ := m.handleDismissError()
	m = next
	if m.Error != "" {
		t.Fatalf("Error = %q, want dismissed", m.Error)
	}
	assertRenderHeight(t, m, "after dismiss")
}

// ---------------------------------------------------------------------------
// Coordinates adapt to disabled sections
// ---------------------------------------------------------------------------

// TestHeaderDisabledAdaptsSelectionCoordinates pins that screen → content
// mapping shifts up with the missing header row: with a header, screen row 2
// is the first record's source line; without one, the same line is row 1 and
// row 0 is the card's border.
func TestHeaderDisabledAdaptsSelectionCoordinates(t *testing.T) {
	build := func(theme Theme) Model {
		m := newAppearanceModel(t, theme, 80, 24)
		m.Records = append(m.Records, core.TranslationRecord{
			SourceLang: "en", TargetLang: "ja", Source: "Hello World", Translation: "こんにちは世界",
		})
		return m.refreshHistory(false)
	}

	withHeader := build(DefaultTheme())
	withoutHeader := build(themeWith(func(a *config.AppearanceConfig) {
		a.Header.Enabled = false
	}))

	// Default: y=0 header, y=1 card border (not selectable), y=2 source.
	if pt := withHeader.screenToSelectionPoint(4, 2); pt == nil {
		t.Error("default: source row should accept selection at y=2")
	}
	if pt := withHeader.screenToSelectionPoint(4, 1); pt != nil {
		t.Errorf("default: card border at y=1 should reject selection, got %+v", pt)
	}

	// Header disabled: everything shifts up by one row.
	if pt := withoutHeader.screenToSelectionPoint(4, 1); pt == nil {
		t.Error("header disabled: source row should accept selection at y=1")
	}
	if pt := withoutHeader.screenToSelectionPoint(4, 0); pt != nil {
		t.Errorf("header disabled: card border at y=0 should reject selection, got %+v", pt)
	}

	// Mouse hit-testing follows the same shift.
	if withHeader.mouseInHistoryViewport(tea.Mouse{X: 4, Y: 0}) {
		t.Error("default: y=0 is the header, not the viewport")
	}
	if !withoutHeader.mouseInHistoryViewport(tea.Mouse{X: 4, Y: 0}) {
		t.Error("header disabled: y=0 should be inside the viewport")
	}
}

// TestHeaderDisabledAdaptsInputOrigin pins that the textarea/cursor origin
// loses exactly the header row.
func TestHeaderDisabledAdaptsInputOrigin(t *testing.T) {
	theme := themeWith(func(a *config.AppearanceConfig) {
		a.Header.Enabled = false
		a.StatusBar.Enabled = false
	})
	m := newAppearanceModel(t, theme, 80, 24)
	next, _ := m.enterInputMode()
	m = next

	_, y := m.inputTextAreaOrigin()
	// y = headerHeight(0) + viewport.Height + input panel top frame(1)
	if want := 0 + m.viewport.Height() + 1; y != want {
		t.Errorf("inputTextAreaOrigin y = %d, want %d (no header row)", y, want)
	}
	if want := m.viewport.Height() + m.inputPanelHeight(); want != 24 {
		t.Errorf("viewport+input panel = %d, want 24 (fills header-less terminal)", want)
	}
}

// ---------------------------------------------------------------------------
// Transparency
// ---------------------------------------------------------------------------

// TestTransparentBackgroundDropsSurfaceBackgrounds is the core transparency
// assertion: with transparent_background the rendered view must contain no
// background color sequences at all (status bar, record card, error panel,
// input panel), while foregrounds and borders keep their colors and the
// rendered dimensions stay byte-for-byte the same size.
func TestTransparentBackgroundDropsSurfaceBackgrounds(t *testing.T) {
	appearance := func(transparent bool) Theme {
		return themeWith(func(a *config.AppearanceConfig) {
			a.TransparentBackground = transparent
			// Give the record card and error panel backgrounds so the
			// assertion covers surfaces that are background-less by default.
			a.Record.Background = "17"
			a.ErrorPanel.Background = "52"
			a.Input.Background = "22"
		})
	}

	build := func(theme Theme) Model {
		m := newAppearanceModel(t, theme, 80, 24)
		m = withLayoutRecords(m)
		m.Error = "boom"
		m = m.recalcViewportHeight()
		next, _ := m.enterInputMode()
		return next
	}

	opaque := build(appearance(false))
	transparent := build(appearance(true))
	oView, tView := opaque.renderView(), transparent.renderView()

	// The opaque render must actually paint these, otherwise the negative
	// assertion below would pass vacuously.
	backgrounds := map[string]string{
		"48;5;235": "status bar background (default 235)",
		"48;5;17":  "record card background",
		"48;5;52":  "error panel background",
		"48;5;22":  "input panel background",
	}
	for seq, what := range backgrounds {
		if !strings.Contains(oView, seq) {
			t.Fatalf("opaque view missing %s (escape %q)", what, seq)
		}
		if strings.Contains(tView, seq) {
			t.Errorf("transparent view still paints %s (escape %q)", what, seq)
		}
	}

	// Foregrounds and borders survive transparency.
	foregrounds := map[string]string{
		"38;5;241": "status bar foreground",
		"38;5;62":  "record border foreground",
		"38;5;196": "error foreground",
	}
	for seq, what := range foregrounds {
		if !strings.Contains(tView, seq) {
			t.Errorf("transparent view lost %s (escape %q)", what, seq)
		}
	}

	// Transparency must not change the geometry: same lines, same width.
	if oh, th_ := lipgloss.Height(oView), lipgloss.Height(tView); oh != th_ {
		t.Errorf("height changed by transparency: opaque %d, transparent %d", oh, th_)
	}
	if ow, tw := lipgloss.Width(oView), lipgloss.Width(tView); ow != tw {
		t.Errorf("width changed by transparency: opaque %d, transparent %d", ow, tw)
	}
	if got := lipgloss.Height(tView); got != 24 {
		t.Errorf("transparent render height = %d, want 24", got)
	}
}

// TestTransparentBackgroundKeepsSelectionHighlight pins that the selection
// highlight keeps its background even when application surfaces go
// transparent — it marks the user's own selection, not a surface.
func TestTransparentBackgroundKeepsSelectionHighlight(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.TransparentBackground = true
		a.Record.Background = "17"
	}), 80, 24)
	m.Records = append(m.Records, core.TranslationRecord{
		SourceLang: "en", TargetLang: "ja", Source: "Hello World", Translation: "こんにちは世界",
	})
	m = m.refreshHistory(false)

	// Select part of the source line (semRows: border, source, translation,
	// border — source is row 1).
	m.sel = selection{
		start: SelectionPoint{VisualRow: 1, CellCol: 0},
		end:   SelectionPoint{VisualRow: 1, CellCol: 5},
	}
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	view := m.renderView()

	if !strings.Contains(view, "48;5;240") {
		t.Error("selection highlight background missing from transparent render")
	}
	if strings.Contains(view, "48;5;17") {
		t.Error("record card background should be dropped while transparent")
	}
}

// ---------------------------------------------------------------------------
// Header / status bar / loading configuration
// ---------------------------------------------------------------------------

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func TestHeaderTitleAndRecordCountConfigurable(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.Title = "my-translator"
	}), 80, 24)
	header := m.renderHeader()
	if !strings.Contains(header, "my-translator") {
		t.Errorf("custom title missing from header: %q", header)
	}
	if strings.Contains(header, "trans-tui") {
		t.Errorf("default title still rendered: %q", header)
	}
	if !strings.Contains(header, "Records: 0") {
		t.Errorf("record count missing from header: %q", header)
	}

	noCount := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.ShowRecordCount = false
	}), 80, 24)
	header = noCount.renderHeader()
	if strings.Contains(header, "Records:") {
		t.Errorf("record count still rendered when disabled: %q", header)
	}
	if !strings.Contains(header, "trans-tui") {
		t.Errorf("title should remain without record count: %q", header)
	}
}

func TestStatusBarFlagsConfigurable(t *testing.T) {
	all := newAppearanceModel(t, DefaultTheme(), 80, 24)
	bar := all.renderStatusBar()
	for _, want := range []string{"Records:", "Scroll:", ": input", ": quit"} {
		if !strings.Contains(bar, want) {
			t.Errorf("default status bar missing %q: %q", want, bar)
		}
	}

	cases := []struct {
		name   string
		mutate func(*config.AppearanceConfig)
		lack   []string
	}{
		{"no record count", func(a *config.AppearanceConfig) { a.StatusBar.ShowRecordCount = false }, []string{"Records:"}},
		{"no scroll position", func(a *config.AppearanceConfig) { a.StatusBar.ShowScrollPosition = false }, []string{"Scroll:"}},
		{"no input hint", func(a *config.AppearanceConfig) { a.StatusBar.ShowInputHint = false }, []string{": input"}},
		{"no quit hint", func(a *config.AppearanceConfig) { a.StatusBar.ShowQuitHint = false }, []string{": quit"}},
		{"nothing enabled", func(a *config.AppearanceConfig) {
			a.StatusBar.ShowRecordCount = false
			a.StatusBar.ShowScrollPosition = false
			a.StatusBar.ShowInputHint = false
			a.StatusBar.ShowQuitHint = false
		}, []string{"Records:", "Scroll:", ": input", ": quit"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newAppearanceModel(t, themeWith(tc.mutate), 80, 24)
			bar := m.renderStatusBar()
			for _, lack := range tc.lack {
				if strings.Contains(bar, lack) {
					t.Errorf("status bar still contains %q: %q", lack, bar)
				}
			}
		})
	}
}

func TestStatusBarCancelHintConfigurable(t *testing.T) {
	withHint := newAppearanceModel(t, DefaultTheme(), 80, 24)
	next, _ := withHint.enterInputMode()
	withHint = next
	if bar := withHint.renderStatusBar(); !strings.Contains(bar, "esc: cancel") {
		t.Errorf("input-mode status bar missing cancel hint: %q", bar)
	}

	without := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.StatusBar.ShowCancelHint = false
	}), 80, 24)
	next, _ = without.enterInputMode()
	without = next
	if bar := without.renderStatusBar(); strings.Contains(bar, ": cancel") {
		t.Errorf("cancel hint still rendered when disabled: %q", bar)
	}
}

func TestStatusBarHintsUseConfiguredFirstKeys(t *testing.T) {
	m := newAppearanceModel(t, DefaultTheme(), 80, 24)
	m.keyMap = NewKeyMapFromBindings(config.KeyBindingsConfig{
		Quit:        []string{"Q"},
		ManualInput: []string{"m"},
	})
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "Q: quit") {
		t.Errorf("status bar should show the configured quit key: %q", bar)
	}
	if !strings.Contains(bar, "m: input") {
		t.Errorf("status bar should show the configured input key: %q", bar)
	}
}

func TestLoadingTextConfigurable(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Loading.Text = "Please wait…"
	}), 80, 24)
	m.Loading = true
	m = m.recalcViewportHeight()

	view := m.renderView()
	if !strings.Contains(view, "Please wait…") {
		t.Errorf("custom loading text missing: %q", view)
	}
	if strings.Contains(view, "Translating...") {
		t.Errorf("default loading text still rendered: %q", view)
	}
	assertRenderHeight(t, m, "custom loading text")
}

func TestCustomAppearanceColorsReachRender(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.Foreground = "33"
		a.StatusBar.Foreground = "44"
	}), 80, 24)

	view := m.renderView()
	if !strings.Contains(view, "38;5;33") {
		t.Error("configured header foreground not present in render")
	}
	if !strings.Contains(view, "38;5;44") {
		t.Error("configured status bar foreground not present in render")
	}
}

func TestInputPromptConfigurable(t *testing.T) {
	ta := newInputTextArea(themeWith(func(a *config.AppearanceConfig) {
		a.Input.PromptForeground = "196"
		a.Input.PromptBold = false
	}))
	_ = ta.Focus()
	view := ta.View()

	if !strings.Contains(view, "38;5;196") {
		t.Errorf("configured prompt foreground not rendered: %q", view)
	}
	// A bold render would extend the sequence ("38;5;196;1m" or ";1m"); a
	// non-bold prompt resets right after the color ("38;5;196m").
	if strings.Contains(view, "38;5;196;1m") || strings.Contains(view, "[1m") {
		t.Errorf("prompt should not be bold when prompt_bold = false: %q", view)
	}
}

// TestThemesArePerModelNotGlobal pins that building a model with one theme
// never changes another model's rendering: the theme is stored per model and
// the default theme stays the default.
func TestThemesArePerModelNotGlobal(t *testing.T) {
	custom := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.Header.Title = "other-app"
		a.Header.Foreground = "33"
	}), 80, 24)
	plain := newAppearanceModel(t, DefaultTheme(), 80, 24)

	if strings.Contains(custom.renderHeader(), "trans-tui") {
		t.Error("custom model lost its configured title")
	}
	plainHeader := plain.renderHeader()
	if !strings.Contains(plainHeader, "trans-tui") {
		t.Errorf("default model lost the default title: %q", plainHeader)
	}
	if strings.Contains(plainHeader, "38;5;33") {
		t.Error("custom header color leaked into the default model")
	}
	// Re-rendering the custom model after the default one must not drift.
	if again := custom.renderHeader(); !strings.Contains(again, "other-app") {
		t.Errorf("custom model changed after rendering the default model: %q", again)
	}
}
