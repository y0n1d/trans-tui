package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/y0n1d/trans-tui/internal/core"
)

// TestTopBorderContainsLanguageLabel verifies that the language label is embedded
// in the top border line of the card, not as a separate content line.
func TestTopBorderContainsLanguageLabel(t *testing.T) {
	model := Model{viewport: viewportForTest(80, 20)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好",
	})
	model.buildSemanticMap(model.Records, 80)

	card := model.renderRecordHighlighted(model.Records[0], 80, 0)
	lines := strings.Split(card, "\n")
	if len(lines) == 0 {
		t.Fatal("card has no lines")
	}

	topLine := lines[0]
	if !strings.Contains(topLine, "[auto]") {
		t.Errorf("top border missing [auto], got: %q", topLine)
	}
	if !strings.Contains(topLine, "→") {
		t.Errorf("top border missing arrow, got: %q", topLine)
	}
	if !strings.Contains(topLine, "[zh]") {
		t.Errorf("top border missing [zh], got: %q", topLine)
	}
}

// TestTopBorderDifferentLanguages verifies language labels are dynamic, not hardcoded.
func TestTopBorderDifferentLanguages(t *testing.T) {
	model := Model{viewport: viewportForTest(80, 20)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
	})
	model.buildSemanticMap(model.Records, 80)

	card := model.renderRecordHighlighted(model.Records[0], 80, 0)
	lines := strings.Split(card, "\n")
	topLine := lines[0]

	if !strings.Contains(topLine, "[en]") {
		t.Errorf("top border missing [en], got: %q", topLine)
	}
	if !strings.Contains(topLine, "[ja]") {
		t.Errorf("top border missing [ja], got: %q", topLine)
	}
	if strings.Contains(topLine, "[auto]") {
		t.Errorf("top border should not contain [auto], got: %q", topLine)
	}
}

// TestLanguageLabelNotInBody verifies that the source/translation body lines
// no longer contain the [lang] prefix.
func TestLanguageLabelNotInBody(t *testing.T) {
	model := Model{viewport: viewportForTest(80, 20)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
	})
	model.buildSemanticMap(model.Records, 80)

	card := model.renderRecordHighlighted(model.Records[0], 80, 0)
	lines := strings.Split(card, "\n")
	// Body lines (between top and bottom border) should not contain [en] or [ja] as prefixes.
	for i := 1; i < len(lines)-1; i++ {
		line := lines[i]
		trimmed := strings.TrimLeft(line, " │")
		if strings.HasPrefix(trimmed, "[en]") || strings.HasPrefix(trimmed, "[ja]") {
			t.Errorf("body line %d contains language prefix: %q", i, line)
		}
	}
}

// TestTopBorderWidthInvariant verifies the card width is correct for different viewport widths.
func TestTopBorderWidthInvariant(t *testing.T) {
	widths := []int{20, 30, 40, 60, 80, 120}
	for _, w := range widths {
		t.Run(func() string { return string(rune('0'+w/10)) + string(rune('0'+w%10)) }(), func(t *testing.T) {
			model := Model{viewport: viewportForTest(w, 20)}
			model.Records = append(model.Records, core.TranslationRecord{
				SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好",
			})
			model.buildSemanticMap(model.Records, w)

			card := model.renderRecordHighlighted(model.Records[0], w, 0)
			cardWidth := lipgloss.Width(card)

			if cardWidth != w {
				t.Errorf("viewport=%d card=%d (expected %d)", w, cardWidth, w)
			}
		})
	}
}

// TestTopBorderWidthInvariantWithOtherLanguages verifies width with different lang labels.
func TestTopBorderWidthInvariantWithOtherLanguages(t *testing.T) {
	model := Model{viewport: viewportForTest(60, 20)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界",
	})
	model.buildSemanticMap(model.Records, 60)

	card := model.renderRecordHighlighted(model.Records[0], 60, 0)
	cardWidth := lipgloss.Width(card)

	if cardWidth != 60 {
		t.Errorf("viewport=60 card=%d (expected 60)", cardWidth)
	}

	topLine := strings.Split(card, "\n")[0]
	if !strings.Contains(topLine, "[en]") || !strings.Contains(topLine, "[ja]") {
		t.Errorf("top border should contain [en] → [ja], got: %q", topLine)
	}
}

// TestNarrowWidthNoPanic ensures very narrow widths do not cause panics
// or negative repeat counts.
func TestNarrowWidthNoPanic(t *testing.T) {
	widths := []int{3, 4, 5, 6, 8, 10}
	for _, w := range widths {
		t.Run(func() string { return string(rune('0'+w)) }(), func(t *testing.T) {
			model := Model{viewport: viewportForTest(w, 20)}
			model.Records = append(model.Records, core.TranslationRecord{
				SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好",
			})

			// Should not panic
			model.buildSemanticMap(model.Records, w)
			_ = model.renderRecordHighlighted(model.Records[0], w, 0)
		})
	}
}

// TestResizeRecalculatesTopBorder verifies the top border label is correct
// after a viewport resize.
func TestResizeRecalculatesTopBorder(t *testing.T) {
	model := Model{viewport: viewportForTest(80, 20)}
	model.Records = append(model.Records, core.TranslationRecord{
		SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界",
	})
	model.buildSemanticMap(model.Records, 80)

	// Resize to narrower width
	model.viewport.Width = 40
	model.buildSemanticMap(model.Records, 40)

	card := model.renderRecordHighlighted(model.Records[0], 40, 0)
	cardWidth := lipgloss.Width(card)
	if cardWidth != 40 {
		t.Errorf("after resize: viewport=40 card=%d", cardWidth)
	}

	topLine := strings.Split(card, "\n")[0]
	if !strings.Contains(topLine, "[en]") || !strings.Contains(topLine, "[ja]") {
		t.Errorf("after resize: top border missing labels, got: %q", topLine)
	}

	// Resize back to wider
	model.viewport.Width = 80
	model.buildSemanticMap(model.Records, 80)

	card = model.renderRecordHighlighted(model.Records[0], 80, 0)
	cardWidth = lipgloss.Width(card)
	if cardWidth != 80 {
		t.Errorf("after resize back: viewport=80 card=%d", cardWidth)
	}
}

// TestSemanticRowsAlignWithRenderedAfterBorderLabelChange verifies that semantic
// rows still match rendered rows after the border label change.
func TestSemanticRowsAlignWithRenderedAfterBorderLabelChange(t *testing.T) {
	cases := []struct {
		name   string
		record core.TranslationRecord
		width  int
	}{
		{
			"auto to zh",
			core.TranslationRecord{SourceLang: "auto", Source: "Hello World", TargetLang: "zh", Translation: "你好世界"},
			80,
		},
		{
			"en to ja narrow",
			core.TranslationRecord{SourceLang: "en", Source: "Long source text that should wrap", TargetLang: "ja", Translation: "長いテキスト"},
			40,
		},
		{
			"with provider",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは", Provider: "deepseek", Model: "deepseek-flash"},
			60,
		},
		{
			"error record",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", Error: "network timeout"},
			60,
		},
		{
			"very narrow",
			core.TranslationRecord{SourceLang: "en", Source: "Hi", TargetLang: "ja", Translation: "你好"},
			20,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{viewport: viewportForTest(tc.width, 40)}
			model.Records = append(model.Records, tc.record)
			model.buildSemanticMap(model.Records, tc.width)

			rendered := model.renderRecordHighlighted(tc.record, tc.width, 0)
			renderedRows := lipgloss.Height(rendered)

			if renderedRows != len(model.semRows) {
				t.Fatalf("rendered rows (%d) != semRows (%d)", renderedRows, len(model.semRows))
			}
			if model.semRows[0].Selectable {
				t.Error("top border row must not be selectable")
			}
			if model.semRows[len(model.semRows)-1].Selectable {
				t.Error("bottom border row must not be selectable")
			}
		})
	}
}

// TestTopBottomBorderWidthMatch is a direct regression test for the bug where
// the top border with the language label was shorter than the bottom border.
// It splits the rendered card into physical lines and verifies that the top
// border and bottom border have identical display widths.
func TestTopBottomBorderWidthMatch(t *testing.T) {
	cases := []struct {
		name   string
		record core.TranslationRecord
		width  int
	}{
		{
			"auto to zh, w=20",
			core.TranslationRecord{SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好"},
			20,
		},
		{
			"auto to zh, w=40",
			core.TranslationRecord{SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好"},
			40,
		},
		{
			"auto to zh, w=60",
			core.TranslationRecord{SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好"},
			60,
		},
		{
			"auto to zh, w=80",
			core.TranslationRecord{SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好"},
			80,
		},
		{
			"en to ja, w=40",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			40,
		},
		{
			"en to ja, w=80",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			80,
		},
		{
			"w=120 wide",
			core.TranslationRecord{SourceLang: "auto", Source: "Hello", TargetLang: "zh", Translation: "你好"},
			120,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{viewport: viewportForTest(tc.width, 40)}
			model.Records = append(model.Records, tc.record)
			model.buildSemanticMap(model.Records, tc.width)

			card := model.renderRecordHighlighted(tc.record, tc.width, 0)
			lines := strings.Split(card, "\n")

			var topLine, bottomLine string
			for _, l := range lines {
				if strings.ContainsRune(l, '╭') || strings.ContainsRune(l, '┌') {
					topLine = l
				}
				if strings.ContainsRune(l, '╰') || strings.ContainsRune(l, '└') {
					bottomLine = l
				}
			}

			if topLine == "" {
				t.Fatal("no top border line found in rendered card")
			}
			if bottomLine == "" {
				t.Fatal("no bottom border line found in rendered card")
			}

			topW := xansi.StringWidth(topLine)
			botW := xansi.StringWidth(bottomLine)

			if topW != botW {
				t.Errorf("top border width %d != bottom border width %d\n  top:    %q\n  bottom: %q",
					topW, botW, topLine, bottomLine)
			}

			// Also verify the label is present in the top border.
			if !strings.Contains(topLine, tc.record.SourceLang) {
				t.Errorf("top border missing source lang %q: %q", tc.record.SourceLang, topLine)
			}
			if !strings.Contains(topLine, tc.record.TargetLang) {
				t.Errorf("top border missing target lang %q: %q", tc.record.TargetLang, topLine)
			}

			// Verify all content lines have the same width as the borders.
			for i, l := range lines {
				lw := xansi.StringWidth(l)
				if lw != botW {
					t.Errorf("line %d width %d != border width %d: %q", i, lw, botW, l)
				}
			}
		})
	}
}

// TestBottomBorderModelLabel verifies that the model name appears in the
// bottom border right-aligned, and only the last path segment is shown.
func TestBottomBorderModelLabel(t *testing.T) {
	cases := []struct {
		name       string
		provider   string
		model      string
		wantLabel  string
		dontWant   string
	}{
		{
			"long path",
			"openai-compatible", "Qwen/Qwen2.5-7B-Instruct",
			"Qwen2.5-7B-Instruct", "openai-compatible",
		},
		{
			"short path",
			"openai-compatible", "deepseek-chat",
			"deepseek-chat", "openai-compatible",
		},
		{
			"google path",
			"google", "gemini-2.5-flash",
			"gemini-2.5-flash", "google",
		},
		{
			"single segment",
			"mock", "test-model",
			"test-model", "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{viewport: viewportForTest(60, 20)}
			record := core.TranslationRecord{
				SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
				Provider: tc.provider, Model: tc.model,
			}
			model.Records = append(model.Records, record)
			model.buildSemanticMap(model.Records, 60)

			card := model.renderRecordHighlighted(record, 60, 0)
			lines := strings.Split(card, "\n")

			var bottomLine string
			for _, l := range lines {
				if strings.ContainsRune(l, '╰') || strings.ContainsRune(l, '└') {
					bottomLine = l
				}
			}

			if bottomLine == "" {
				t.Fatal("no bottom border line found")
			}

			if !strings.Contains(bottomLine, tc.wantLabel) {
				t.Errorf("bottom border missing %q: %q", tc.wantLabel, bottomLine)
			}
			if tc.dontWant != "" && strings.Contains(bottomLine, tc.dontWant) {
				t.Errorf("bottom border should not contain %q: %q", tc.dontWant, bottomLine)
			}

			// Model label should NOT appear in body lines.
			for i := 1; i < len(lines)-1; i++ {
				if strings.Contains(lines[i], tc.wantLabel) {
					t.Errorf("body line %d contains model label %q: %q", i, tc.wantLabel, lines[i])
				}
			}
		})
	}
}

// TestModelDisplayName extracts just the model name logic.
func TestModelDisplayName(t *testing.T) {
	cases := []struct {
		provider, model, want string
	}{
		{"openai-compatible", "Qwen/Qwen2.5-7B-Instruct", "Qwen2.5-7B-Instruct"},
		{"openai-compatible", "deepseek-chat", "deepseek-chat"},
		{"google", "gemini-2.5-flash", "gemini-2.5-flash"},
		{"mock", "test-model", "test-model"},
		{"", "", ""},
		{"google", "", ""},
	}
	for _, tc := range cases {
		got := modelDisplayName(tc.provider, tc.model)
		if got != tc.want {
			t.Errorf("modelDisplayName(%q, %q) = %q, want %q", tc.provider, tc.model, got, tc.want)
		}
	}
}

// TestBottomBorderModelLabelNarrowWidth verifies no panic and correct width
// at very narrow widths where the model label may not fit.
func TestBottomBorderModelLabelNarrowWidth(t *testing.T) {
	widths := []int{3, 4, 5, 6, 8, 10, 20, 40, 60, 80, 120}
	for _, w := range widths {
		t.Run(fmt.Sprintf("w=%d", w), func(t *testing.T) {
			model := Model{viewport: viewportForTest(w, 20)}
			record := core.TranslationRecord{
				SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
				Provider: "openai-compatible", Model: "Qwen/Qwen2.5-7B-Instruct",
			}
			model.Records = append(model.Records, record)
			model.buildSemanticMap(model.Records, w)

			card := model.renderRecordHighlighted(record, w, 0)
			cardWidth := lipgloss.Width(card)
			if cardWidth < w {
				t.Errorf("card width %d < viewport %d", cardWidth, w)
			}

			lines := strings.Split(card, "\n")
			var topLine, bottomLine string
			for _, l := range lines {
				if strings.ContainsRune(l, '╭') || strings.ContainsRune(l, '┌') {
					topLine = l
				}
				if strings.ContainsRune(l, '╰') || strings.ContainsRune(l, '└') {
					bottomLine = l
				}
			}

			topW := xansi.StringWidth(topLine)
			botW := xansi.StringWidth(bottomLine)
			if topW != botW {
				t.Errorf("top=%d bottom=%d", topW, botW)
			}
		})
	}
}

// TestBottomBorderModelLabelRightAligned verifies the model label is at the
// right side of the bottom border by checking the border ends with the label.
func TestBottomBorderModelLabelRightAligned(t *testing.T) {
	model := Model{viewport: viewportForTest(60, 20)}
	record := core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
		Provider: "openai-compatible", Model: "deepseek-chat",
	}
	model.Records = append(model.Records, record)
	model.buildSemanticMap(model.Records, 60)

	card := model.renderRecordHighlighted(record, 60, 0)
	lines := strings.Split(card, "\n")

	var bottomLine string
	for _, l := range lines {
		if strings.ContainsRune(l, '╰') || strings.ContainsRune(l, '└') {
			bottomLine = l
		}
	}

	// Bottom border should end with: label + right corner char.
	if !strings.HasSuffix(bottomLine, "deepseek-chat") {
		// The label may be followed by a border corner char; check it's near the end.
		idx := strings.Index(bottomLine, "deepseek-chat")
		if idx < 0 {
			t.Fatalf("model label not found in bottom border: %q", bottomLine)
		}
		// Verify label is in the right half.
		rest := bottomLine[idx+len("deepseek-chat"):]
		if len(rest) > 3 {
			t.Errorf("model label too far from right edge, trailing: %q in %q", rest, bottomLine)
		}
	}
}

// TestNoProviderModelShowsPlainBottomBorder verifies a record without provider/model
// still renders a correct plain bottom border.
func TestNoProviderModelShowsPlainBottomBorder(t *testing.T) {
	model := Model{viewport: viewportForTest(60, 20)}
	record := core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
	}
	model.Records = append(model.Records, record)
	model.buildSemanticMap(model.Records, 60)

	card := model.renderRecordHighlighted(record, 60, 0)
	lines := strings.Split(card, "\n")

	var bottomLine string
	for _, l := range lines {
		if strings.ContainsRune(l, '╰') || strings.ContainsRune(l, '└') {
			bottomLine = l
		}
	}

	botW := xansi.StringWidth(bottomLine)
	if botW != 60 {
		t.Errorf("plain bottom border width %d != 60", botW)
	}
}

// TestSemanticSelectionExcludesModelLabel confirms the model label does not
// appear in any selectable semantic row.
func TestSemanticSelectionExcludesModelLabel(t *testing.T) {
	model := Model{viewport: viewportForTest(80, 20)}
	record := core.TranslationRecord{
		SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは",
		Provider: "openai-compatible", Model: "Qwen/Qwen2.5-7B-Instruct",
	}
	model.Records = append(model.Records, record)
	model.buildSemanticMap(model.Records, 80)

	for _, row := range model.semRows {
		if !row.Selectable {
			continue
		}
		line := model.semLines[row.LineID]
		text := string(line.text)
		if strings.Contains(text, "Qwen2.5-7B-Instruct") {
			t.Errorf("selectable row contains model label: %q", text)
		}
		if strings.Contains(text, "via ") {
			t.Errorf("selectable row contains 'via' prefix: %q", text)
		}
	}
}
