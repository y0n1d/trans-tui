package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
