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
	"github.com/y0n1d/trans-tui/internal/translator"

	tea "github.com/charmbracelet/bubbletea"
)

type stubTranslator struct {
	got    translator.TranslationRequest
	result translator.TranslationResult
	err    error
}

func (s *stubTranslator) Translate(ctx context.Context, req translator.TranslationRequest) (translator.TranslationResult, error) {
	s.got = req
	return s.result, s.err
}

func TestIPCHandlerResolvesAutoTargetPerRequest(t *testing.T) {
	stub := &stubTranslator{
		result: translator.TranslationResult{
			Translation: "translated",
			Provider:    "openai-compatible",
			Model:       "deepseek-flash",
		},
	}
	svc := core.NewService(stub)
	ch := make(chan tea.Msg, 4)
	handler := newIPCHandler(config.DefaultConfig(), svc, ch)

	cases := []struct {
		requestID  string
		text       string
		wantTarget string
	}{
		{"req-cn", "你好世界", core.LangEnglish},
		{"req-en", "Hello world", core.LangChinese},
		{"req-ja", "こんにちは", core.LangChinese},
	}

	for _, tc := range cases {
		t.Run(tc.requestID, func(t *testing.T) {
			// TargetLang "auto" is what runClient sends from a config with
			// target_lang = "auto"; the server must resolve it per request.
			resp := handler(context.Background(), ipc.Request{
				Version:    ipc.ProtocolVersion,
				Type:       "translate",
				RequestID:  tc.requestID,
				Text:       tc.text,
				SourceLang: core.LangAuto,
				TargetLang: core.LangAuto,
			})
			if !resp.OK {
				t.Fatalf("response not OK: %s", resp.Error)
			}
			if resp.Translation != "translated" {
				t.Errorf("response translation = %q, want %q", resp.Translation, "translated")
			}
			if stub.got.TargetLang != tc.wantTarget {
				t.Errorf("provider received target = %q, want %q", stub.got.TargetLang, tc.wantTarget)
			}

			msg, ok := (<-ch).(core.TranslationResultMsg)
			if !ok {
				t.Fatal("handler did not emit a TranslationResultMsg")
			}
			if msg.Source != tc.text {
				t.Errorf("msg source = %q, want %q", msg.Source, tc.text)
			}
			if msg.TargetLang != tc.wantTarget {
				t.Errorf("msg target = %q, want %q", msg.TargetLang, tc.wantTarget)
			}
			if msg.Translation != "translated" {
				t.Errorf("msg translation = %q, want %q", msg.Translation, "translated")
			}
		})
	}
}

func TestIPCHandlerExplicitTargetNotOverridden(t *testing.T) {
	stub := &stubTranslator{
		result: translator.TranslationResult{Translation: "t", Provider: "p", Model: "m"},
	}
	svc := core.NewService(stub)
	ch := make(chan tea.Msg, 1)
	handler := newIPCHandler(config.DefaultConfig(), svc, ch)

	resp := handler(context.Background(), ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       "translate",
		RequestID:  "req-explicit",
		Text:       "你好世界",
		SourceLang: "auto",
		TargetLang: "ja",
	})
	if !resp.OK {
		t.Fatalf("response not OK: %s", resp.Error)
	}
	if stub.got.TargetLang != "ja" {
		t.Errorf("provider target = %q, want %q", stub.got.TargetLang, "ja")
	}
}

func TestEndToEndAutoTargetReachesProviderPrompt(t *testing.T) {
	var received struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"translated"}}]}`)
	}))
	defer server.Close()

	os.Setenv("E2E_AUTO_KEY", "test-key")
	defer os.Unsetenv("E2E_AUTO_KEY")

	cfg := config.DefaultConfig()
	cfg.Provider.APIKeyEnv = "E2E_AUTO_KEY"
	cfg.Provider.OpenAI.BaseURL = server.URL

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("newProvider: %v", err)
	}
	svc := core.NewService(prov)
	ch := make(chan tea.Msg, 4)
	handler := newIPCHandler(cfg, svc, ch)

	cases := []struct {
		text       string
		wantTarget string
	}{
		{"你好世界", "Translate to en"},
		{"Hello world", "Translate to zh"},
		{"こんにちは", "Translate to zh"},
	}

	for _, tc := range cases {
		received.Messages = nil
		resp := handler(context.Background(), ipc.Request{
			Version:    ipc.ProtocolVersion,
			Type:       "translate",
			RequestID:  "e2e",
			Text:       tc.text,
			SourceLang: "auto",
			TargetLang: "auto",
		})
		if !resp.OK {
			t.Fatalf("response not OK for %q: %s", tc.text, resp.Error)
		}
		if len(received.Messages) == 0 {
			t.Fatalf("provider received no messages for %q", tc.text)
		}
		systemMsg := received.Messages[0].Content
		if !strings.Contains(systemMsg, tc.wantTarget) {
			t.Errorf("text %q: system prompt = %q, want it to contain %q", tc.text, systemMsg, tc.wantTarget)
		}
	}
}
