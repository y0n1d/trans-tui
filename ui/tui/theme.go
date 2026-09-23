package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/y0n1d/trans-tui/internal/config"
)

// Theme holds every style and display flag the TUI renders with. It is built
// once from config.AppearanceConfig (NewTheme) and stored on the Model; the
// package has no mutable global theme. A zero Theme means "not configured":
// Model.themeOr falls back to the default theme, so a bare Model{} keeps the
// original look.
type Theme struct {
	// Display flags and text resolved at construction time.
	HeaderEnabled         bool
	HeaderTitle           string
	HeaderShowRecordCount bool
	StatusBarEnabled      bool
	ShowRecordCount       bool
	ShowScrollPosition    bool
	ShowInputHint         bool
	ShowQuitHint          bool
	ShowCancelHint        bool
	LoadingText           string

	// Surface-level background (appearance.transparent_background and
	// appearance.background), resolved once at construction. With
	// SurfaceTransparent off, View fills every terminal cell that no
	// component painted itself with SurfaceBackground, giving a true
	// W × H opaque surface. SurfaceBackground is nil when background is
	// empty or unparsable (lipgloss's "unset" colors), in which case no
	// surface fill happens. Priority stays: surface < component <
	// selection — components keep their own backgrounds, and paintSurface
	// only fills cells without one.
	SurfaceTransparent bool
	SurfaceBackground  color.Color

	// Styles.
	Header            lipgloss.Style
	HeaderRecordCount lipgloss.Style
	StatusBar         lipgloss.Style
	Record            lipgloss.Style
	Source            lipgloss.Style
	Translation       lipgloss.Style
	Error             lipgloss.Style
	ErrorPanel        lipgloss.Style
	Loading           lipgloss.Style
	InputPanel        lipgloss.Style
	Selection         lipgloss.Style
	InputPrompt       lipgloss.Style

	// set marks a Theme as built by NewTheme; the zero value is unset.
	set bool
}

// NewTheme builds a Theme from the [appearance] config section.
// Background colors are dropped when transparent_background is true, so the
// terminal background shows through; everything else is kept. The
// surface-level background (appearance.background) is resolved into
// SurfaceBackground here but only painted when transparent_background is
// false (see Model.surfaceView).
func NewTheme(a config.AppearanceConfig) Theme {
	t := Theme{
		HeaderEnabled:         a.Header.Enabled,
		HeaderTitle:           a.Header.Title,
		HeaderShowRecordCount: a.Header.ShowRecordCount,
		StatusBarEnabled:      a.StatusBar.Enabled,
		ShowRecordCount:       a.StatusBar.ShowRecordCount,
		ShowScrollPosition:    a.StatusBar.ShowScrollPosition,
		ShowInputHint:         a.StatusBar.ShowInputHint,
		ShowQuitHint:          a.StatusBar.ShowQuitHint,
		ShowCancelHint:        a.StatusBar.ShowCancelHint,
		LoadingText:           a.Loading.Text,
		SurfaceTransparent:    a.TransparentBackground,
		SurfaceBackground:     surfaceColor(a.Background),
		set:                   true,
	}

	tr := a.TransparentBackground

	t.Header = horizontalPadding(
		withBold(withForeground(lipgloss.NewStyle(), a.Header.Foreground), a.Header.Bold),
		a.Header.PaddingLeft, a.Header.PaddingRight,
	)
	t.Header = withBackground(t.Header, a.Header.Background, tr)

	// The header's record count and the empty-history hint reuse the status
	// bar's muted colors, matching the pre-config coupling.
	t.HeaderRecordCount = horizontalPadding(
		withForeground(lipgloss.NewStyle(), a.StatusBar.Foreground),
		a.StatusBar.PaddingLeft, a.StatusBar.PaddingRight,
	)
	t.HeaderRecordCount = withBackground(t.HeaderRecordCount, a.StatusBar.Background, tr)

	t.StatusBar = horizontalPadding(
		withForeground(lipgloss.NewStyle(), a.StatusBar.Foreground),
		a.StatusBar.PaddingLeft, a.StatusBar.PaddingRight,
	)
	t.StatusBar = withBackground(t.StatusBar, a.StatusBar.Background, tr)

	// Cards and the error panel keep their fixed 1-cell horizontal padding:
	// the semantic map's row offsets are derived from that frame
	// (recordBorderPadding, leftOffset), so it must not become configurable.
	t.Record = withBackground(
		horizontalPadding(borderStyle(lipgloss.NewStyle(), a.Record.BorderForeground), 1, 1),
		a.Record.Background, tr,
	)

	t.Source = withBold(withForeground(lipgloss.NewStyle(), a.Source.Foreground), a.Source.Bold)
	t.Translation = withForeground(lipgloss.NewStyle(), a.Translation.Foreground)
	t.Error = withBold(withForeground(lipgloss.NewStyle(), a.Error.Foreground), a.Error.Bold)

	t.ErrorPanel = withBackground(
		withForeground(
			horizontalPadding(borderStyle(lipgloss.NewStyle(), a.ErrorPanel.BorderForeground), 1, 1),
			a.ErrorPanel.Foreground,
		),
		a.ErrorPanel.Background, tr,
	)

	t.Loading = withBold(withForeground(lipgloss.NewStyle(), a.Loading.Foreground), a.Loading.Bold)

	t.InputPanel = borderStyle(lipgloss.NewStyle(), a.Input.BorderForeground)
	t.InputPanel = withBackground(t.InputPanel, a.Input.Background, tr)

	// The selection highlight stays even when transparent: it marks the
	// user's own selection, not an application surface.
	t.Selection = lipgloss.NewStyle().
		Background(lipgloss.Color(a.Selection.Background)).
		Foreground(lipgloss.Color(a.Selection.Foreground))

	t.InputPrompt = withBold(withForeground(lipgloss.NewStyle(), a.Input.PromptForeground), a.Input.PromptBold)

	return t
}

// DefaultTheme is the theme built from the default [appearance] values.
func DefaultTheme() Theme {
	return NewTheme(config.DefaultAppearance())
}

// themeOr returns the model's theme, falling back to the default theme for a
// model built as a literal (tests, zero value).
func (m Model) themeOr() Theme {
	if !m.theme.set {
		return defaultTheme
	}
	return m.theme
}

// defaultTheme is the immutable default: built once from the default config.
// Nothing mutates it at runtime; models copy Theme values.
var defaultTheme = NewTheme(config.DefaultAppearance())

// withForeground sets the color unless it is empty ("unset" keeps the
// terminal default and lipgloss's NoColor state).
func withForeground(s lipgloss.Style, color string) lipgloss.Style {
	if color == "" {
		return s
	}
	return s.Foreground(lipgloss.Color(color))
}

// withBackground sets the background color unless it is empty or the caller
// asked for a transparent background.
func withBackground(s lipgloss.Style, color string, transparent bool) lipgloss.Style {
	if color == "" || transparent {
		return s
	}
	return s.Background(lipgloss.Color(color))
}

func withBold(s lipgloss.Style, bold bool) lipgloss.Style {
	if !bold {
		return s
	}
	return s.Bold(true)
}

// surfaceColor resolves the appearance background string for the
// surface-level fill. It parses exactly like every other appearance color
// (lipgloss.Color: ANSI palette number or hex) and returns nil for an empty
// or unparsable value, because lipgloss treats those as "no color" — the
// surface fill must never paint a default background as if it were a color.
func surfaceColor(s string) color.Color {
	if s == "" {
		return nil
	}
	c := lipgloss.Color(s)
	if _, unset := c.(lipgloss.NoColor); unset {
		return nil
	}
	return c
}

func horizontalPadding(s lipgloss.Style, left, right int) lipgloss.Style {
	if left == 0 && right == 0 {
		return s
	}
	return s.Padding(0, right, 0, left)
}

// borderStyle applies the rounded border used by all panels/cards plus its
// foreground color. The border shape is not configurable — only its color.
func borderStyle(s lipgloss.Style, color string) lipgloss.Style {
	s = s.Border(lipgloss.RoundedBorder())
	if color == "" {
		return s
	}
	return s.BorderForeground(lipgloss.Color(color))
}
