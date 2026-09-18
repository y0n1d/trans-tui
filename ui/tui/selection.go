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
	text []rune
}

type semanticRow struct {
	LineID    int
	StartChar int
	EndChar   int
	ScreenX0  int
	ScreenX1  int
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
		if row.LineID >= len(m.semLines) {
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
	lineID := 0

	for _, rec := range records {
		srcText := fmt.Sprintf("[%s] %s", rec.SourceLang, rec.Source)
		srcRunes := []rune(srcText)
		srcVRows := buildVisualRows(srcRunes, wrapWidth)
		lines = append(lines, semanticLine{text: srcRunes})
		for _, vr := range srcVRows {
			cellWidth := 0
			for r := vr.StartChar; r < vr.EndChar; r++ {
				cellWidth += runewidth.RuneWidth(srcRunes[r])
			}
			rows = append(rows, semanticRow{
				LineID:    lineID,
				StartChar: vr.StartChar,
				EndChar:   vr.EndChar,
				ScreenX0:  leftOffset,
				ScreenX1:  leftOffset + cellWidth,
			})
		}
		lineID++

		var content string
		if rec.Error != "" {
			content = fmt.Sprintf("Error: %s", rec.Error)
		} else {
			content = fmt.Sprintf("[%s] %s", rec.TargetLang, rec.Translation)
		}
		contentRunes := []rune(content)
		contentVRows := buildVisualRows(contentRunes, wrapWidth)
		lines = append(lines, semanticLine{text: contentRunes})
		for _, vr := range contentVRows {
			cellWidth := 0
			for r := vr.StartChar; r < vr.EndChar; r++ {
				cellWidth += runewidth.RuneWidth(contentRunes[r])
			}
			rows = append(rows, semanticRow{
				LineID:    lineID,
				StartChar: vr.StartChar,
				EndChar:   vr.EndChar,
				ScreenX0:  leftOffset,
				ScreenX1:  leftOffset + cellWidth,
			})
		}
		lineID++
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
	for _, record := range m.Records {
		records = append(records, m.renderRecordHighlighted(record, w))
	}
	return lipgloss.JoinVertical(lipgloss.Left, records...)
}

func (m Model) renderRecordHighlighted(record core.TranslationRecord, viewportWidth int) string {
	contentWidth := viewportWidth - recordBorderPadding
	if contentWidth < 1 {
		contentWidth = 1
	}
	style := RecordStyle.Width(contentWidth)
	var parts []string

	srcContent := fmt.Sprintf("[%s] %s", record.SourceLang, record.Source)
	parts = append(parts, m.renderContentLine(srcContent, SourceStyle))

	if record.Error != "" {
		errContent := fmt.Sprintf("Error: %s", record.Error)
		parts = append(parts, m.renderContentLine(errContent, ErrorStyle))
	} else if record.Translation != "" {
		transContent := fmt.Sprintf("[%s] %s", record.TargetLang, record.Translation)
		parts = append(parts, m.renderContentLine(transContent, TranslationStyle))
	}

	if record.Provider != "" && record.Model != "" {
		provContent := fmt.Sprintf("via %s/%s", record.Provider, record.Model)
		parts = append(parts, m.renderContentLine(provContent, StatusBarStyle))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return style.Render(content)
}

func (m Model) renderContentLine(content string, baseStyle lipgloss.Style) string {
	vi := m.findVisualRowForContent(content)
	if vi < 0 || !m.isRowInSelection(vi) {
		return baseStyle.Render(content)
	}
	startCell, endCell := m.rowSelectionCellRange(vi)
	contentRunes := []rune(content)
	startRune := cellColToCharIndex(contentRunes, 0, len(contentRunes), startCell)
	endRune := cellColToCharIndex(contentRunes, 0, len(contentRunes), endCell)
	if startRune >= endRune {
		return baseStyle.Render(content)
	}
	prefix := string(contentRunes[:startRune])
	highlighted := SelectionHighlightStyle.Render(string(contentRunes[startRune:endRune]))
	suffix := string(contentRunes[endRune:])
	return baseStyle.Render(prefix) + highlighted + baseStyle.Render(suffix)
}

func (m Model) findVisualRowForContent(content string) int {
	contentRunes := []rune(content)
	for vi, row := range m.semRows {
		if row.LineID >= len(m.semLines) {
			continue
		}
		line := m.semLines[row.LineID]
		if row.StartChar == 0 && row.EndChar == len(line.text) &&
			string(line.text) == string(contentRunes) {
			return vi
		}
	}
	for vi, row := range m.semRows {
		if row.LineID >= len(m.semLines) {
			continue
		}
		line := m.semLines[row.LineID]
		if row.StartChar == 0 && strings.HasPrefix(string(line.text), string(contentRunes)) {
			return vi
		}
	}
	return -1
}
