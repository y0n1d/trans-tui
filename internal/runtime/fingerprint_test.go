package runtime

import (
	"context"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/ipc"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.sock")
}

func fingerprintServer(t *testing.T, cfg config.Config) (string, func()) {
	t.Helper()
	socketPath := tempSocketPath(t)

	fp := cfg.Fingerprint()
	handler := func(ctx context.Context, req ipc.Request) ipc.Response {
		if req.Type == "status" {
			return ipc.Response{
				Version:     ipc.ProtocolVersion,
				RequestID:   req.RequestID,
				OK:          true,
				Translation: fp,
			}
		}
		return ipc.Response{
			Version:     ipc.ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: "translated: " + req.Text,
			Provider:    "test",
			Model:       "test",
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	return socketPath, cancel
}

func TestFingerprintDeterministic(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
	}

	fp1 := cfg.Fingerprint()
	fp2 := cfg.Fingerprint()
	if fp1 != fp2 {
		t.Errorf("Fingerprint not deterministic: %q != %q", fp1, fp2)
	}
}

func TestFingerprintDifferentConfigs(t *testing.T) {
	cfg1 := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "OPENAI_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			},
		},
	}
	cfg2 := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
	}

	if cfg1.Fingerprint() == cfg2.Fingerprint() {
		t.Error("different configs should produce different fingerprints")
	}
}

func TestFingerprintIgnoresTranslationConfig(t *testing.T) {
	cfg1 := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "OPENAI_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			},
		},
		Translation: config.TranslationConfig{SourceLang: "auto", TargetLang: "zh"},
	}
	cfg2 := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "OPENAI_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			},
		},
		Translation: config.TranslationConfig{SourceLang: "en", TargetLang: "ja"},
	}

	if cfg1.Fingerprint() != cfg2.Fingerprint() {
		t.Error("translation config should not affect fingerprint")
	}
}

func TestFingerprintIgnoresAPIKeyValue(t *testing.T) {
	os.Setenv("TEST_KEY_A", "secret-value-1")
	os.Setenv("TEST_KEY_B", "secret-value-2")
	defer os.Unsetenv("TEST_KEY_A")
	defer os.Unsetenv("TEST_KEY_B")

	cfg1 := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "TEST_KEY_A",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
	}
	cfg2 := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "TEST_KEY_B",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
	}

	if cfg1.Fingerprint() == cfg2.Fingerprint() {
		t.Error("different API key env vars should produce different fingerprints")
	}
}

func TestFingerprintLength(t *testing.T) {
	cfg := config.DefaultConfig()
	fp := cfg.Fingerprint()
	if len(fp) != 64 {
		t.Errorf("fingerprint length = %d, want 64", len(fp))
	}
}

func TestServerStatusRequest(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "TEST_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.test.com",
				Model:   "test-model",
			},
		},
	}

	socketPath, cancel := fingerprintServer(t, cfg)
	defer cancel()

	resp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      "status",
		RequestID: "status-001",
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if !resp.OK {
		t.Fatalf("expected OK=true, got error: %s", resp.Error)
	}

	expectedFP := cfg.Fingerprint()
	if resp.Translation != expectedFP {
		t.Errorf("fingerprint = %q, want %q", resp.Translation, expectedFP)
	}
}

func TestServerTranslationStillWorks(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "TEST_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.test.com",
				Model:   "test-model",
			},
		},
	}

	socketPath, cancel := fingerprintServer(t, cfg)
	defer cancel()

	resp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       "translate",
		RequestID:  "tr-001",
		Text:       "hello",
		SourceLang: "en",
		TargetLang: "fr",
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if !resp.OK {
		t.Fatalf("expected OK=true, got error: %s", resp.Error)
	}

	if resp.Translation != "translated: hello" {
		t.Errorf("translation = %q, want %q", resp.Translation, "translated: hello")
	}
}
