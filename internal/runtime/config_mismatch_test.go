package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/core"
	"github.com/y0n1d/trans-tui/internal/ipc"
	"github.com/y0n1d/trans-tui/internal/translator"

	tea "charm.land/bubbletea/v2"
)

// testProviderCfg builds a config whose fingerprint is distinct per
// provider settings, mirroring the DeepSeek-vs-OpenAI scenarios this file
// has always covered.
func testProviderCfg(apiKeyEnv, baseURL, model string) config.Config {
	return config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: apiKeyEnv,
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: baseURL,
				Model:   model,
			},
		},
		Translation: config.TranslationConfig{SourceLang: "auto", TargetLang: "zh"},
	}
}

// startProductionIPCServer serves cfg through the production IPC path: the
// runtime's own newIPCHandler behind a real ipc.Server on a temp socket.
// Listen binds synchronously, so the socket is ready when this returns and
// no sleep is needed. The returned channel receives every tea message the
// handler would forward to the TUI.
func startProductionIPCServer(t *testing.T, cfg config.Config) (string, chan tea.Msg) {
	t.Helper()
	socketPath := tempSocketPath(t)

	svc := core.NewService(&stubTranslator{
		result: translator.TranslationResult{
			Translation: "translated",
			Provider:    "openai-compatible",
			Model:       "test-model",
		},
	})
	ipcCh := make(chan tea.Msg, 10)

	ctx, cancel := context.WithCancel(context.Background())
	srv := ipc.NewServer(socketPath, newIPCHandler(cfg, svc, ipcCh))
	if err := srv.Listen(ctx); err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ctx) }()
	t.Cleanup(cancel)

	return socketPath, ipcCh
}

// TestClientConfigMismatchThroughProductionPath is the P2-1 seam test: a real
// server started with production handler code advertises fingerprint A, the
// production client path (the code runClient delegates to) runs with
// fingerprint B, and must reject it with the exact user-facing message — no
// hand-rolled handlers, no requests or fingerprint comparisons built by the
// test itself.
func TestClientConfigMismatchThroughProductionPath(t *testing.T) {
	cases := []struct {
		name   string
		server config.Config
		client config.Config
	}{
		{
			name:   "openai server vs deepseek client",
			server: testProviderCfg("OPENAI_API_KEY", "https://api.openai.com/v1", "gpt-4o-mini"),
			client: testProviderCfg("DEEPSEEK_API_KEY", "https://api.deepseek.com", "deepseek-flash"),
		},
		{
			name:   "default server vs deepseek client",
			server: config.DefaultConfig(),
			client: testProviderCfg("DEEPSEEK_API_KEY", "https://api.deepseek.com", "deepseek-flash"),
		},
	}

	const wantErr = "existing server uses a different configuration.\nStop the running server or use matching configuration."

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.server.Fingerprint() == tc.client.Fingerprint() {
				t.Fatal("test config bug: server and client fingerprints must differ")
			}
			socketPath, _ := startProductionIPCServer(t, tc.server)

			var stdout bytes.Buffer
			err := clientExchange(&stdout, socketPath, "Hello world", false, false, tc.client)
			if err == nil {
				t.Fatal("clientExchange must reject a server with a different config fingerprint")
			}
			if err.Error() != wantErr {
				t.Errorf("error = %q, want %q", err.Error(), wantErr)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty on failure", stdout.String())
			}
		})
	}
}

// TestClientTranslateThroughProductionPath covers the matching-config
// scenario: the production client passes the fingerprint check against the
// production status handler and the translate reply reaches stdout.
func TestClientTranslateThroughProductionPath(t *testing.T) {
	cfg := testProviderCfg("DEEPSEEK_API_KEY", "https://api.deepseek.com", "deepseek-flash")
	socketPath, ipcCh := startProductionIPCServer(t, cfg)

	var stdout bytes.Buffer
	if err := clientExchange(&stdout, socketPath, "Hello world", false, false, cfg); err != nil {
		t.Fatalf("clientExchange: %v", err)
	}
	if stdout.String() != "translated\n" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "translated\n")
	}

	// The handler posts the result before writing the response, so it is
	// already buffered once clientExchange returned.
	select {
	case msg := <-ipcCh:
		res, ok := msg.(core.TranslationResultMsg)
		if !ok {
			t.Fatalf("handler message = %T, want core.TranslationResultMsg", msg)
		}
		if res.Source != "Hello world" {
			t.Errorf("result source = %q, want %q", res.Source, "Hello world")
		}
	default:
		t.Fatal("production handler did not emit a TranslationResultMsg")
	}
}

// TestClientInputInitialThroughProductionPath fills a low-cost runClient gap:
// the -i branch must reach the production handler as an EnterInputModeMsg
// instead of sending a translate request.
func TestClientInputInitialThroughProductionPath(t *testing.T) {
	cfg := testProviderCfg("DEEPSEEK_API_KEY", "https://api.deepseek.com", "deepseek-flash")
	socketPath, ipcCh := startProductionIPCServer(t, cfg)

	var stdout bytes.Buffer
	if err := clientExchange(&stdout, socketPath, "", true, false, cfg); err != nil {
		t.Fatalf("clientExchange: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty in input-initial mode", stdout.String())
	}

	// The handler pushes the message before responding, so it is buffered.
	select {
	case msg := <-ipcCh:
		if _, ok := msg.(core.EnterInputModeMsg); !ok {
			t.Fatalf("handler message = %T, want core.EnterInputModeMsg", msg)
		}
	default:
		t.Fatal("production handler did not receive enter_input_mode")
	}
}
