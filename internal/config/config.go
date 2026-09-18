package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type OpenAIConfig struct {
	BaseURL string `toml:"base_url"`
	Model   string `toml:"model"`
}

type GoogleConfig struct {
}

type LibreTranslateConfig struct {
	BaseURL   string `toml:"base_url"`
	APIKeyEnv string `toml:"api_key_env"`
}

type DeepLConfig struct {
	BaseURL string `toml:"base_url"`
}

type ProviderConfig struct {
	Type           string               `toml:"type"`
	APIKeyEnv      string               `toml:"api_key_env"`
	Timeout        int                  `toml:"timeout"`
	OpenAI         OpenAIConfig         `toml:"openai"`
	DeepL          DeepLConfig          `toml:"deepl"`
	LibreTranslate LibreTranslateConfig `toml:"libretranslate"`
}

type TranslationConfig struct {
	SourceLang string `toml:"source_lang"`
	TargetLang string `toml:"target_lang"`
}

type Config struct {
	Provider    ProviderConfig    `toml:"provider"`
	Translation TranslationConfig `toml:"translation"`
	SocketPath  string            `toml:"-"`
}

func DefaultConfig() Config {
	return Config{
		Provider: ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "OPENAI_API_KEY",
			Timeout:   30,
			OpenAI: OpenAIConfig{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			},
		},
		Translation: TranslationConfig{
			SourceLang: "auto",
			TargetLang: "zh",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return Config{}, err
		}
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = defaultSocketPath()
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Provider.Type == "" {
		return fmt.Errorf("provider type is required")
	}
	return nil
}

type fingerprintInput struct {
	Type                    string `json:"type"`
	APIKeyEnv               string `json:"api_key_env"`
	Timeout                 int    `json:"timeout"`
	OpenAIBaseURL           string `json:"openai_base_url,omitempty"`
	OpenAIModel             string `json:"openai_model,omitempty"`
	DeepLBaseURL            string `json:"deepl_base_url,omitempty"`
	LibreTranslateBaseURL   string `json:"libretranslate_base_url,omitempty"`
	LibreTranslateAPIKeyEnv string `json:"libretranslate_api_key_env,omitempty"`
}

func (c Config) Fingerprint() string {
	input := fingerprintInput{
		Type:                    c.Provider.Type,
		APIKeyEnv:               c.Provider.APIKeyEnv,
		Timeout:                 c.Provider.Timeout,
		OpenAIBaseURL:           c.Provider.OpenAI.BaseURL,
		OpenAIModel:             c.Provider.OpenAI.Model,
		DeepLBaseURL:            c.Provider.DeepL.BaseURL,
		LibreTranslateBaseURL:   c.Provider.LibreTranslate.BaseURL,
		LibreTranslateAPIKeyEnv: c.Provider.LibreTranslate.APIKeyEnv,
	}

	h := sha256.Sum256([]byte(fmt.Sprintf("%+v", input)))
	return hex.EncodeToString(h[:])
}

func defaultSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), "my-trans")
	}
	return filepath.Join(runtimeDir, "my-trans.sock")
}
