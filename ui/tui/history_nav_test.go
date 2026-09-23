package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/core"
)

// historyNavTestModel returns a normal-mode model with three history records
// of different heights plus a short viewport, so every record start is a
// reachable scroll offset. Layout comes exclusively from the production
// refreshHistory / semantic map path.
func historyNavTestModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel(t)
	// Terminal height 8 => history viewport height 6, which keeps every
	// record start below inside the viewport's scroll range.
	m.terminalHeight = 8
	m.Records = []core.TranslationRecord{
		{
			ID: "r0", SourceLang: "en", TargetLang: "ja",
			Source:      "Hello world",
			Translation: "こんにちは世界",
		},
		{
			ID: "r1", SourceLang: "en", TargetLang: "ja",
			Source:      strings.Repeat("alpha ", 60),
			Translation: strings.Repeat("beta ", 60),
		},
		{
			ID: "r2", SourceLang: "en", TargetLang: "ja",
			Source:      strings.Repeat("gamma ", 40),
			Translation: strings.Repeat("delta ", 40),
		},
	}
	return m.refreshHistory(false)
}

// requireRecordStarts builds the fixture and verifies the structural
// invariants the navigation tests rely on: three record starts, the first one
// at row 0, strictly increasing, and each one being a non-selectable
// top-border placeholder directly above that record's own line row.
func requireRecordStarts(t *testing.T, m Model) []int {
	t.Helper()
	starts := m.historyRecordStartRows()
	if len(starts) != 3 {
		t.Fatalf("historyRecordStartRows() = %d starts, want 3", len(starts))
	}
	if starts[0] != 0 {
		t.Fatalf("first record start = %d, want 0", starts[0])
	}
	for i, s := range starts {
		if i > 0 && s <= starts[i-1] {
			t.Fatalf("record starts not strictly increasing: %v", starts)
		}
		if s >= len(m.semRows) {
			t.Fatalf("record start %d beyond semRows (%d rows)", s, len(m.semRows))
		}
		row := m.semRows[s]
		if row.LineID != -1 || row.Selectable {
			t.Errorf("record %d start row %d: LineID=%d Selectable=%v, want non-selectable top border",
				i, s, row.LineID, row.Selectable)
		}
		if s+1 >= len(m.semRows) {
			t.Fatalf("record %d has no line row after its top border", i)
		}
		next := m.semRows[s+1]
		if next.LineID < 0 || next.LineID >= len(m.semLines) ||
			m.semLines[next.LineID].recordIndex != i {
			t.Errorf("row after record %d start is not its own line row (LineID=%d)", i, next.LineID)
		}
	}
	return starts
}

// setHistoryOffset positions the viewport and fails if the fixture cannot
// reach the offset, so a later assertion can never pass by coincidence.
func setHistoryOffset(t *testing.T, m *Model, off int) {
	t.Helper()
	m.viewport.SetYOffset(off)
	if m.viewport.YOffset() != off {
		t.Fatalf("fixture cannot reach YOffset %d (got %d)", off, m.viewport.YOffset())
	}
}

// TestHistoryNavigationBetweenRecords pins the exact h/l record transitions
// on three records: boundaries clamp (no-op with an unchanged offset), the
// interior moves by one record and lands on the target's top border row.
func TestHistoryNavigationBetweenRecords(t *testing.T) {
	tests := []struct {
		name string
		from int
		msg  tea.KeyPressMsg
		want int
		stay bool // boundary: offset must not move at all
	}{
		{"h on first record is no-op", 0, tea.KeyPressMsg{Text: "h", Code: 'h'}, 0, true},
		{"l from record 0 to record 1", 0, tea.KeyPressMsg{Text: "l", Code: 'l'}, 1, false},
		{"h from record 1 to record 0", 1, tea.KeyPressMsg{Text: "h", Code: 'h'}, 0, false},
		{"l from record 1 to record 2", 1, tea.KeyPressMsg{Text: "l", Code: 'l'}, 2, false},
		{"h from record 2 to record 1", 2, tea.KeyPressMsg{Text: "h", Code: 'h'}, 1, false},
		{"l on last record is no-op", 2, tea.KeyPressMsg{Text: "l", Code: 'l'}, 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := historyNavTestModel(t)
			starts := requireRecordStarts(t, m)

			// Start one row inside the source record: any row of the record
			// counts as being on that record, not only its top border.
			from := starts[tt.from] + 1
			setHistoryOffset(t, &m, from)
			if got := m.currentHistoryRecordIndex(); got != tt.from {
				t.Fatalf("currentHistoryRecordIndex() = %d at offset %d, want %d", got, from, tt.from)
			}

			m = pressKey(t, m, tt.msg)

			if tt.stay {
				if got := m.viewport.YOffset(); got != from {
					t.Errorf("YOffset = %d, want unchanged %d (boundary no-op)", got, from)
				}
			} else {
				if got := m.viewport.YOffset(); got != starts[tt.want] {
					t.Errorf("YOffset = %d, want record %d top border at row %d", got, tt.want, starts[tt.want])
				}
			}
			if got := m.currentHistoryRecordIndex(); got != tt.want {
				t.Errorf("currentHistoryRecordIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestHistoryNavigationFromInsideRecord covers an offset in the middle of a
// record: both keys treat the whole record span (top border, wrapped body
// rows, bottom border) as one record and jump to the neighbour's top border.
func TestHistoryNavigationFromInsideRecord(t *testing.T) {
	m := historyNavTestModel(t)
	starts := requireRecordStarts(t, m)

	mid := starts[1] + (starts[2]-starts[1])/2
	if mid <= starts[1] || mid >= starts[2] {
		t.Fatalf("fixture must place an offset strictly inside record 1: starts=%v", starts)
	}
	setHistoryOffset(t, &m, mid)
	if got := m.currentHistoryRecordIndex(); got != 1 {
		t.Fatalf("currentHistoryRecordIndex() = %d at mid of record 1, want 1", got)
	}

	// h -> record 0, top border at the viewport top.
	afterH := pressKey(t, m, tea.KeyPressMsg{Text: "h", Code: 'h'})
	if got := afterH.viewport.YOffset(); got != starts[0] {
		t.Errorf("h: YOffset = %d, want record 0 start %d", got, starts[0])
	}
	if got := afterH.currentHistoryRecordIndex(); got != 0 {
		t.Errorf("h: currentHistoryRecordIndex() = %d, want 0", got)
	}
	if row := afterH.semRows[afterH.viewport.YOffset()]; row.LineID != -1 {
		t.Errorf("h: target row LineID = %d, want top-border placeholder (-1)", row.LineID)
	}

	// l -> record 2, top border at the viewport top.
	afterL := pressKey(t, m, tea.KeyPressMsg{Text: "l", Code: 'l'})
	if got := afterL.viewport.YOffset(); got != starts[2] {
		t.Errorf("l: YOffset = %d, want record 2 start %d", got, starts[2])
	}
	if got := afterL.currentHistoryRecordIndex(); got != 2 {
		t.Errorf("l: currentHistoryRecordIndex() = %d, want 2", got)
	}
	if row := afterL.semRows[afterL.viewport.YOffset()]; row.LineID != -1 {
		t.Errorf("l: target row LineID = %d, want top-border placeholder (-1)", row.LineID)
	}
}

// TestHistoryNavigationUsesSemanticRowHeights proves navigation follows the
// actual per-record semantic row counts (records differ in height, one is
// taller than the whole viewport) instead of any fixed record height.
func TestHistoryNavigationUsesSemanticRowHeights(t *testing.T) {
	m := historyNavTestModel(t)
	starts := requireRecordStarts(t, m)

	h0 := starts[1] - starts[0]
	h1 := starts[2] - starts[1]
	h2 := len(m.semRows) - starts[2]
	if h0 == h1 || h1 == h2 || h0 == h2 {
		t.Fatalf("fixture records must have distinct heights, got %d/%d/%d", h0, h1, h2)
	}
	if h1 <= h0 || h1 <= h2 {
		t.Fatalf("record 1 must be the tallest record, got %d vs %d and %d", h1, h0, h2)
	}
	if h1 <= m.viewport.Height() {
		t.Fatalf("record 1 (%d rows) must be taller than the viewport (%d rows) to exercise tall targets",
			h1, m.viewport.Height())
	}

	// From inside the short record 0, l puts the tall record 1's top border
	// at the viewport top — not a fixed-step scroll.
	setHistoryOffset(t, &m, starts[0]+1)
	m = pressKey(t, m, tea.KeyPressMsg{Text: "l", Code: 'l'})
	if got := m.viewport.YOffset(); got != starts[1] {
		t.Errorf("l: YOffset = %d, want tall record's start row %d", got, starts[1])
	}
	if row := m.semRows[m.viewport.YOffset()]; row.LineID != -1 || row.Selectable {
		t.Errorf("l: landed on LineID=%d Selectable=%v, want non-selectable top border", row.LineID, row.Selectable)
	}

	// From inside record 1, h returns to record 0's top border regardless of
	// how many rows record 1 spans.
	setHistoryOffset(t, &m, starts[1]+1)
	m = pressKey(t, m, tea.KeyPressMsg{Text: "h", Code: 'h'})
	if got := m.viewport.YOffset(); got != starts[0] {
		t.Errorf("h: YOffset = %d, want record 0 start %d", got, starts[0])
	}
}

// TestHistoryNavigationEmptyHistory pins h/l as no-ops on an empty history:
// offset, record index and command output are all untouched.
func TestHistoryNavigationEmptyHistory(t *testing.T) {
	m := newTestModel(t)
	if len(m.Records) != 0 {
		t.Fatalf("fixture must start with empty history, got %d records", len(m.Records))
	}
	if got := m.currentHistoryRecordIndex(); got != -1 {
		t.Fatalf("currentHistoryRecordIndex() = %d on empty history, want -1", got)
	}
	if got := len(m.historyRecordStartRows()); got != 0 {
		t.Fatalf("historyRecordStartRows() = %d starts on empty history, want 0", got)
	}

	for _, msg := range []tea.KeyPressMsg{
		{Text: "h", Code: 'h'},
		{Text: "l", Code: 'l'},
	} {
		r, cmd := m.Update(msg)
		next := r.(Model)
		if cmd != nil {
			t.Errorf("%q on empty history returned a command, want nil", msg.String())
		}
		if got := next.viewport.YOffset(); got != 0 {
			t.Errorf("%q on empty history: YOffset = %d, want 0", msg.String(), got)
		}
		if got := next.currentHistoryRecordIndex(); got != -1 {
			t.Errorf("%q on empty history: currentHistoryRecordIndex() = %d, want -1", msg.String(), got)
		}
		m = next
	}
}

// TestHistoryNavigationKeysTypeIntoTextareaInInputMode pins that h/l are not
// hijacked by history navigation while the textarea has focus: they insert
// their characters and the viewport does not move.
func TestHistoryNavigationKeysTypeIntoTextareaInInputMode(t *testing.T) {
	m := historyNavTestModel(t)
	m, _ = m.enterInputMode()
	if !m.inputMode {
		t.Fatal("fixture must be in input mode")
	}
	off := m.viewport.YOffset()

	m = pressKey(t, m, tea.KeyPressMsg{Text: "h", Code: 'h'})
	m = pressKey(t, m, tea.KeyPressMsg{Text: "l", Code: 'l'})

	if !m.inputMode {
		t.Error("h/l must leave input mode active")
	}
	if got, want := m.textArea.Value(), "hl"; got != want {
		t.Errorf("textarea value = %q, want %q (h/l must type, not navigate)", got, want)
	}
	if got := m.viewport.YOffset(); got != off {
		t.Errorf("YOffset = %d, want unchanged %d (no history navigation in input mode)", got, off)
	}
	if got := m.currentHistoryRecordIndex(); got != 0 {
		t.Errorf("currentHistoryRecordIndex() = %d, want 0 (offset untouched)", got)
	}
}
