package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/y0n1d/trans-tui/internal/core"
)

func TestBuildVisualRows_Ascii(t *testing.T) {
	text := []rune("Hello World")
	rows := buildVisualRows(text, 20)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].StartChar != 0 || rows[0].EndChar != 11 {
		t.Errorf("expected [0,11), got [%d,%d)", rows[0].StartChar, rows[0].EndChar)
	}
}

func TestBuildVisualRows_WrapAtWord(t *testing.T) {
	text := []rune("Hello World Foo Bar")
	rows := buildVisualRows(text, 11)
	if len(rows) < 2 {
		t.Fatalf("expected >= 2 rows, got %d", len(rows))
	}
	extracted := ""
	for _, r := range rows {
		extracted += string(text[r.StartChar:r.EndChar])
	}
	if extracted != "Hello World Foo Bar" {
		t.Errorf("reassembly failed: %q", extracted)
	}
}

func TestBuildVisualRows_Cjk(t *testing.T) {
	text := []rune("こんにち")
	rows := buildVisualRows(text, 6)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	extracted := ""
	for _, r := range rows {
		extracted += string(text[r.StartChar:r.EndChar])
	}
	if extracted != "こんにち" {
		t.Errorf("reassembly failed: %q", extracted)
	}
}

func TestBuildVisualRows_HardWrap(t *testing.T) {
	text := []rune("abcdefghijkl")
	rows := buildVisualRows(text, 5)
	if len(rows) < 2 {
		t.Fatalf("expected >= 2 rows, got %d", len(rows))
	}
	extracted := ""
	for _, r := range rows {
		extracted += string(text[r.StartChar:r.EndChar])
	}
	if extracted != "abcdefghijkl" {
		t.Errorf("reassembly failed: %q", extracted)
	}
}

func TestBuildVisualRows_Empty(t *testing.T) {
	rows := buildVisualRows([]rune(""), 10)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestCellColToCharIndex(t *testing.T) {
	text := []rune("Hello World")
	type tc struct {
		startChar, endChar, targetCell, expected int
	}
	tests := []tc{
		{0, 11, 0, 0},
		{0, 11, 5, 5},
		{0, 11, 11, 11},
		{0, 11, 20, 11},
		{6, 11, 0, 6},
		{6, 11, 5, 11},
	}
	for _, tt := range tests {
		got := cellColToCharIndex(text, tt.startChar, tt.endChar, tt.targetCell)
		if got != tt.expected {
			t.Errorf("cellColToCharIndex(%d,%d,%d) = %d, want %d",
				tt.startChar, tt.endChar, tt.targetCell, got, tt.expected)
		}
	}
}

func TestCellColToCharIndex_Cjk(t *testing.T) {
	text := []rune("こんにち世界")
	got := cellColToCharIndex(text, 0, 7, 4)
	if got != 2 {
		t.Errorf("cellColToCharIndex for CJK: got %d, want 2", got)
	}
}

func TestExtractSelectedText_SingleLine(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
	}, 40)

	if len(model.semRows) == 0 {
		t.Fatal("no semantic rows")
	}

	srcRow := firstSelectableRow(model)
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 5},
		end:   SelectionPoint{VisualRow: srcRow, CellCol: 10},
	}
	text := model.extractSelectedText()
	if text != "Hello" {
		t.Errorf("expected %q, got %q", "Hello", text)
	}
}

func TestExtractSelectedText_FullLine(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hi", TargetLang: "ja", Translation: "你好"},
	}, 40)

	if len(model.semRows) == 0 {
		t.Fatal("no semantic rows")
	}

	srcRow := firstSelectableRow(model)
	row := model.semRows[srcRow]
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 0},
		end:   SelectionPoint{VisualRow: srcRow, CellCol: row.ScreenX1 - row.ScreenX0},
	}
	text := model.extractSelectedText()
	if text != "[en] Hi" {
		t.Errorf("full line selection: expected %q, got %q", "[en] Hi", text)
	}
}

func TestExtractSelectedText_MultiRecord(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "First", TargetLang: "ja", Translation: "一番目"},
		{SourceLang: "en", Source: "Second", TargetLang: "ja", Translation: "二番目"},
	}, 40)

	if len(model.semRows) < 4 {
		t.Fatalf("expected >= 4 semantic rows, got %d", len(model.semRows))
	}

	srcRow := firstSelectableRow(model)
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 0},
		end:   SelectionPoint{VisualRow: srcRow + 1, CellCol: 20},
	}
	text := model.extractSelectedText()
	if text == "" {
		t.Error("expected non-empty cross-line selection")
	}
}

func TestExtractSelectedText_Reverse(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
	}, 40)

	if len(model.semRows) == 0 {
		t.Fatal("no semantic rows")
	}

	srcRow := firstSelectableRow(model)
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 10},
		end:   SelectionPoint{VisualRow: srcRow, CellCol: 5},
	}
	text := model.extractSelectedText()
	if text != "Hello" {
		t.Errorf("reverse selection: expected %q, got %q", "Hello", text)
	}
}

func TestExtractSelectedText_Empty(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	srcRow := firstSelectableRow(model)
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 0},
		end:   SelectionPoint{VisualRow: srcRow, CellCol: 0},
	}
	text := model.extractSelectedText()
	if text != "" {
		t.Errorf("empty selection: expected empty, got %q", text)
	}
}

func TestScreenToSelectionPoint_Border(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	_ = model.renderRecordsWithHighlight()

	pt := model.screenToSelectionPoint(0, 1)
	if pt != nil {
		t.Errorf("click on border should be nil, got %+v", pt)
	}
}

func TestScreenToSelectionPoint_Padding(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	_ = model.renderRecordsWithHighlight()

	pt := model.screenToSelectionPoint(1, 2)
	if pt != nil {
		t.Errorf("click on padding should be nil, got %+v", pt)
	}
}

func TestScreenToSelectionPoint_Content(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	_ = model.renderRecordsWithHighlight()

	pt := model.screenToSelectionPoint(2, 2)
	if pt == nil {
		t.Error("click on content should not be nil")
	}
}

func TestIsRowInSelection(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	_ = model.renderRecordsWithHighlight()

	srcRow := firstSelectableRow(model)
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 0},
		end:   SelectionPoint{VisualRow: srcRow, CellCol: 5},
	}
	if !model.isRowInSelection(srcRow) {
		t.Errorf("row %d should be in selection", srcRow)
	}
	if model.isRowInSelection(srcRow + 1) {
		t.Errorf("row %d should not be in selection", srcRow+1)
	}
}

func TestRowSelectionCellRange(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	_ = model.renderRecordsWithHighlight()

	srcRow := firstSelectableRow(model)
	model.sel = selection{
		start: SelectionPoint{VisualRow: srcRow, CellCol: 2},
		end:   SelectionPoint{VisualRow: srcRow, CellCol: 7},
	}
	startCell, endCell := model.rowSelectionCellRange(srcRow)
	if startCell != 2 || endCell != 7 {
		t.Errorf("expected (2,7), got (%d,%d)", startCell, endCell)
	}
}

func TestRuneWidth(t *testing.T) {
	if got := runeWidth([]rune("Hello")); got != 5 {
		t.Errorf("ASCII: expected 5, got %d", got)
	}
	if got := runeWidth([]rune("你好")); got != 4 {
		t.Errorf("CJK: expected 4, got %d", got)
	}
}

func TestBuildSemanticMap(t *testing.T) {
	model := setupModelWithRecords(t, []core.TranslationRecord{
		{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "你好"},
	}, 40)

	if len(model.semLines) == 0 {
		t.Error("expected semantic lines")
	}
	if len(model.semRows) == 0 {
		t.Error("expected semantic rows")
	}
	for i, row := range model.semRows {
		if !row.Selectable {
			continue
		}
		if row.ScreenX0 >= row.ScreenX1 {
			t.Errorf("selectable row %d: ScreenX0 (%d) >= ScreenX1 (%d)", i, row.ScreenX0, row.ScreenX1)
		}
	}
}

func TestBuildVisualRows_WordBreak(t *testing.T) {
	text := []rune("aa bb cc dd")
	rows := buildVisualRows(text, 6)
	extracted := ""
	for _, r := range rows {
		extracted += string(text[r.StartChar:r.EndChar])
	}
	if extracted != "aa bb cc dd" {
		t.Errorf("word break reassembly: %q", extracted)
	}
}

func setupModelWithRecords(t *testing.T, records []core.TranslationRecord, vpWidth int) Model {
	t.Helper()
	model := Model{
		viewport: viewport.New(vpWidth, 20),
	}
	model.Records = append(model.Records, records...)
	model.buildSemanticMap(records, vpWidth)
	return model
}

func firstSelectableRow(m Model) int {
	for i, row := range m.semRows {
		if row.Selectable {
			return i
		}
	}
	return -1
}
