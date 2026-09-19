package runtime

import (
	"context"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/ipc"
	"testing"
	"time"
)

func TestScenarioA_DeepSeekConfigStartsServer(t *testing.T) {
	socketPath := tempSocketPath(t)

	deepseekCfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
		Translation: config.TranslationConfig{SourceLang: "auto", TargetLang: "zh"},
		SocketPath:  socketPath,
	}

	fp := deepseekCfg.Fingerprint()
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
			Provider:    "openai-compatible",
			Model:       "deepseek-flash",
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)
	defer cancel()

	resp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      "status",
		RequestID: "status-a",
	})
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	if !resp.OK {
		t.Fatalf("status not OK: %s", resp.Error)
	}
	if resp.Translation != fp {
		t.Errorf("server fingerprint = %q, want %q", resp.Translation, fp)
	}
}

func TestScenarioB_SameConfigMatchesFingerprint(t *testing.T) {
	socketPath := tempSocketPath(t)

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
		Translation: config.TranslationConfig{SourceLang: "auto", TargetLang: "zh"},
		SocketPath:  socketPath,
	}

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
			Provider:    "openai-compatible",
			Model:       "deepseek-flash",
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)
	defer cancel()

	statusResp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      "status",
		RequestID: "status-b",
	})
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	if !statusResp.OK {
		t.Fatalf("status not OK: %s", statusResp.Error)
	}
	if statusResp.Translation != cfg.Fingerprint() {
		t.Fatalf("fingerprint mismatch: server=%q client=%q", statusResp.Translation, cfg.Fingerprint())
	}

	trResp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:    ipc.ProtocolVersion,
		Type:       "translate",
		RequestID:  "tr-b",
		Text:       "Hello world",
		SourceLang: "auto",
		TargetLang: "zh",
	})
	if err != nil {
		t.Fatalf("translate request failed: %v", err)
	}
	if !trResp.OK {
		t.Fatalf("translate not OK: %s", trResp.Error)
	}
	if trResp.Translation != "translated: Hello world" {
		t.Errorf("translation = %q, want %q", trResp.Translation, "translated: Hello world")
	}
}

func TestScenarioC_DifferentConfigMismatchesFingerprint(t *testing.T) {
	socketPath := tempSocketPath(t)

	serverCfg := config.Config{
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
		SocketPath:  socketPath,
	}

	clientCfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
		Translation: config.TranslationConfig{SourceLang: "auto", TargetLang: "zh"},
		SocketPath:  socketPath,
	}

	fp := serverCfg.Fingerprint()
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
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)
	defer cancel()

	statusResp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      "status",
		RequestID: "status-c",
	})
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	if !statusResp.OK {
		t.Fatalf("status not OK: %s", statusResp.Error)
	}

	if statusResp.Translation == clientCfg.Fingerprint() {
		t.Fatal("expected fingerprint mismatch but got match")
	}

	t.Logf("Server fingerprint: %s", statusResp.Translation)
	t.Logf("Client fingerprint: %s", clientCfg.Fingerprint())
	t.Log("CORRECT: fingerprints do NOT match - client would exit with configuration mismatch error")
}

func TestScenarioD_DefaultConfigVsDeepSeek(t *testing.T) {
	socketPath := tempSocketPath(t)

	defaultCfg := config.DefaultConfig()
	defaultCfg.SocketPath = socketPath

	deepseekCfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-flash",
			},
		},
		Translation: config.TranslationConfig{SourceLang: "auto", TargetLang: "zh"},
		SocketPath:  socketPath,
	}

	fp := defaultCfg.Fingerprint()
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
			Version:   ipc.ProtocolVersion,
			RequestID: req.RequestID,
			OK:        false,
			Error:     "should not reach here",
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)
	defer cancel()

	statusResp, err := ipc.SendRequest(socketPath, ipc.Request{
		Version:   ipc.ProtocolVersion,
		Type:      "status",
		RequestID: "status-d",
	})
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	if !statusResp.OK {
		t.Fatalf("status not OK: %s", statusResp.Error)
	}

	serverFP := statusResp.Translation
	clientFP := deepseekCfg.Fingerprint()

	if serverFP == clientFP {
		t.Fatal("default config and DeepSeek config should produce different fingerprints")
	}

	t.Logf("Server (default) fingerprint: %s", serverFP)
	t.Logf("Client (DeepSeek) fingerprint: %s", clientFP)
	t.Log("CORRECT: fingerprints do NOT match - client would NOT silently use OPENAI_API_KEY")
}
