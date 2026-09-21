package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/y0n1d/trans-tui/internal/core"
)

type SelectionPoint struct {
	VisualRow int
	CellCol   int
}

type selection struct {
	selecting bool
	start     SelectionPoint
	end       SelectionPoint
}

type semanticLine struct {
	text        []rune
	recordIndex int
	selectable  bool
}

type semanticRow struct {
	LineID     int
	StartChar  int
	EndChar    int
	ScreenX0   int
	ScreenX1   int
	Selectable bool
}

type charSpan struct {
	StartChar int
	EndChar   int
}

var SelectionHighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("240")).
	Foreground(lipgloss.Color("15"))

func (m Model) screenToSelectionPoint(x, y int) *SelectionPoint {
	if len(m.semRows) == 0 {
		return nil
	}
	vpLocalY := y - m.headerHeight()
	vpLocalX := x
	if vpLocalY < 0 || vpLocalY >= m.viewport.Height {
		return nil
	}
	vr := vpLocalY + m.viewport.YOffset
	if vr < 0 || vr >= len(m.semRows) {
		return nil
	}
	row := m.semRows[vr]
	if !row.Selectable {
		return nil
	}
	if vpLocalX < row.ScreenX0 || vpLocalX >= row.ScreenX1 {
		return nil
	}
	return &SelectionPoint{
		VisualRow: vr,
		CellCol:   vpLocalX - row.ScreenX0,
	}
}

func (m Model) clampDragPoint(x, y int) *SelectionPoint {
	if len(m.semRows) == 0 {
		return nil
	}
	vpLocalY := y - m.headerHeight()
	vpLocalX := x
	vr := vpLocalY + m.viewport.YOffset
	if vr < 0 {
		vr = 0
	}
	if vr >= len(m.semRows) {
		vr = len(m.semRows) - 1
	}
	row := m.semRows[vr]
	cellCol := vpLocalX - row.ScreenX0
	if cellCol < 0 {
		cellCol = 0
	}
	maxCell := row.ScreenX1 - row.ScreenX0
	if cellCol > maxCell {
		cellCol = maxCell
	}
	return &SelectionPoint{
		VisualRow: vr,
		CellCol:   cellCol,
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			pt := m.screenToSelectionPoint(msg.X, msg.Y)
			if pt == nil {
				return m, nil
			}
			m.sel.selecting = true
			m.sel.start = *pt
			m.sel.end = *pt
			m.viewport.SetContent(m.renderRecordsWithHighlight())
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp {
			m.viewport.LineUp(3)
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.viewport.LineDown(3)
			return m, nil
		}
		return m, nil

	case tea.MouseActionMotion:
		if m.sel.selecting {
			pt := m.clampDragPoint(msg.X, msg.Y)
			if pt != nil {
				m.sel.end = *pt
				m.viewport.SetContent(m.renderRecordsWithHighlight())
			}
		}
		return m, nil

	case tea.MouseActionRelease:
		if msg.Button == tea.MouseButtonLeft && m.sel.selecting {
			m.sel.selecting = false
			pt := m.clampDragPoint(msg.X, msg.Y)
			if pt != nil {
				m.sel.end = *pt
			}

			// Extract and copy before clearing the selection, because
			// extractSelectedText reads m.sel.
			text := m.extractSelectedText()
			var cmd tea.Cmd
			if text != "" {
				cmd = m.copyToClipboard(text)
			}

			// Releasing always ends the selection lifecycle: the highlight
			// disappears immediately, no Escape or second click required.
			m.sel = selection{}
			m.viewport.SetContent(m.renderRecordsWithHighlight())
			return m, cmd
		}
		return m, nil
	}
	return m, nil
}

func (m Model) extractSelectedText() string {
	if m.sel.start == m.sel.end {
		return ""
	}
	s, e := m.sel.start, m.sel.end
	if s.VisualRow > e.VisualRow || (s.VisualRow == e.VisualRow && s.CellCol > e.CellCol) {
		s, e = e, s
	}
	var result []rune
	prevLineID := -1
	prevEndChar := 0
	for vi := s.VisualRow; vi <= e.VisualRow; vi++ {
		if vi >= len(m.semRows) {
			break
		}
		row := m.semRows[vi]
		if !row.Selectable {
			continue
		}
		line := m.semLines[row.LineID]
		var startRune, endRune int
		if vi == s.VisualRow && vi == e.VisualRow {
			startRune = cellColToCharIndex(line.text, row.StartChar, row.EndChar, s.CellCol)
			endRune = cellColToCharIndex(line.text, row.StartChar, row.EndChar, e.CellCol)
		} else if vi == s.VisualRow {
			startRune = cellColToCharIndex(line.text, row.StartChar, row.EndChar, s.CellCol)
			endRune = row.EndChar
		} else if vi == e.VisualRow {
			startRune = row.StartChar
			endRune = cellColToCharIndex(line.text, row.StartChar, row.EndChar, e.CellCol)
		} else {
			startRune = row.StartChar
			endRune = row.EndChar
		}
		if prevLineID >= 0 {
			if row.LineID != prevLineID {
				result = append(result, '\n')
			} else if row.StartChar > prevEndChar {
				result = append(result, '\n')
			}
		}
		if startRune < endRune && startRune >= 0 && endRune <= len(line.text) {
			result = append(result, line.text[startRune:endRune]...)
		}
		prevLineID = row.LineID
		prevEndChar = row.EndChar
	}
	return string(result)
}

func cellColToCharIndex(text []rune, startChar, endChar, targetCell int) int {
	if startChar < 0 {
		startChar = 0
	}
	if endChar > len(text) {
		endChar = len(text)
	}
	cell := 0
	i := startChar
	for i < endChar {
		if cell >= targetCell {
			return i
		}
		n, w := firstGraphemeWidth(text[i:endChar])
		if n < 1 {
			break
		}
		i += n
		cell += w
	}
	return endChar
}

// firstGraphemeWidth returns the number of runes and the display width of the
// first grapheme cluster in text. It uses the same width model as lipgloss
// (charmbracelet/x/ansi grapheme widths) so the semantic map and the renderer
// never disagree about how wide a cell sequence is.
func firstGraphemeWidth(text []rune) (runeCount, width int) {
	if len(text) == 0 {
		return 0, 0
	}
	cluster, w := xansi.FirstGraphemeCluster(string(text), xansi.GraphemeWidth)
	if cluster == "" {
		return 0, 0
	}
	return len([]rune(cluster)), w
}

// displayWidth returns the terminal cell width of text using the same
// grapheme-aware width model as lipgloss.
func displayWidth(text []rune) int {
	if len(text) == 0 {
		return 0
	}
	return xansi.StringWidth(string(text))
}

func (m *Model) buildSemanticMap(records []core.TranslationRecord, viewportWidth int) {
	contentWidth := viewportWidth - recordBorderPadding
	if contentWidth < 1 {
		contentWidth = 1
	}
	wrapWidth := contentWidth - 2
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	leftOffset := 2

	var lines []semanticLine
	var rows []semanticRow

	addPlaceholder := func() {
		rows = append(rows, semanticRow{LineID: -1, Selectable: false})
	}

	// addLine records one logical line. leftPad/rightPad describe the
	// horizontal padding the line's render style will add inside the record
	// content area (source/translation/error use none; the provider row uses
	// StatusBarStyle's Padding(0,1)). The padding consumes render width, so
	// the text must wrap that much earlier; otherwise lipgloss would wrap the
	// padded line into an extra visual row the semantic map does not know
	// about, shifting every subsequent row.
	addLine := func(recordIndex int, text string, selectable bool, leftPad, rightPad int) {
		runes := []rune(sanitizeDisplay(text))
		lineID := len(lines)
		lines = append(lines, semanticLine{text: runes, recordIndex: recordIndex, selectable: selectable})
		lineWrap := wrapWidth - leftPad - rightPad
		if lineWrap < 1 {
			lineWrap = 1
		}
		for _, vr := range buildVisualRows(runes, lineWrap) {
			cellWidth := displayWidth(runes[vr.StartChar:vr.EndChar])
			rows = append(rows, semanticRow{
				LineID:     lineID,
				StartChar:  vr.StartChar,
				EndChar:    vr.EndChar,
				ScreenX0:   leftOffset + leftPad,
				ScreenX1:   leftOffset + leftPad + cellWidth,
				Selectable: selectable,
			})
		}
	}

	for i, rec := range records {
		addPlaceholder() // record top border
		addLine(i, rec.Source, true, 0, 0)
		if rec.Error != "" {
			addLine(i, fmt.Sprintf("Error: %s", rec.Error), true, 0, 0)
		} else if rec.Translation != "" {
			addLine(i, rec.Translation, true, 0, 0)
		}
		if rec.Provider != "" && rec.Model != "" {
			// StatusBarStyle adds Padding(0, 1).
			addLine(i, fmt.Sprintf("via %s/%s", rec.Provider, rec.Model), false, 1, 1)
		}
		addPlaceholder() // record bottom border
	}

	m.semLines = lines
	m.semRows = rows
}

// sanitizeDisplay normalizes text before it is measured, wrapped and rendered.
// buildSemanticMap and renderRecordHighlighted must both call this so the
// semantic row layout and the rendered layout stay in the same coordinate
// system. It:
//   - converts CRLF and lone CR to LF (a bare '\r' inside a bordered line
//     makes the terminal return to column 0 and overwrite the border),
//   - expands tabs to spaces so runewidth, lipgloss and the terminal agree on
//     tab width.
func sanitizeDisplay(text string) string {
	if !strings.ContainsAny(text, "\r\t") {
		return text
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var b strings.Builder
	b.Grow(len(text))
	col := 0
	for _, r := range text {
		switch r {
		case '\n':
			b.WriteRune(r)
			col = 0
		case '\t':
			n := 8 - (col % 8)
			b.WriteString(strings.Repeat(" ", n))
			col += n
		default:
			b.WriteRune(r)
			col += runewidth.RuneWidth(r)
		}
	}
	return b.String()
}

func buildVisualRows(text []rune, cellWidth int) []charSpan {
	var rows []charSpan
	pos := 0
	cell := 0
	lineStart := 0

	for pos < len(text) {
		r := text[pos]

		if r == '\n' {
			rows = append(rows, charSpan{lineStart, pos})
			pos++
			lineStart = pos
			cell = 0
			continue
		}

		// Advance by whole grapheme clusters, not runes. Splitting inside a
		// cluster (for example a regional-indicator flag) changes its measured
		// width and would let lipgloss wrap a row the semantic map thought fit.
		n, w := firstGraphemeWidth(text[pos:])
		if n < 1 {
			n = 1
		}

		if cell+w > cellWidth && lineStart < pos {
			rows = append(rows, charSpan{lineStart, pos})
			lineStart = pos
			cell = 0
		}

		cell += w
		pos += n
	}

	if lineStart < len(text) {
		rows = append(rows, charSpan{lineStart, len(text)})
	} else if len(text) > 0 && text[len(text)-1] == '\n' {
		// A trailing newline denotes a final empty line; emit it so the
		// semantic map, the renderer and extraction agree on the row count.
		rows = append(rows, charSpan{len(text), len(text)})
	}
	return rows
}

func runeWidth(text []rune) int {
	w := 0
	for _, r := range text {
		w += runewidth.RuneWidth(r)
	}
	return w
}

// clipboardWrite writes text to the system clipboard and reports whether the
// write succeeded. It is stored on Model so tests can inject a fake writer
// instead of touching the real clipboard.
type clipboardWrite func(text string) error

// clipboardErrorMsg is emitted when a clipboard write fails so the UI can
// surface it without blocking the selection lifecycle.
type clipboardErrorMsg struct{ err error }

// osc52ClipboardWrite is the production clipboard writer. It emits an OSC 52
// sequence on stderr, which the terminal turns into a clipboard write.
func osc52ClipboardWrite(text string) error {
	_, err := fmt.Fprint(os.Stderr, osc52.New(text).String())
	return err
}

// copyToClipboard returns a command that writes text with the injected
// clipboard writer, falling back to OSC 52 when none is set.
func (m Model) copyToClipboard(text string) tea.Cmd {
	write := m.clipboard
	if write == nil {
		write = osc52ClipboardWrite
	}
	return func() tea.Msg {
		if err := write(text); err != nil {
			return clipboardErrorMsg{err: err}
		}
		return nil
	}
}

func (m Model) isRowInSelection(vi int) bool {
	if m.sel.start == m.sel.end && !m.sel.selecting {
		return false
	}
	s, e := m.sel.start, m.sel.end
	if s.VisualRow > e.VisualRow || (s.VisualRow == e.VisualRow && s.CellCol > e.CellCol) {
		s, e = e, s
	}
	return vi >= s.VisualRow && vi <= e.VisualRow
}

func (m Model) rowSelectionCellRange(vi int) (startCell, endCell int) {
	s, e := m.sel.start, m.sel.end
	if s.VisualRow > e.VisualRow || (s.VisualRow == e.VisualRow && s.CellCol > e.CellCol) {
		s, e = e, s
	}
	if vi < 0 || vi >= len(m.semRows) {
		return 0, 0
	}
	row := m.semRows[vi]
	contentWidth := row.ScreenX1 - row.ScreenX0
	if vi == s.VisualRow && vi == e.VisualRow {
		if s.CellCol == e.CellCol {
			return s.CellCol, s.CellCol + 1
		}
		return s.CellCol, e.CellCol
	} else if vi == s.VisualRow {
		return s.CellCol, contentWidth
	} else if vi == e.VisualRow {
		return 0, e.CellCol
	}
	return 0, contentWidth
}

func (m Model) renderRecordsWithHighlight() string {
	w := m.viewport.Width
	if w <= 0 {
		w = 80
	}
	if len(m.Records) == 0 {
		return StatusBarStyle.Render("No translations yet. Type text to translate.")
	}

	m.buildSemanticMap(m.Records, w)

	var records []string
	for i, record := range m.Records {
		records = append(records, m.renderRecordHighlighted(record, w, i))
	}
	return lipgloss.JoinVertical(lipgloss.Left, records...)
}

func (m Model) renderRecordHighlighted(record core.TranslationRecord, viewportWidth, recordIndex int) string {
	contentWidth := viewportWidth - recordBorderPadding
	if contentWidth < 1 {
		contentWidth = 1
	}
	style := RecordStyle.Width(contentWidth)

	// Build the top border line with the language label embedded.
	// The top border must match the full outer card width (viewportWidth),
	// not contentWidth, because the border characters themselves occupy
	// the outer edge of the card.
	topBorder := m.renderTopBorderLine(record, viewportWidth)

	var parts []string

	srcContent := sanitizeDisplay(record.Source)
	parts = append(parts, m.renderRecordLine(recordIndex, srcContent, SourceStyle))

	if record.Error != "" {
		errContent := sanitizeDisplay(fmt.Sprintf("Error: %s", record.Error))
		parts = append(parts, m.renderRecordLine(recordIndex, errContent, ErrorStyle))
	} else if record.Translation != "" {
		transContent := sanitizeDisplay(record.Translation)
		parts = append(parts, m.renderRecordLine(recordIndex, transContent, TranslationStyle))
	}

	if record.Provider != "" && record.Model != "" {
		provContent := sanitizeDisplay(fmt.Sprintf("via %s/%s", record.Provider, record.Model))
		parts = append(parts, m.renderRecordLine(recordIndex, provContent, StatusBarStyle))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	bordered := style.Render(content)

	// Replace the standard top border with our custom one that embeds the
	// language label (e.g. ┌────[en] → [ja]────┐).
	lines := strings.Split(bordered, "\n")
	if len(lines) > 0 {
		lines[0] = topBorder
	}
	return strings.Join(lines, "\n")
}

// renderTopBorderLine builds the top border line of a card with the language
// label embedded in it. For example: ┌────[en] → [ja]────┐
// The label is placed after the top-left corner, with horizontal border
// characters filling the remaining space.
// outerWidth is the full display width of the card including border characters
// (i.e. viewportWidth, NOT contentWidth).
func (m Model) renderTopBorderLine(record core.TranslationRecord, outerWidth int) string {
	borders := RecordStyle.GetBorderStyle()
	leftChar := borders.TopLeft
	rightChar := borders.TopRight
	horizChar := borders.Top

	label := fmt.Sprintf("[%s] → [%s]", record.SourceLang, record.TargetLang)
	labelWidth := xansi.StringWidth(label)

	// Available space between the corner characters.
	avail := outerWidth
	if avail < 2 {
		avail = 2
	}
	dashCount := avail - 2 // subtract left and right corner chars
	if dashCount < 0 {
		dashCount = 0
	}

	fg := RecordStyle.GetBorderTopForeground()
	borderTextStyle := lipgloss.NewStyle().Foreground(fg)

	if labelWidth+2 > avail {
		// Label doesn't fit even with minimum one dash on each side.
		return borderTextStyle.Render(string(leftChar) +
			strings.Repeat(string(horizChar), dashCount) +
			string(rightChar))
	}

	leftDashes := (dashCount - labelWidth) / 2
	rightDashes := dashCount - labelWidth - leftDashes
	if leftDashes < 0 {
		leftDashes = 0
	}
	if rightDashes < 0 {
		rightDashes = 0
	}

	return borderTextStyle.Render(string(leftChar) +
		strings.Repeat(string(horizChar), leftDashes) +
		label +
		strings.Repeat(string(horizChar), rightDashes) +
		string(rightChar))
}

// renderRecordLine renders one logical line, split into the visual rows recorded
// in the semantic map, applying the current selection highlight to each row.
func (m Model) renderRecordLine(recordIndex int, content string, baseStyle lipgloss.Style) string {
	lineID := m.findSemanticLineID(recordIndex, content)
	if lineID < 0 {
		return baseStyle.Render(content)
	}
	line := m.semLines[lineID]
	var parts []string
	for vi, row := range m.semRows {
		if row.LineID != lineID {
			continue
		}
		rowRunes := line.text[row.StartChar:row.EndChar]
		if !row.Selectable || !m.isRowInSelection(vi) {
			parts = append(parts, baseStyle.Render(string(rowRunes)))
			continue
		}
		startCell, endCell := m.rowSelectionCellRange(vi)
		startRune := cellColToCharIndex(line.text, row.StartChar, row.EndChar, startCell) - row.StartChar
		endRune := cellColToCharIndex(line.text, row.StartChar, row.EndChar, endCell) - row.StartChar
		if startRune < 0 {
			startRune = 0
		}
		if endRune > len(rowRunes) {
			endRune = len(rowRunes)
		}
		if startRune >= endRune {
			parts = append(parts, baseStyle.Render(string(rowRunes)))
			continue
		}
		parts = append(parts,
			baseStyle.Render(string(rowRunes[:startRune]))+
				SelectionHighlightStyle.Render(string(rowRunes[startRune:endRune]))+
				baseStyle.Render(string(rowRunes[endRune:])))
	}
	return strings.Join(parts, "\n")
}

func (m Model) findSemanticLineID(recordIndex int, content string) int {
	contentRunes := []rune(content)
	for i, line := range m.semLines {
		if line.recordIndex == recordIndex && string(line.text) == string(contentRunes) {
			return i
		}
	}
	return -1
}
