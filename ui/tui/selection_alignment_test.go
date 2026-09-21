package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/y0n1d/trans-tui/internal/core"
)

// alignmentInputs covers the cases that historically broke the semantic map:
// hard newlines, empty lines, trailing newlines, auto-wrap, CJK, tabs, CRLF
// and provider rows.
var alignmentInputs = []struct {
	name string
	rec  core.TranslationRecord
}{
	{"no newline", core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "zh", Translation: "你好世界"}},
	{"hard newlines", core.TranslationRecord{SourceLang: "en", Source: "line1\nline2\nline3\nline4", TargetLang: "zh", Translation: "A\nB\nC\nD"}},
	{"empty middle line", core.TranslationRecord{SourceLang: "en", Source: "a\n\nb", TargetLang: "zh", Translation: "x\ny"}},
	{"only newline", core.TranslationRecord{SourceLang: "en", Source: "\n", TargetLang: "zh", Translation: "A"}},
	{"double newline", core.TranslationRecord{SourceLang: "en", Source: "\n\n", TargetLang: "zh", Translation: "A"}},
	{"trailing newline", core.TranslationRecord{SourceLang: "en", Source: "line1\n", TargetLang: "zh", Translation: "A"}},
	{"leading newline", core.TranslationRecord{SourceLang: "en", Source: "\nline2", TargetLang: "zh", Translation: "A"}},
	{"newline and wrap", core.TranslationRecord{SourceLang: "en", Source: "very long line that wraps many times indeed yes\nsecond line", TargetLang: "zh", Translation: "first wrapped translation line that is also long"}},
	{"cjk newlines", core.TranslationRecord{SourceLang: "zh", Source: "中文第一行\n中文第二行\n中文第三行", TargetLang: "en", Translation: "first\nsecond\nthird"}},
	{"tabs", core.TranslationRecord{SourceLang: "en", Source: "tab\there and more\nnext\tline", TargetLang: "zh", Translation: "A\tB"}},
	{"crlf", core.TranslationRecord{SourceLang: "en", Source: "line1\r\nline2\r\nline3\r\nline4", TargetLang: "zh", Translation: "A\r\nB\r\nC\r\nD"}},
	{"provider", core.TranslationRecord{SourceLang: "en", Source: "line1\nline2\nline3\nline4", TargetLang: "zh", Translation: "A\nB\nC\nD", Provider: "openai-compatible", Model: "deepseek-chat"}},
	{"error", core.TranslationRecord{SourceLang: "en", Source: "line1\nline2", Error: "network timeout while contacting the upstream provider"}},
	{"emoji zwj", core.TranslationRecord{SourceLang: "en", Source: "family 👨‍👩‍👧‍👦 here\nnext 🚀 line", TargetLang: "zh", Translation: "emoji 🎉 translation"}},
	{"flag", core.TranslationRecord{SourceLang: "en", Source: "flag 🇨🇳 test line\nnext flag 🇯🇵", TargetLang: "zh", Translation: "A\nB"}},
	{"combining", core.TranslationRecord{SourceLang: "en", Source: "cafe\u0301 line with accents\nnext", TargetLang: "zh", Translation: "A\nB"}},
}

var alignmentWidths = []int{8, 10, 12, 15, 20, 24, 30, 40, 60, 80}

// TestSemanticRowsMatchRenderedRows asserts the fundamental invariant:
//
//	semantic row count == rendered visual row count
//
// for every input and viewport width. This is what guarantees that mouse Y
// maps to the same line the user sees.
func TestSemanticRowsMatchRenderedRows(t *testing.T) {
	for _, in := range alignmentInputs {
		for _, w := range alignmentWidths {
			model := Model{viewport: viewportForTest(w, 60)}
			model.Records = []core.TranslationRecord{in.rec}
			model.buildSemanticMap(model.Records, w)

			rendered := model.renderRecordHighlighted(in.rec, w, 0)
			if got := lipgloss.Height(rendered); got != len(model.semRows) {
				t.Errorf("%s w=%d: rendered rows=%d != semRows=%d", in.name, w, got, len(model.semRows))
			}
			full := model.renderRecordsWithHighlight()
			if got := lipgloss.Height(full); got != len(model.semRows) {
				t.Errorf("%s w=%d: full rendered rows=%d != semRows=%d", in.name, w, got, len(model.semRows))
			}
			for i, line := range strings.Split(full, "\n") {
				if lw := lipgloss.Width(line); lw > w {
					t.Errorf("%s w=%d: line %d width %d overflows viewport", in.name, w, i, lw)
				}
			}
		}
	}
}

// TestSemanticRowsMatchRenderedRowsMultiRecord guards the accumulating-offset
// failure: a mismatch in an earlier record (for example a wrapped provider
// row) must not shift the rows of later records.
func TestSemanticRowsMatchRenderedRowsMultiRecord(t *testing.T) {
	records := []core.TranslationRecord{
		{SourceLang: "en", Source: "line1\nline2\nline3\nline4", TargetLang: "zh", Translation: "A\nB\nC\nD", Provider: "openai-compatible", Model: "deepseek-chat"},
		{SourceLang: "zh", Source: "中文第一行\n中文第二行\n中文第三行", TargetLang: "en", Translation: "first\nsecond\nthird", Provider: "openai-compatible", Model: "deepseek-chat"},
		{SourceLang: "en", Source: "a\n\nb", TargetLang: "zh", Translation: "x\ny", Provider: "p", Model: "m"},
	}
	for _, w := range alignmentWidths {
		model := Model{viewport: viewportForTest(w, 60)}
		model.Records = append([]core.TranslationRecord(nil), records...)
		model.buildSemanticMap(model.Records, w)
		rendered := model.renderRecordsWithHighlight()
		if got := lipgloss.Height(rendered); got != len(model.semRows) {
			t.Errorf("w=%d: rendered rows=%d != semRows=%d", w, got, len(model.semRows))
		}
	}
}

// TestSelectionHighlightLandsOnSamePhysicalRow verifies that selecting a
// semantic row highlights exactly that physical rendered line for every
// selectable row, including newline, wrap and multi-record layouts.
func TestSelectionHighlightLandsOnSamePhysicalRow(t *testing.T) {
	records := []core.TranslationRecord{
		{SourceLang: "en", Source: "line1\nline2\nline3\nline4", TargetLang: "zh", Translation: "A\nB\nC\nD", Provider: "openai-compatible", Model: "deepseek-chat"},
		{SourceLang: "en", Source: "a\n\nb", TargetLang: "zh", Translation: "x\ny"},
	}
	for _, w := range []int{12, 20, 30, 40, 80} {
		model := Model{viewport: viewportForTest(w, 60)}
		model.Records = append([]core.TranslationRecord(nil), records...)
		model.buildSemanticMap(model.Records, w)

		base := strings.Split(model.renderRecordsWithHighlight(), "\n")
		for vi, row := range model.semRows {
			if !row.Selectable || row.ScreenX1 <= row.ScreenX0 {
				continue
			}
			selected := model
			selected.sel = selection{
				selecting: true,
				start:     SelectionPoint{VisualRow: vi, CellCol: 0},
				end:       SelectionPoint{VisualRow: vi, CellCol: 1},
			}
			changed := strings.Split(selected.renderRecordsWithHighlight(), "\n")
			hit := -1
			for i := range base {
				if i < len(changed) && base[i] != changed[i] {
					if hit >= 0 {
						hit = -2
						break
					}
					hit = i
				}
			}
			if hit != vi {
				t.Errorf("w=%d: selecting semRow %d highlighted physical row %d", w, vi, hit)
			}
		}
	}
}

// TestExtractSelectedTextRoundTripsSource selects the whole source of a record
// and asserts it equals the (sanitized) "[lang] source" text, across hard
// newlines, empty lines and auto-wrap.
func TestExtractSelectedTextRoundTripsSource(t *testing.T) {
	records := []core.TranslationRecord{
		{SourceLang: "en", Source: "line1\nline2\nline3\nline4"},
		{SourceLang: "en", Source: "a\n\nb"},
		{SourceLang: "en", Source: "very long line that wraps many times indeed yes\nsecond line"},
		{SourceLang: "zh", Source: "中文第一行\n中文第二行\n中文第三行"},
		{SourceLang: "en", Source: "line1\n"},
		{SourceLang: "en", Source: "\nline2"},
	}
	for _, rec := range records {
		for _, w := range alignmentWidths {
			rec.TargetLang = "zh"
			rec.Translation = "A\nB"
			model := Model{viewport: viewportForTest(w, 60)}
			model.Records = []core.TranslationRecord{rec}
			model.buildSemanticMap(model.Records, w)

			first, last := -1, -1
			for i, r := range model.semRows {
				if r.Selectable && r.LineID == 0 {
					if first < 0 {
						first = i
					}
					last = i
				}
			}
			if first < 0 {
				t.Fatalf("w=%d src=%q: no source rows", w, rec.Source)
			}
			lastRow := model.semRows[last]
			model.sel = selection{
				selecting: true,
				start:     SelectionPoint{VisualRow: first, CellCol: 0},
				end:       SelectionPoint{VisualRow: last, CellCol: lastRow.ScreenX1 - lastRow.ScreenX0},
			}
			got := model.extractSelectedText()
			want := sanitizeDisplay(rec.Source)
			if got != want {
				t.Errorf("w=%d src=%q: extract = %q, want %q", w, rec.Source, got, want)
			}
		}
	}
}

func TestSanitizeDisplay(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"a\r\nb", "a\nb"},
		{"a\rb", "a\nb"},
		{"a\tb", "a       b"},                // tab advances to column 8
		{"12345678\tx", "12345678        x"}, // full tab stop
		{"a\tb\nc\td", "a       b\nc       d"},
		{"中文\tx", "中文    x"}, // CJK counts as 2 cells
	}
	for _, tc := range cases {
		if got := sanitizeDisplay(tc.in); got != tc.want {
			t.Errorf("sanitizeDisplay(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildVisualRows_TrailingNewline(t *testing.T) {
	rows := buildVisualRows([]rune("line1\n"), 80)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for trailing newline, got %d", len(rows))
	}
	if string([]rune("line1\n")[rows[0].StartChar:rows[0].EndChar]) != "line1" {
		t.Errorf("row 0 = %q, want %q", string([]rune("line1\n")[rows[0].StartChar:rows[0].EndChar]), "line1")
	}
	if rows[1].StartChar != rows[1].EndChar {
		t.Errorf("row 1 should be empty, got [%d,%d)", rows[1].StartChar, rows[1].EndChar)
	}
}
