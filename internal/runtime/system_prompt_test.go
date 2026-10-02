package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
	"github.com/y0n1d/trans-tui/internal/ipc"

	tea "charm.land/bubbletea/v2"
)

// captureSystemPrompt runs one translate request through the production path
// (newProvider -> core.Service -> newIPCHandler) against an httptest backend
// and returns the system message the provider actually sent.
func captureSystemPrompt(t *testing.T, cfg config.Config, text, sourceLang, targetLang string) string {
	t.Helper()

	var systemMsg string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		if len(body.Messages) > 0 {
			systemMsg = body.Messages[0].Content
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"translated"}}]}`)
	}))
	defer server.Close()

	cfg.Provider.OpenAI.BaseURL = server.URL

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("newProvider: %v", err)
	}
	svc := core.NewService(prov)
	ch := make(chan tea.Msg, 4)
	handler := newIPCHandler(cfg, svc, ch)

	resp := handler(context.Background(), ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       ipc.TypeTranslate,
		RequestID:  "prompt-wiring",
		Text:       text,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	})
	if !resp.OK {
		t.Fatalf("translate failed: %s", resp.Error)
	}
	return systemMsg
}

// A custom translation.system_prompt reaches the provider through the whole
// server path, with the language sentences and output constraint appended.
func TestWiringCustomSystemPromptReachesProvider(t *testing.T) {
	const customBase = "You are a concise bilingual subtitle translator. Preserve technical terms in English."

	os.Setenv("SYSTEM_PROMPT_WIRING_KEY", "test-key")
	defer os.Unsetenv("SYSTEM_PROMPT_WIRING_KEY")

	cfg := config.DefaultConfig()
	cfg.Provider.APIKeyEnv = "SYSTEM_PROMPT_WIRING_KEY"
	cfg.Translation.SystemPrompt = customBase

	got := captureSystemPrompt(t, cfg, "Hello", "fr", "en")

	want := customBase + " The source language is fr. Translate to en. Return ONLY the translated text, nothing else."
	if got != want {
		t.Errorf("system prompt =\n%q\nwant\n%q", got, want)
	}
}

// The default config sends the default prompt byte-for-byte — pinning
// config.DefaultSystemPrompt, the translator fallback and the wiring against
// drift between the two packages.
func TestWiringDefaultSystemPromptByteExact(t *testing.T) {
	os.Setenv("SYSTEM_PROMPT_DEFAULT_KEY", "test-key")
	defer os.Unsetenv("SYSTEM_PROMPT_DEFAULT_KEY")

	cfg := config.DefaultConfig()
	cfg.Provider.APIKeyEnv = "SYSTEM_PROMPT_DEFAULT_KEY"

	// auto source: no source sentence; target resolved by core (Chinese
	// text would resolve to en, "Hello" resolves to zh).
	got := captureSystemPrompt(t, cfg, "Hello world", "auto", "auto")

	want := config.DefaultSystemPrompt + " Translate to zh. Return ONLY the translated text, nothing else."
	if got != want {
		t.Errorf("default system prompt =\n%q\nwant\n%q", got, want)
	}
	if !strings.HasPrefix(got, "You are a professional translator.") {
		t.Errorf("unexpected default prompt start: %q", got)
	}
}
