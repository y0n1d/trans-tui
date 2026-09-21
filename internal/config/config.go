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

// KeyBindingsConfig holds configurable TUI key bindings.
// Each field is a list of key strings accepted by Bubble Tea
// (e.g. "q", "esc", ",", "，", "ctrl+c").
type KeyBindingsConfig struct {
	Quit         []string `toml:"quit"`
	ManualInput  []string `toml:"manual_input"`
}

type BaiduOCRConfig struct {
	BaseURL      string `toml:"base_url"`
	APIKeyEnv    string `toml:"api_key_env"`
	SecretKeyEnv string `toml:"secret_key_env"`
}

type OCRConfig struct {
	Provider     string         `toml:"provider"`
	Model        string         `toml:"model"`
	LanguageType string         `toml:"language_type"`
	Timeout      int            `toml:"timeout"`
	Baidu        BaiduOCRConfig `toml:"baidu"`
}

type Config struct {
	Provider    ProviderConfig      `toml:"provider"`
	Translation TranslationConfig  `toml:"translation"`
	OCR         OCRConfig           `toml:"ocr"`
	KeyBindings KeyBindingsConfig   `toml:"keybindings"`
	SocketPath  string              `toml:"-"`
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
		OCR: OCRConfig{
			Provider:     "baidu",
			Model:        "general_basic",
			LanguageType: "CHN_ENG",
			Timeout:      30,
			Baidu: BaiduOCRConfig{
				BaseURL:      "https://aip.baidubce.com",
				APIKeyEnv:    "BAIDU_OCR_API_KEY",
				SecretKeyEnv: "BAIDU_OCR_SECRET_KEY",
			},
		},
		KeyBindings: KeyBindingsConfig{
			Quit:        []string{"q", "ctrl+c", "esc"},
			ManualInput: []string{",", "，"},
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

[ocr]
# OCR provider: "baidu"
provider = "baidu"

# OCR model/endpoint: "general_basic" (standard), "accurate_basic" (high precision)
model = "general_basic"

# Language type for Baidu OCR
language_type = "CHN_ENG"

# Request timeout in seconds
timeout = 30

[ocr.baidu]
base_url = "https://aip.baidubce.com"
api_key_env = "BAIDU_OCR_API_KEY"
secret_key_env = "BAIDU_OCR_SECRET_KEY"

# [keybindings]
# quit = ["q", "ctrl+c", "esc"]
# manual_input = [",", "，"]
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
	if err := c.KeyBindings.Validate(); err != nil {
		return err
	}
	return nil
}

// DefaultKeyBindings returns the program's hardcoded default key bindings.
func DefaultKeyBindings() KeyBindingsConfig {
	return KeyBindingsConfig{
		Quit:        []string{"q", "ctrl+c", "esc"},
		ManualInput: []string{",", "，"},
	}
}

// Validate checks that keybindings are syntactically valid and non-empty.
func (kb KeyBindingsConfig) Validate() error {
	if len(kb.Quit) == 0 {
		return fmt.Errorf("keybindings.quit must not be empty")
	}
	if len(kb.ManualInput) == 0 {
		return fmt.Errorf("keybindings.manual_input must not be empty")
	}
	return nil
}

// ResolveKeyBindings merges the user-supplied keybindings with the program
// defaults. Empty/missing fields fall back to the program defaults.
func (c Config) ResolveKeyBindings() KeyBindingsConfig {
	def := DefaultKeyBindings()
	kb := c.KeyBindings
	if len(kb.Quit) == 0 {
		kb.Quit = def.Quit
	}
	if len(kb.ManualInput) == 0 {
		kb.ManualInput = def.ManualInput
	}
	return kb
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
