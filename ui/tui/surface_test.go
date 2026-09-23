package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
)

// ---------------------------------------------------------------------------
// Cell-level view parsing
// ---------------------------------------------------------------------------

// viewGrid is a column-indexed cell view of a rendered view string. Parsing
// mirrors what a terminal does with the bytes it receives: each grapheme
// becomes a cell, and a wide grapheme covers its full width — both of its
// columns resolve to the same cell, exactly like the terminal paints them
// from one wide glyph. String-level checks (strings.Contains on escape
// sequences) cannot see reset trailing cells, JoinVertical padding or whole
// blank regions, so every coverage assertion below goes through this grid.
type viewGrid struct {
	rows [][]uv.Cell
}

// parseView decomposes view into cells the same way Bubble Tea's own renderer
// does: ultraviolet StyledString drawn into a screen buffer with the grapheme
// width method. Unwritten cells stay visible as empty cells (background nil),
// which is exactly what the transparent-mode assertions need.
func parseView(view string) viewGrid {
	width, height := lipgloss.Width(view), lipgloss.Height(view)
	if width <= 0 || height <= 0 {
		return viewGrid{}
	}
	scr := uv.NewScreenBuffer(width, height)
	scr.Method = ansi.GraphemeWidth
	uv.NewStyledString(view).Draw(scr, scr.Bounds())

	g := viewGrid{rows: make([][]uv.Cell, height)}
	for y := 0; y < height; y++ {
		row := g.rows[y]
		for x := 0; x < width; {
			c := scr.CellAt(x, y)
			if c == nil {
				c = &uv.EmptyCell
			}
			switch {
			case c.Width > 1:
				// A wide grapheme covers its full width from one cell, so
				// both columns resolve to the same (explicit) cell.
				for range c.Width {
					row = append(row, *c)
				}
				x += c.Width
			case c.Width == 0:
				// Continuation column of a wide glyph: already covered.
				x++
			default:
				row = append(row, *c)
				x++
			}
		}
		g.rows[y] = row
	}
	return g
}

// at returns the cell covering terminal column x of row y.
func (g viewGrid) at(x, y int) (uv.Cell, bool) {
	if y < 0 || y >= len(g.rows) || x < 0 || x >= len(g.rows[y]) {
		return uv.Cell{}, false
	}
	return g.rows[y][x], true
}

// sameColor compares two colors by their rendered RGBA values so a palette
// index survives the SGR round-trip regardless of its concrete Go type.
func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// requireOpaqueSurface asserts the full W × H contract cell by cell: exactly
// height rows, every row exactly width columns wide, and every single cell
// carrying an explicit background. It returns the grid for sampling.
func requireOpaqueSurface(t *testing.T, view string, width, height int) viewGrid {
	t.Helper()
	if got := lipgloss.Height(view); got != height {
		t.Fatalf("surface height = %d, want terminalHeight %d", got, height)
	}
	g := parseView(view)
	if len(g.rows) != height {
		t.Fatalf("surface rows = %d, want %d", len(g.rows), height)
	}
	for y, row := range g.rows {
		if len(row) != width {
			t.Fatalf("surface row %d width = %d, want terminalWidth %d", y, len(row), width)
		}
		for x, c := range row {
			if c.Style.Bg == nil {
				t.Fatalf("surface cell (%d,%d) has no explicit background (content %q)", x, y, c.Content)
			}
		}
	}
	return g
}

// requireBg asserts the background color of one sampled cell.
func requireBg(t *testing.T, g viewGrid, x, y int, want color.Color, what string) {
	t.Helper()
	c, ok := g.at(x, y)
	if !ok {
		t.Fatalf("%s: cell (%d,%d) outside the grid", what, x, y)
	}
	if c.Style.Bg == nil {
		t.Fatalf("%s: cell (%d,%d) has no background (content %q)", what, x, y, c.Content)
	}
	if !sameColor(c.Style.Bg, want) {
		t.Errorf("%s: cell (%d,%d) background = %v, want %v", what, x, y, c.Style.Bg, want)
	}
}

// requireNoBg asserts a cell has no background at all (transparent mode).
func requireNoBg(t *testing.T, g viewGrid, x, y int, what string) {
	t.Helper()
	c, ok := g.at(x, y)
	if !ok {
		t.Fatalf("%s: cell (%d,%d) outside the grid", what, x, y)
	}
	if c.Style.Bg != nil {
		t.Errorf("%s: cell (%d,%d) background = %v, want none", what, x, y, c.Style.Bg)
	}
}

// requireLayoutInvariants checks that painting did not disturb the raw
// layout: renderView still fills exactly the terminal height, and the
// painted surface matches the terminal size.
func requireLayoutInvariants(t *testing.T, m Model, context string) {
	t.Helper()
	assertRenderHeight(t, m, context)
	requireOpaqueSurface(t, m.surfaceView(), m.terminalWidth, m.terminalHeight)
}

// ---------------------------------------------------------------------------
// Theme surface fields
// ---------------------------------------------------------------------------

func TestThemeResolvesSurfaceBackground(t *testing.T) {
	th := NewTheme(config.DefaultAppearance())
	if th.SurfaceTransparent {
		t.Error("default theme is surface-transparent, want opaque")
	}
	if !sameColor(th.SurfaceBackground, lipgloss.Color("235")) {
		t.Errorf("default surface background = %v, want palette 235", th.SurfaceBackground)
	}

	transparent := NewTheme(config.AppearanceConfig{
		TransparentBackground: true,
		Background:            "235",
	})
	if !transparent.SurfaceTransparent {
		t.Error("transparent_background = true did not set SurfaceTransparent")
	}

	unset := NewTheme(func() config.AppearanceConfig {
		a := config.DefaultAppearance()
		a.Background = ""
		return a
	}())
	if unset.SurfaceBackground != nil {
		t.Errorf("empty background = %v, want nil (no surface fill)", unset.SurfaceBackground)
	}

	invalid := NewTheme(func() config.AppearanceConfig {
		a := config.DefaultAppearance()
		a.Background = "not-a-color"
		return a
	}())
	if invalid.SurfaceBackground != nil {
		t.Errorf("unparsable background = %v, want nil (lipgloss NoColor semantics)", invalid.SurfaceBackground)
	}

	hex := NewTheme(func() config.AppearanceConfig {
		a := config.DefaultAppearance()
		a.Background = "#1e1e1e"
		return a
	}())
	if !sameColor(hex.SurfaceBackground, color.RGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff}) {
		t.Errorf("hex background = %v, want #1e1e1e", hex.SurfaceBackground)
	}
}

// ---------------------------------------------------------------------------
// Surface coverage across states
// ---------------------------------------------------------------------------

// withEmojiRecord adds a record full of wide graphemes (emoji, ZWJ sequences)
// so the wide-cell path of paintSurface is exercised end to end.
func withEmojiRecord(m Model) Model {
	m.Records = append(m.Records, core.TranslationRecord{
		SourceLang: "en", TargetLang: "zh",
		Source:      "🌍 world 🎉 party " + strings.Repeat("emoji 😀 ", 12),
		Translation: "😀 世界 🎉 " + strings.Repeat("表情包 ", 12),
	})
	return m.refreshHistory(false)
}

// withSelection records a mouse selection over the first record's source line
// and re-renders the viewport with the highlight, as handleMouse does.
func withSelection(m Model) Model {
	m.Records = append(m.Records, core.TranslationRecord{
		SourceLang: "en", TargetLang: "ja", Source: "Hello World", Translation: "こんにちは世界",
	})
	m = m.refreshHistory(false)
	m.sel = selection{
		start: SelectionPoint{VisualRow: 1, CellCol: 0},
		end:   SelectionPoint{VisualRow: 1, CellCol: 5},
	}
	m.viewport.SetContent(m.renderRecordsWithHighlight())
	return m
}

// TestOpaqueSurfaceCoversEveryState is the core coverage regression: with
// transparent_background = false and background = "235" the final view must
// have an explicit background on every one of the W × H cells — idle, empty
// history, loading, error, input mode, records, long/CJK/emoji content and
// selection alike. The raw view keeps its height invariant (paintSurface is
// checked not to re-flow or grow the layout).
func TestOpaqueSurfaceCoversEveryState(t *testing.T) {
	states := []struct {
		name  string
		apply func(t *testing.T, m Model) Model
	}{
		{"idle empty history", func(t *testing.T, m Model) Model { return m }},
		{"loading", func(t *testing.T, m Model) Model {
			m.Loading = true
			return m.recalcViewportHeight()
		}},
		{"error", func(t *testing.T, m Model) Model {
			m.Error = "boom"
			return m.recalcViewportHeight()
		}},
		{"error and loading", func(t *testing.T, m Model) Model {
			m.Error = "boom"
			m.Loading = true
			return m.recalcViewportHeight()
		}},
		{"input", func(t *testing.T, m Model) Model {
			next, _ := m.enterInputMode()
			return next
		}},
		{"input with error", func(t *testing.T, m Model) Model {
			m.Error = "provider exploded"
			m = m.recalcViewportHeight()
			next, _ := m.enterInputMode()
			return next
		}},
		{"multiple records", func(t *testing.T, m Model) Model { return withLayoutRecords(m) }},
		{"long records", func(t *testing.T, m Model) Model {
			m.Records = append(m.Records, core.TranslationRecord{
				SourceLang:  "en",
				TargetLang:  "en",
				Source:      strings.Repeat("wrap-me-", 60),
				Translation: strings.Repeat("long-token-", 60),
			})
			return m.refreshHistory(false)
		}},
		{"CJK", func(t *testing.T, m Model) Model {
			m.Records = append(m.Records, core.TranslationRecord{
				SourceLang:  "zh",
				TargetLang:  "en",
				Source:      strings.Repeat("这是一段很长的中文文本用于测试换行", 8),
				Translation: strings.Repeat("a fairly long english translation ", 8),
			})
			return m.refreshHistory(false)
		}},
		{"emoji", func(t *testing.T, m Model) Model { return withEmojiRecord(m) }},
		{"selection", func(t *testing.T, m Model) Model { return withSelection(m) }},
		{"records loading and input", func(t *testing.T, m Model) Model {
			m = withLayoutRecords(m)
			m.Loading = true
			m = m.recalcViewportHeight()
			next, _ := m.enterInputMode()
			return next
		}},
	}
	sizes := []struct {
		name          string
		width, height int
	}{
		{"standard", 80, 24},
		{"small", 40, 12},
		{"large", 200, 50},
	}

	for _, sz := range sizes {
		for _, st := range states {
			t.Run(sz.name+"/"+st.name, func(t *testing.T) {
				m := newAppearanceModel(t, DefaultTheme(), sz.width, sz.height)
				m = st.apply(t, m)
				requireLayoutInvariants(t, m, sz.name+"/"+st.name)
			})
		}
	}
}

// TestOpaqueSurfaceCoversSectionToggleMatrix runs the same cell-level
// coverage over every section-toggle combination, each state and a resize:
// the surface must be complete regardless of which sections render.
func TestOpaqueSurfaceCoversSectionToggleMatrix(t *testing.T) {
	themes := []struct {
		name   string
		mutate func(*config.AppearanceConfig)
	}{
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
		{"short", 24, 10},
		{"tall", 100, 40},
	}
	states := []struct {
		name  string
		apply func(m Model) Model
	}{
		{"idle", func(m Model) Model { return m }},
		{"loading", func(m Model) Model {
			m.Loading = true
			return m.recalcViewportHeight()
		}},
		{"error", func(m Model) Model {
			m.Error = "boom"
			return m.recalcViewportHeight()
		}},
		{"input", func(m Model) Model {
			next, _ := m.enterInputMode()
			return next
		}},
	}
	resizes := []tea.WindowSizeMsg{
		{Width: 60, Height: 18},
		{Width: 120, Height: 30},
	}

	for _, th := range themes {
		for _, sz := range sizes {
			for _, st := range states {
				t.Run(th.name+"/"+sz.name+"/"+st.name, func(t *testing.T) {
					m := newAppearanceModel(t, themeWith(th.mutate), sz.width, sz.height)
					m = withLayoutRecords(m)
					m = st.apply(m)
					requireLayoutInvariants(t, m, "initial")

					for _, size := range resizes {
						r, _ := m.Update(size)
						m = r.(Model)
						requireLayoutInvariants(t, m, "resize")
					}
				})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Component priority sampling
// ---------------------------------------------------------------------------

// TestOpaqueSurfaceColorPriority pins the background priority chain on real
// cells: surface (235) under everything no component paints, component
// backgrounds (status bar 236, record card 17) kept where components do
// paint, selection (240) winning over both.
func TestOpaqueSurfaceColorPriority(t *testing.T) {
	theme := themeWith(func(a *config.AppearanceConfig) {
		a.Background = "235"
		a.StatusBar.Background = "236"
		a.Record.Background = "17"
	})
	m := newAppearanceModel(t, theme, 80, 24)
	m.Records = append(m.Records, core.TranslationRecord{
		SourceLang: "en", TargetLang: "ja", Source: "Hello World", Translation: "こんにちは世界",
	})
	m = m.refreshHistory(false)
	m.sel = selection{
		start: SelectionPoint{VisualRow: 1, CellCol: 0},
		end:   SelectionPoint{VisualRow: 1, CellCol: 5},
	}
	m.viewport.SetContent(m.renderRecordsWithHighlight())

	requireLayoutInvariants(t, m, "priority sampling")
	g := parseView(m.surfaceView())

	surface := lipgloss.Color("235")
	compStatus := lipgloss.Color("236")
	compCard := lipgloss.Color("17")
	selection := lipgloss.Color("240")

	// Row layout at 80x24 idle: y=0 header, y=1 card top border,
	// y=2 source line, y=23 status bar.
	requireBg(t, g, 0, 0, surface, "header padding (surface-only)")
	requireBg(t, g, 79, 0, surface, "header row end (surface-only)")

	// Status bar: explicit component background inside its content, surface
	// color on the trailing cells past the text.
	requireBg(t, g, 2, 23, compStatus, "status bar text (component override)")
	requireBg(t, g, 79, 23, surface, "status bar row end (surface-only)")

	// Selection wins over everything on the source line: "Hello" is the
	// selected span, cells x=2..6.
	for x := 2; x <= 6; x++ {
		requireBg(t, g, x, 2, selection, "selection highlight")
	}
	// Record card cells that carry the component background explicitly in
	// the raw render (its padding) keep it; at least the left padding cell
	// proves the component color survives the surface pass.
	cardCells := 0
	for x := 0; x < 80; x++ {
		c, _ := g.at(x, 2)
		if c.Style.Bg != nil && sameColor(c.Style.Bg, compCard) {
			cardCells++
		}
	}
	if cardCells == 0 {
		t.Error("record card component background (17) disappeared from the painted view")
	}
	// fg-only content inside the card — the card border and the " World"
	// run — has no component background in the raw render, so the surface
	// color fills it: that is the bug this whole mechanism fixes.
	requireBg(t, g, 0, 2, surface, "record card border (fg-only, surface under it)")
	requireBg(t, g, 79, 2, surface, "record card right border (fg-only, surface under it)")
	requireBg(t, g, 8, 2, surface, "record card text without its own background")
}

// ---------------------------------------------------------------------------
// Row-end regression
// ---------------------------------------------------------------------------

// TestRowEndBackgroundRegression proves that the cells a terminal would
// otherwise leave at default background — header/status/loading row ends, the
// viewport's right side and the area below viewport content — all carry the
// surface background in the final (painted) view.
func TestRowEndBackgroundRegression(t *testing.T) {
	surface := lipgloss.Color("235")

	t.Run("header row end", func(t *testing.T) {
		m := newAppearanceModel(t, DefaultTheme(), 80, 24)
		g := parseView(m.surfaceView())
		requireBg(t, g, 79, 0, surface, "header row end")
		requireBg(t, g, 0, 0, surface, "header row start")
	})

	t.Run("status bar row end", func(t *testing.T) {
		m := newAppearanceModel(t, DefaultTheme(), 80, 24)
		g := parseView(m.surfaceView())
		// The status text ends long before column 79; the trailing cells
		// must still be painted.
		requireBg(t, g, 79, 23, surface, "status bar row end")
		requireBg(t, g, 78, 23, surface, "status bar row end-1")
	})

	t.Run("loading row end", func(t *testing.T) {
		m := newAppearanceModel(t, DefaultTheme(), 80, 24)
		m.Loading = true
		m = m.recalcViewportHeight()
		g := parseView(m.surfaceView())
		if h := lipgloss.Height(m.surfaceView()); h != 24 {
			t.Fatalf("height with loading = %d, want 24", h)
		}
		requireBg(t, g, 0, 23, surface, "loading text cell (fg-only component)")
		requireBg(t, g, 79, 23, surface, "loading row end")
	})

	t.Run("viewport right side", func(t *testing.T) {
		m := newAppearanceModel(t, DefaultTheme(), 80, 24)
		m = withLayoutRecords(m)
		g := parseView(m.surfaceView())
		for y := 1; y <= 22; y++ {
			requireBg(t, g, 79, y, surface, "viewport right edge")
		}
	})

	t.Run("viewport below content", func(t *testing.T) {
		// Empty history: the hint occupies the viewport's first row; every
		// row below it inside the viewport is an unwritten cell.
		m := newAppearanceModel(t, DefaultTheme(), 80, 24)
		g := parseView(m.surfaceView())
		for y := 5; y <= 21; y++ {
			requireBg(t, g, 0, y, surface, "viewport area below content")
			requireBg(t, g, 79, y, surface, "viewport area below content, right edge")
		}
	})

	t.Run("gap between sections", func(t *testing.T) {
		// Header disabled + loading: row 0 (top of viewport) and the rows
		// between viewport content and the status bar are section gaps.
		m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
			a.Header.Enabled = false
		}), 80, 24)
		m = withLayoutRecords(m)
		m.Loading = true
		m = m.recalcViewportHeight()
		g := parseView(m.surfaceView())
		requireBg(t, g, 79, 0, surface, "first row without header")
		requireBg(t, g, 79, 22, surface, "status bar row end without header")
		requireBg(t, g, 79, 23, surface, "loading row end without header")
	})
}

// ---------------------------------------------------------------------------
// Transparent mode
// ---------------------------------------------------------------------------

// TestTransparentModeSkipsPaintSurface pins that transparent_background = true
// skips paintSurface completely: the final view is byte-for-byte the raw
// render, no cell receives the surface background (235 is nowhere in the
// grid), component backgrounds stay dropped per the existing contract, and
// the selection highlight keeps its color.
func TestTransparentModeSkipsPaintSurface(t *testing.T) {
	theme := themeWith(func(a *config.AppearanceConfig) {
		a.TransparentBackground = true
		a.Background = "235"
		a.Record.Background = "17"
		a.StatusBar.Background = "235"
	})
	m := newAppearanceModel(t, theme, 80, 24)
	m.Records = append(m.Records, core.TranslationRecord{
		SourceLang: "en", TargetLang: "ja", Source: "Hello World", Translation: "こんにちは世界",
	})
	m = m.refreshHistory(false)
	m.sel = selection{
		start: SelectionPoint{VisualRow: 1, CellCol: 0},
		end:   SelectionPoint{VisualRow: 1, CellCol: 5},
	}
	m.viewport.SetContent(m.renderRecordsWithHighlight())

	raw := m.renderView()
	final := m.surfaceView()
	if final != raw {
		t.Error("transparent surface modified the raw view; paintSurface must be skipped")
	}
	if v := m.View(); v.Content != raw {
		t.Error("View() content differs from the raw render while transparent")
	}

	// Height stays correct; widths are the raw layout's own (no fill).
	if got := lipgloss.Height(final); got != 24 {
		t.Errorf("transparent view height = %d, want 24", got)
	}

	g := parseView(final)
	if len(g.rows) != 24 {
		t.Fatalf("transparent rows = %d, want 24", len(g.rows))
	}
	surface := lipgloss.Color("235")
	selColor := lipgloss.Color("240")
	selCount := 0
	for y, row := range g.rows {
		for x, c := range row {
			if c.Style.Bg == nil {
				continue
			}
			if sameColor(c.Style.Bg, surface) {
				t.Fatalf("transparent view cell (%d,%d) carries the surface background 235", x, y)
			}
			if !sameColor(c.Style.Bg, selColor) {
				t.Errorf("transparent view cell (%d,%d) background = %v, want none or the selection", x, y, c.Style.Bg)
			}
			selCount++
		}
	}
	if selCount == 0 {
		t.Error("selection highlight disappeared from the transparent view")
	}

	// The selection itself is intact cell-wise.
	requireBg(t, g, 2, 2, selColor, "selection highlight while transparent")
	// Surface-only cells were never painted.
	requireNoBg(t, g, 79, 0, "header row end while transparent")
	requireNoBg(t, g, 79, 23, "status bar row end while transparent")
	if strings.Contains(final, "48;5;235") {
		t.Error("transparent view contains the global 235 background sequence")
	}
}

// TestTransparentModeWithoutSelectionHasNoBackgrounds is the stronger
// negative: in transparent mode no cell may carry the *surface* background
// (the global 235 does not exist), and the cells no component painted —
// header/status row ends, for example — stay at default background.
func TestTransparentModeWithoutSelectionHasNoBackgrounds(t *testing.T) {
	m := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.TransparentBackground = true
	}), 80, 24)
	m = withLayoutRecords(m)
	m.Error = "boom"
	m = m.recalcViewportHeight()
	next, _ := m.enterInputMode()
	m = next

	surface := lipgloss.Color("235")
	g := parseView(m.surfaceView())
	for y, row := range g.rows {
		for x, c := range row {
			if c.Style.Bg != nil && sameColor(c.Style.Bg, surface) {
				t.Fatalf("transparent cell (%d,%d) carries the surface background 235", x, y)
			}
		}
	}
	if len(g.rows) != m.terminalHeight {
		t.Errorf("rows = %d, want %d", len(g.rows), m.terminalHeight)
	}
	// Unpainted cells keep the terminal default (nil background).
	requireNoBg(t, g, 79, 0, "header row end while transparent")
	requireNoBg(t, g, 79, m.terminalHeight-1, "status/loading row end while transparent")
}

// ---------------------------------------------------------------------------
// View entry point and guards
// ---------------------------------------------------------------------------

// TestViewUsesSurfaceBackground pins that Model.View — the actual Bubble Tea
// entry point — hands the painted string to the program in opaque mode and
// the raw string in transparent mode.
func TestViewUsesSurfaceBackground(t *testing.T) {
	opaque := newAppearanceModel(t, DefaultTheme(), 80, 24)
	content := opaque.View().Content
	if content == opaque.renderView() {
		t.Fatal("opaque View() content is unpainted")
	}
	requireOpaqueSurface(t, content, 80, 24)

	transparent := newAppearanceModel(t, themeWith(func(a *config.AppearanceConfig) {
		a.TransparentBackground = true
	}), 80, 24)
	if got := transparent.View().Content; got != transparent.renderView() {
		t.Error("transparent View() painted a surface background")
	}
}

// TestSurfaceViewBeforeLayout guards the pre-Windowsize state: without a
// terminal size there is nothing to fill, and the raw view is returned
// untouched.
func TestSurfaceViewBeforeLayout(t *testing.T) {
	var m Model
	if got := m.surfaceView(); got != "Initializing..." {
		t.Errorf("surfaceView before layout = %q, want the raw initializing view", got)
	}
}

// ---------------------------------------------------------------------------
// paintSurface unit tests
// ---------------------------------------------------------------------------

func TestPaintSurfaceGuards(t *testing.T) {
	bg := lipgloss.Color("235")
	if got := paintSurface("hello", 0, 10, bg); got != "hello" {
		t.Errorf("zero width painted %q, want the raw view", got)
	}
	if got := paintSurface("hello", 10, 0, bg); got != "hello" {
		t.Errorf("zero height painted %q, want the raw view", got)
	}
	if got := paintSurface("hello", 10, 2, nil); got != "hello" {
		t.Errorf("nil background painted %q, want the raw view", got)
	}
}

// TestPaintSurfaceFillsUnwrittenCells: content shorter than the buffer leaves
// whole rows/columns unwritten — they must still come out with the surface
// background and without losing the trailing spaces (no TrimSpace-style
// render path).
func TestPaintSurfaceFillsUnwrittenCells(t *testing.T) {
	bg := lipgloss.Color("235")
	painted := paintSurface("ab\ncd", 5, 3, bg)
	g := parseView(painted)

	if len(g.rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(g.rows))
	}
	for y, row := range g.rows {
		if len(row) != 5 {
			t.Fatalf("row %d width = %d, want 5 (trailing background must survive)", y, len(row))
		}
		for x, c := range row {
			if !sameColor(c.Style.Bg, bg) {
				t.Errorf("cell (%d,%d) bg = %v, want %v", x, y, c.Style.Bg, bg)
			}
		}
	}
	// The written content itself is still there.
	if g.rows[0][0].Content != "a" || g.rows[1][1].Content != "d" {
		t.Errorf("content lost: %q / %q", g.rows[0][0].Content, g.rows[1][1].Content)
	}
	// Nothing but spaces outside the content.
	if g.rows[2][4].Content != " " {
		t.Errorf("bottom-right cell content = %q, want a space", g.rows[2][4].Content)
	}
}

// TestPaintSurfaceKeepsComponentBackgrounds: cells that already have a
// background (component or selection) keep it; cells without one get the
// surface color — even in the middle of a row, after an SGR reset.
func TestPaintSurfaceKeepsComponentBackgrounds(t *testing.T) {
	bg := lipgloss.Color("235")
	comp := lipgloss.Color("17")
	raw := "\x1b[48;5;17mAB\x1b[0mCD\x1b[48;5;240mEF\x1b[0m"

	g := parseView(paintSurface(raw, 8, 1, bg))
	if len(g.rows) != 1 || len(g.rows[0]) != 8 {
		t.Fatalf("grid = %d rows x ?, want 1x8", len(g.rows))
	}
	wants := []struct {
		x    int
		want color.Color
	}{{0, comp}, {1, comp}, {2, bg}, {3, bg}, {4, lipgloss.Color("240")}, {5, lipgloss.Color("240")}, {6, bg}, {7, bg}}
	for _, w := range wants {
		c := g.rows[0][w.x]
		if !sameColor(c.Style.Bg, w.want) {
			t.Errorf("cell %d bg = %v, want %v", w.x, c.Style.Bg, w.want)
		}
	}
}

// TestPaintSurfaceWideCells: wide graphemes occupy two terminal columns from
// one cell; both columns resolve to the explicitly painted background and
// the row keeps its exact width.
func TestPaintSurfaceWideCells(t *testing.T) {
	bg := lipgloss.Color("235")
	g := parseView(paintSurface("你好😀ok", 6, 1, bg))

	if len(g.rows) != 1 || len(g.rows[0]) != 6 {
		t.Fatalf("rows = %d, first row width = %d, want 1x6", len(g.rows), len(g.rows[0]))
	}
	for x, c := range g.rows[0] {
		if c.Style.Bg == nil {
			t.Errorf("wide-cell row column %d has no background", x)
		}
	}
	// 你 = columns 0-1, 好 = 2-3, 😀 = 4-5.
	if g.rows[0][0].Content != "你" || g.rows[0][2].Content != "好" || g.rows[0][4].Content != "😀" {
		t.Errorf("wide cells misplaced: %q %q %q",
			g.rows[0][0].Content, g.rows[0][2].Content, g.rows[0][4].Content)
	}
}

// TestPaintSurfaceClipsWideCellAtRightEdge: a wide glyph starting in the
// last column is clipped to a styled blank instead of overflowing the row.
func TestPaintSurfaceClipsWideCellAtRightEdge(t *testing.T) {
	bg := lipgloss.Color("235")
	g := parseView(paintSurface("abcd你", 5, 1, bg))

	if len(g.rows) != 1 || len(g.rows[0]) != 5 {
		t.Fatalf("rows = %d, first row width = %d, want 1x5", len(g.rows), len(g.rows[0]))
	}
	for x, c := range g.rows[0] {
		if c.Style.Bg == nil {
			t.Errorf("clipped column %d has no background", x)
		}
	}
}

// TestPaintSurfaceHexBackground pins hex parsing end to end.
func TestPaintSurfaceHexBackground(t *testing.T) {
	bg := surfaceColor("#1e1e1e")
	g := parseView(paintSurface("", 3, 2, bg))
	want := color.RGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff}
	for y, row := range g.rows {
		for x, c := range row {
			if !sameColor(c.Style.Bg, want) {
				t.Errorf("cell (%d,%d) bg = %v, want #1e1e1e", x, y, c.Style.Bg)
			}
		}
	}
}
