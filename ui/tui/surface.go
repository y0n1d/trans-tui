package tui

import (
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// surfaceView is the exact string handed to tea.NewView: the assembled
// renderView, with a full terminal-width × terminal-height background
// painted underneath when the surface is opaque.
//
// Rendering path:
//
//	renderView()          — assemble the existing sections (unchanged)
//	                     — if transparent: return the raw view as-is
//	paintSurface(view, W, H, surfaceBackground)
//
// transparent_background = true skips paintSurface entirely: no
// surface-level background sequence is emitted, component backgrounds stay
// dropped by the theme, and the selection highlight keeps its own color.
func (m Model) surfaceView() string {
	view := m.renderView()
	th := m.themeOr()
	if th.SurfaceTransparent || th.SurfaceBackground == nil {
		return view
	}
	return paintSurface(view, m.terminalWidth, m.terminalHeight, th.SurfaceBackground)
}

// paintSurface draws view onto a fixed width × height cell buffer and gives
// every cell an explicit background:
//
//  1. a ScreenBuffer of exactly W × H is created,
//  2. view is drawn into it with ultraviolet's StyledString.Draw — the same
//     parse-and-draw path Bubble Tea's renderer uses for the view string.
//     It never soft-wraps and never truncates within the bounds, so the
//     layout, cursor coordinates and semantic rows are preserved cell for
//     cell; a cell the view does not write stays an empty cell,
//  3. every cell without an explicit background — unwritten cells and
//     cells whose component paints only a foreground — gets the surface
//     background. Cells that already have a component or selection
//     background keep it (priority: surface < component < selection),
//  4. Buffer.Render writes every cell of every row, including the trailing
//     background spaces (it never trims, unlike Buffer.String or
//     TrimSpace-style render paths).
//
// Wide glyphs stay one cell plus zero-width continuation columns, exactly
// as the terminal paints them; the continuation columns inherit the wide
// cell's background and are never emitted as extra columns, so no row can
// grow beyond W.
func paintSurface(view string, width, height int, bg color.Color) string {
	if width <= 0 || height <= 0 || bg == nil {
		return view
	}

	scr := uv.NewScreenBuffer(width, height)
	// The default is WcWidth; the project measures with grapheme widths
	// (lipgloss, x/ansi.StringWidth, the semantic map), so the surface must
	// use the same model or wide cells would land in different columns.
	scr.Method = ansi.GraphemeWidth

	uv.NewStyledString(view).Draw(scr, scr.Bounds())

	// Fill pass: empty (unwritten) cells and nil-background content cells
	// both get the surface color. Zero cells are the continuation columns
	// of wide glyphs: they are never emitted — the terminal paints them
	// from the wide cell itself, which always carries an explicit
	// background — so they are left alone deliberately.
	lines := scr.Lines
	for y := range lines {
		row := lines[y]
		for x := range row {
			cell := &row[x]
			if !cell.IsZero() && cell.Style.Bg == nil {
				cell.Style.Bg = bg
			}
		}
	}

	return scr.Render()
}
