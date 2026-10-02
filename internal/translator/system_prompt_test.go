package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// The prompt assembled from the default base must stay byte-identical to the
// hard-coded prompt of the pre-configurable versions.
func TestBuildSystemPromptDefaultByteExact(t *testing.T) {
	const defaultBase = "You are a professional translator. Translate the user's text accurately and naturally."

	cases := []struct {
		name string
		req  TranslationRequest
		want string
	}{
		{
			name: "specific source and target",
			req:  TranslationRequest{Text: "Bonjour", SourceLang: "fr", TargetLang: "en"},
			want: defaultBase + " The source language is fr. Translate to en. Return ONLY the translated text, nothing else.",
		},
		{
			name: "auto source omits the source sentence",
			req:  TranslationRequest{Text: "Hello", SourceLang: "auto", TargetLang: "zh"},
			want: defaultBase + " Translate to zh. Return ONLY the translated text, nothing else.",
		},
		{
			name: "empty source omits the source sentence",
			req:  TranslationRequest{Text: "Hello", SourceLang: "", TargetLang: "ja"},
			want: defaultBase + " Translate to ja. Return ONLY the translated text, nothing else.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Empty base prompt = provider fallback = default prompt.
			if got := buildSystemPrompt("", tc.req); got != tc.want {
				t.Errorf("buildSystemPrompt(\"\") =\n%q\nwant\n%q", got, tc.want)
			}
			// The configured default produces the same string.
			if got := buildSystemPrompt(defaultBase, tc.req); got != tc.want {
				t.Errorf("buildSystemPrompt(default) =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// A user-configured base prompt replaces only the base sentence; the
// language sentences and the output constraint keep their exact order,
// spacing and punctuation.
func TestBuildSystemPromptCustomBase(t *testing.T) {
	const customBase = "You are a concise bilingual subtitle translator.\nPreserve technical terms in English."

	got := buildSystemPrompt(customBase, TranslationRequest{
		Text:       "Hello",
		SourceLang: "fr",
		TargetLang: "en",
	})
	want := customBase + " The source language is fr. Translate to en. Return ONLY the translated text, nothing else."
	if got != want {
		t.Errorf("custom base prompt =\n%q\nwant\n%q", got, want)
	}

	// Auto source still omits its sentence with a custom base.
	got = buildSystemPrompt(customBase, TranslationRequest{
		Text:       "Hello",
		SourceLang: "auto",
		TargetLang: "zh",
	})
	want = customBase + " Translate to zh. Return ONLY the translated text, nothing else."
	if got != want {
		t.Errorf("custom base, auto source =\n%q\nwant\n%q", got, want)
	}
	if contains(got, "source language is auto") {
		t.Error("system prompt should not mention auto as source language")
	}
}

// End-to-end through Translate: the configured base prompt reaches the wire
// as the system message, the user message stays the raw text.
func TestTranslate_SystemPrompt_ConfiguredBase(t *testing.T) {
	const customBase = "You are a concise bilingual subtitle translator.\nPreserve technical terms in English."

	var receivedReq openaiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Role: "assistant", Content: "translated"}},
			},
		})
	}))
	defer server.Close()

	os.Setenv("CUSTOM_PROMPT_TEST_KEY", "test-key")
	defer os.Unsetenv("CUSTOM_PROMPT_TEST_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:      server.URL,
		Model:        "gpt-4o-mini",
		APIKeyEnv:    "CUSTOM_PROMPT_TEST_KEY",
		TimeoutSec:   5,
		SystemPrompt: customBase,
	})

	if _, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		SourceLang: "fr",
		TargetLang: "en",
	}); err != nil {
		t.Fatalf("Translate: %v", err)
	}

	if len(receivedReq.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(receivedReq.Messages))
	}
	wantSystem := customBase + " The source language is fr. Translate to en. Return ONLY the translated text, nothing else."
	if receivedReq.Messages[0].Content != wantSystem {
		t.Errorf("system message =\n%q\nwant\n%q", receivedReq.Messages[0].Content, wantSystem)
	}
	if receivedReq.Messages[1].Role != "user" || receivedReq.Messages[1].Content != "Hello" {
		t.Errorf("user message = {%q %q}, want {\"user\" \"Hello\"}", receivedReq.Messages[1].Role, receivedReq.Messages[1].Content)
	}
}
