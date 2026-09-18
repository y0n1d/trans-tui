package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"my-trans/internal/core"
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
			m.viewport.SetContent(m.renderRecordsWithHighlight())
			text := m.extractSelectedText()
			if text != "" {
				return m, copyToClipboard(text)
			}
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
		if prevLineID >= 0 && row.LineID != prevLineID {
			result = append(result, '\n')
		}
		if startRune < endRune && startRune >= 0 && endRune <= len(line.text) {
			result = append(result, line.text[startRune:endRune]...)
		}
		prevLineID = row.LineID
	}
	return string(result)
}

func cellColToCharIndex(text []rune, startChar, endChar, targetCell int) int {
	cell := 0
	for i := startChar; i < endChar; i++ {
		if cell >= targetCell {
			return i
		}
		cell += runewidth.RuneWidth(text[i])
	}
	return endChar
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

	addLine := func(recordIndex int, text string, selectable bool) {
		runes := []rune(text)
		lineID := len(lines)
		lines = append(lines, semanticLine{text: runes, recordIndex: recordIndex, selectable: selectable})
		for _, vr := range buildVisualRows(runes, wrapWidth) {
			cellWidth := 0
			for r := vr.StartChar; r < vr.EndChar; r++ {
				cellWidth += runewidth.RuneWidth(runes[r])
			}
			rows = append(rows, semanticRow{
				LineID:     lineID,
				StartChar:  vr.StartChar,
				EndChar:    vr.EndChar,
				ScreenX0:   leftOffset,
				ScreenX1:   leftOffset + cellWidth,
				Selectable: selectable,
			})
		}
	}

	for i, rec := range records {
		addPlaceholder() // record top border
		addLine(i, fmt.Sprintf("[%s] %s", rec.SourceLang, rec.Source), true)
		if rec.Error != "" {
			addLine(i, fmt.Sprintf("Error: %s", rec.Error), true)
		} else if rec.Translation != "" {
			addLine(i, fmt.Sprintf("[%s] %s", rec.TargetLang, rec.Translation), true)
		}
		if rec.Provider != "" && rec.Model != "" {
			addLine(i, fmt.Sprintf("via %s/%s", rec.Provider, rec.Model), false)
		}
		addPlaceholder() // record bottom border
	}

	m.semLines = lines
	m.semRows = rows
}

func buildVisualRows(text []rune, cellWidth int) []charSpan {
	var rows []charSpan
	pos := 0
	cell := 0
	lineStart := 0

	for pos < len(text) {
		r := text[pos]
		w := runewidth.RuneWidth(r)

		if cell+w > cellWidth && lineStart < pos {
			rows = append(rows, charSpan{lineStart, pos})
			lineStart = pos
			cell = 0
		}

		cell += w
		pos++
	}

	if lineStart < len(text) {
		rows = append(rows, charSpan{lineStart, len(text)})
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

func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		seq := osc52.New(text)
		fmt.Fprint(os.Stderr, seq.String())
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
	var parts []string

	srcContent := fmt.Sprintf("[%s] %s", record.SourceLang, record.Source)
	parts = append(parts, m.renderRecordLine(recordIndex, srcContent, SourceStyle))

	if record.Error != "" {
		errContent := fmt.Sprintf("Error: %s", record.Error)
		parts = append(parts, m.renderRecordLine(recordIndex, errContent, ErrorStyle))
	} else if record.Translation != "" {
		transContent := fmt.Sprintf("[%s] %s", record.TargetLang, record.Translation)
		parts = append(parts, m.renderRecordLine(recordIndex, transContent, TranslationStyle))
	}

	if record.Provider != "" && record.Model != "" {
		provContent := fmt.Sprintf("via %s/%s", record.Provider, record.Model)
		parts = append(parts, m.renderRecordLine(recordIndex, provContent, StatusBarStyle))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return style.Render(content)
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
