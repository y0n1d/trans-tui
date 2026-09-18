package core

import (
	"context"
	"testing"

	"my-trans/internal/translator"
)

// captureTranslator records the request it received so tests can assert what
// the provider is actually asked to translate.
type captureTranslator struct {
	got    translator.TranslationRequest
	result translator.TranslationResult
	err    error
}

func (c *captureTranslator) Translate(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	c.got = req
	return c.result, c.err
}

func TestServiceResolvesAutoTargetForProvider(t *testing.T) {
	cases := []struct {
		name         string
		text         string
		configured   string
		wantTarget   string
		wantProvider string
	}{
		{"chinese to english", "你好世界", LangAuto, LangEnglish, "siliconflow"},
		{"english to chinese", "Hello world", LangAuto, LangChinese, "openai-compatible"},
		{"japanese to chinese", "こんにちは", LangAuto, LangChinese, "openai-compatible"},
		{"korean to chinese", "안녕하세요", LangAuto, LangChinese, "openai-compatible"},
		{"chinese punctuation to english", "你好，世界！", LangAuto, LangEnglish, "openai-compatible"},
		{"mixed dominant chinese to english", "这个 API 很好用", LangAuto, LangEnglish, "openai-compatible"},
		{"mixed dominant english to chinese", "Hello 世界", LangAuto, LangChinese, "openai-compatible"},
		{"explicit target preserved", "你好世界", "fr", "fr", "openai-compatible"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cap := &captureTranslator{
				result: translator.TranslationResult{
					Translation: "translated",
					Provider:    tc.wantProvider,
					Model:       "test-model",
				},
			}
			svc := NewService(cap)

			result, err := svc.Translate(context.Background(), translator.TranslationRequest{
				Text:       tc.text,
				SourceLang: LangAuto,
				TargetLang: tc.configured,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cap.got.TargetLang != tc.wantTarget {
				t.Errorf("provider received TargetLang = %q, want %q", cap.got.TargetLang, tc.wantTarget)
			}
			if cap.got.SourceLang != LangAuto {
				t.Errorf("provider received SourceLang = %q, want %q", cap.got.SourceLang, LangAuto)
			}
			if cap.got.Text != tc.text {
				t.Errorf("provider received Text = %q, want %q", cap.got.Text, tc.text)
			}
			if result.TargetLang != tc.wantTarget {
				t.Errorf("result.TargetLang = %q, want %q", result.TargetLang, tc.wantTarget)
			}
			if result.Provider != tc.wantProvider {
				t.Errorf("result.Provider = %q, want %q", result.Provider, tc.wantProvider)
			}
			if result.Model != "test-model" {
				t.Errorf("result.Model = %q, want %q", result.Model, "test-model")
			}
		})
	}
}

func TestServiceAutoTargetPerRequest(t *testing.T) {
	cap := &captureTranslator{
		result: translator.TranslationResult{Translation: "x", Provider: "mock"},
	}
	svc := NewService(cap)

	// Two consecutive calls through the same service must resolve their own
	// target from their own text, so a single running server handles both
	// directions.
	if _, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text: "你好世界", SourceLang: LangAuto, TargetLang: LangAuto,
	}); err != nil {
		t.Fatal(err)
	}
	if cap.got.TargetLang != LangEnglish {
		t.Errorf("first call target = %q, want %q", cap.got.TargetLang, LangEnglish)
	}

	if _, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text: "Hello world", SourceLang: LangAuto, TargetLang: LangAuto,
	}); err != nil {
		t.Fatal(err)
	}
	if cap.got.TargetLang != LangChinese {
		t.Errorf("second call target = %q, want %q", cap.got.TargetLang, LangChinese)
	}
}

func TestServiceAutoTargetNotAppliedOnError(t *testing.T) {
	cap := &captureTranslator{
		result: translator.TranslationResult{},
		err:    context.DeadlineExceeded,
	}
	svc := NewService(cap)

	_, err := svc.Translate(context.Background(), translator.TranslationRequest{
		Text: "你好", SourceLang: LangAuto, TargetLang: LangAuto,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if cap.got.TargetLang != LangEnglish {
		t.Errorf("provider should still receive resolved target, got %q", cap.got.TargetLang)
	}
}
