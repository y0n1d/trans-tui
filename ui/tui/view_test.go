package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"my-trans/internal/core"
)

func viewportForTest(w, h int) viewport.Model {
	return viewport.New(w, h)
}

func TestRecordCardWidthInvariant(t *testing.T) {
	testCases := []struct {
		name          string
		record        core.TranslationRecord
		viewportWidth int
	}{
		{
			"80 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			80,
		},
		{
			"60 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			60,
		},
		{
			"40 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hello World", TargetLang: "ja", Translation: "こんにちは世界"},
			40,
		},
		{
			"25 English",
			core.TranslationRecord{SourceLang: "en", Source: "Hi", TargetLang: "ja", Translation: "你好"},
			25,
		},
		{
			"80 long CJK source",
			core.TranslationRecord{SourceLang: "zh", Source: "这是一个很长的翻译测试文本，需要确保在正确的宽度内换行不会溢出视口边界", TargetLang: "en", Translation: "This is a long translation test text that must wrap correctly within the viewport"},
			80,
		},
		{
			"40 long CJK source",
			core.TranslationRecord{SourceLang: "zh", Source: "这是一个很长的翻译测试文本需要换行", TargetLang: "en", Translation: "Long translation text"},
			40,
		},
		{
			"with provider",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは", Provider: "google", Model: "nmt"},
			60,
		},
		{
			"very narrow 25",
			core.TranslationRecord{SourceLang: "en", Source: "Hello", TargetLang: "ja", Translation: "こんにちは世界"},
			25,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{
				viewport: viewportForTest(tc.viewportWidth, 20),
			}
			model.Records = append(model.Records, tc.record)
			model.buildSemanticMap(model.Records, tc.viewportWidth)

			card := model.renderRecordHighlighted(tc.record, tc.viewportWidth)
			cardWidth := lipgloss.Width(card)

			if cardWidth != tc.viewportWidth {
				t.Errorf("viewport=%d card=%d (expected %d)", tc.viewportWidth, cardWidth, tc.viewportWidth)
			}
		})
	}
}
