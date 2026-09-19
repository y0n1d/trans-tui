package runtime

import (
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/translator"
	"testing"
)

func TestNewProvider_OpenAICompatible(t *testing.T) {
	cfg := config.Config{
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

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.OpenAICompatibleProvider); !ok {
		t.Errorf("expected *OpenAICompatibleProvider, got %T", prov)
	}
}

func TestNewProvider_Google(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "google",
			APIKeyEnv: "GOOGLE_TRANSLATE_API_KEY",
			Timeout:   30,
		},
	}

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.GoogleProvider); !ok {
		t.Errorf("expected *GoogleProvider, got %T", prov)
	}
}

func TestNewProvider_GoogleDefaultKey(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:    "google",
			Timeout: 30,
		},
	}

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.GoogleProvider); !ok {
		t.Errorf("expected *GoogleProvider, got %T", prov)
	}
}

func TestNewProvider_DeepL(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "deepl",
			APIKeyEnv: "DEEPL_API_KEY",
			Timeout:   30,
			DeepL: config.DeepLConfig{
				BaseURL: "https://api-free.deepl.com",
			},
		},
	}

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.DeepLProvider); !ok {
		t.Errorf("expected *DeepLProvider, got %T", prov)
	}
}

func TestNewProvider_DeepLDefaultKey(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:    "deepl",
			Timeout: 30,
		},
	}

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.DeepLProvider); !ok {
		t.Errorf("expected *DeepLProvider, got %T", prov)
	}
}

func TestNewProvider_LibreTranslate(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:    "libretranslate",
			Timeout: 30,
			LibreTranslate: config.LibreTranslateConfig{
				BaseURL:   "http://localhost:5000",
				APIKeyEnv: "LIBRE_API_KEY",
			},
		},
	}

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.LibreTranslateProvider); !ok {
		t.Errorf("expected *LibreTranslateProvider, got %T", prov)
	}
}

func TestNewProvider_LibreTranslateNoKey(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:    "libretranslate",
			Timeout: 30,
		},
	}

	prov, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := prov.(*translator.LibreTranslateProvider); !ok {
		t.Errorf("expected *LibreTranslateProvider, got %T", prov)
	}
}

func TestNewProvider_OpenAIRequiresKey(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:    "openai-compatible",
			Timeout: 30,
		},
	}

	_, err := newProvider(cfg)
	if err == nil {
		t.Fatal("expected error for missing API key env")
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type: "unknown-provider",
		},
	}

	_, err := newProvider(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}
