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
			// "auto": Chinese input is translated to English, anything else
			// to Chinese. Resolved by the core service before the provider
			// is called. Any explicit language code passes through unchanged.
			TargetLang: "auto",
		},
	}
}

const defaultConfigTemplate = `# trans-tui default configuration
# API keys are never stored in this file — only the environment variable name.

[provider]
# Provider type: "openai-compatible", "google", "deepl", or "libretranslate"
type = "openai-compatible"

# Name of the environment variable containing the API key
api_key_env = "OPENAI_API_KEY"

# Request timeout in seconds
timeout = 30

[provider.openai]
base_url = "https://api.openai.com/v1"
model = "gpt-4o-mini"

# [provider.deepl]
# base_url = "https://api-free.deepl.com/v2"

# [provider.libretranslate]
# base_url = "http://localhost:5000"
# api_key_env = ""

[translation]
# "auto" = Chinese input → English, otherwise → Chinese
source_lang = "auto"
target_lang = "auto"
`

// defaultConfigPath returns the platform-standard default config path:
// ${XDG_CONFIG_HOME}/trans-tui/config.toml (or equivalent via os.UserConfigDir).
func DefaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "trans-tui", "config.toml")
}

// ensureDefaultConfig creates the default config directory and template file
// if they do not yet exist. Only called when no explicit -c/--config was given.
func ensureDefaultConfig(path string) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Created default config: %s\n", path)
	}
	return nil
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		// Explicit -c/--config: file must exist.
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return Config{}, err
		}
	} else {
		// No -c: auto-discover default config path.
		dp := DefaultConfigPath()
		if dp != "" {
			if err := ensureDefaultConfig(dp); err != nil {
				// Creation failed (e.g. permission denied). Not fatal —
				// fall through to DecodeFile which handles missing file.
			}
			if _, err := toml.DecodeFile(dp, &cfg); err != nil && !os.IsNotExist(err) {
				return Config{}, err
			}
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
		runtimeDir = filepath.Join(os.TempDir(), "trans-tui")
	}
	return filepath.Join(runtimeDir, "trans-tui.sock")
}
