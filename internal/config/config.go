package config

import (
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

func defaultSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), "my-trans")
	}
	return filepath.Join(runtimeDir, "my-trans.sock")
}
