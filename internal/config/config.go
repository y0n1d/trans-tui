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
	Type      string `toml:"type"`
	APIKeyEnv string `toml:"api_key_env"`
	Timeout   int    `toml:"timeout"`
	// Proxy selects how API requests reach the network.
	//
	// true (the default): use the Go standard library's environment proxy
	// mechanism (HTTP_PROXY / HTTPS_PROXY / NO_PROXY). Without environment
	// proxy variables requests go direct, so true does not mean "force
	// traffic through a proxy".
	// false: force a direct connection, ignoring HTTP_PROXY/HTTPS_PROXY.
	//
	// Part of the config fingerprint: it changes the network egress of the
	// running server, so client and server must agree on it.
	Proxy          bool                 `toml:"proxy"`
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
//
// Every action listed by KeyBindingsConfig.actions must be non-empty after
// Load (Validate enforces this) — an action with no key could never be
// triggered. Cross-action key overlap is allowed: the TUI resolves it by a
// fixed dispatch precedence, not by rejecting the config.
type KeyBindingsConfig struct {
	Quit           []string `toml:"quit"`
	ManualInput    []string `toml:"manual_input"`
	ScrollUp       []string `toml:"scroll_up"`
	ScrollDown     []string `toml:"scroll_down"`
	PageUp         []string `toml:"page_up"`
	PageDown       []string `toml:"page_down"`
	GotoTop        []string `toml:"goto_top"`
	GotoBottom     []string `toml:"goto_bottom"`
	PreviousRecord []string `toml:"previous_record"`
	NextRecord     []string `toml:"next_record"`
	Retry          []string `toml:"retry"`
	DismissError   []string `toml:"dismiss_error"`
	CancelInput    []string `toml:"cancel_input"`
	SubmitInput    []string `toml:"submit_input"`
	CopySelection  []string `toml:"copy_selection"`
}

// keyBindingAction pairs one config field with its TOML name so Validate,
// ResolveKeyBindings and their tests iterate every action in one stable
// order instead of keeping three hand-written lists in sync.
type keyBindingAction struct {
	name string
	keys *[]string
}

// actions returns the actions in a fixed order. The receiver must be a
// pointer so the returned slice aliases the actual fields.
func (kb *KeyBindingsConfig) actions() []keyBindingAction {
	return []keyBindingAction{
		{"quit", &kb.Quit},
		{"manual_input", &kb.ManualInput},
		{"scroll_up", &kb.ScrollUp},
		{"scroll_down", &kb.ScrollDown},
		{"page_up", &kb.PageUp},
		{"page_down", &kb.PageDown},
		{"goto_top", &kb.GotoTop},
		{"goto_bottom", &kb.GotoBottom},
		{"previous_record", &kb.PreviousRecord},
		{"next_record", &kb.NextRecord},
		{"retry", &kb.Retry},
		{"dismiss_error", &kb.DismissError},
		{"cancel_input", &kb.CancelInput},
		{"submit_input", &kb.SubmitInput},
		{"copy_selection", &kb.CopySelection},
	}
}

// ---------------------------------------------------------------------------
// Appearance
// ---------------------------------------------------------------------------

// AppearanceConfig is the [appearance] TOML section: colors, section toggles
// and display flags for the TUI. Colors are ANSI palette numbers ("62",
// "235") or hex ("#rrggbb"); an empty string means "unset" (terminal
// default) for component colors. Defaults reproduce the hard-coded styles of
// the pre-config TUI, with one deliberate change: the surface-level
// background defaults to opaque ("235") instead of relying on the terminal
// background (see Background).
type AppearanceConfig struct {
	// TransparentBackground controls the surface-level background.
	//
	// false: the TUI must fill the whole terminal surface (width × height)
	// with Background, so every cell has an explicit background and the
	// window is opaque.
	// true: no surface-level background is painted at all — the terminal's
	// own background shows through. Component backgrounds stay dropped as
	// before, foregrounds/borders are kept, and the selection highlight is
	// kept. Background is not used in this mode.
	TransparentBackground bool `toml:"transparent_background"`
	// Background is the surface-level fallback background painted under the
	// entire TUI when TransparentBackground is false. It uses the same
	// parsing as the other appearance colors: an ANSI palette number
	// ("235") or hex ("#rrggbb"). Cells that already carry a component
	// background (status bar, record cards, selection, ...) keep that color.
	//
	// The default is the non-empty "235": an empty value would produce
	// default-background cells again and contradict the opaque-surface
	// meaning of transparent_background = false. An explicitly empty or
	// unparsable value disables the surface fill (no color can be painted),
	// which is documented in the README as a non-opaque configuration.
	Background  string                `toml:"background"`
	Header      HeaderAppearance      `toml:"header"`
	StatusBar   StatusBarAppearance   `toml:"status_bar"`
	Record      RecordAppearance      `toml:"record"`
	Source      SourceAppearance      `toml:"source"`
	Translation TranslationAppearance `toml:"translation"`
	Error       ErrorAppearance       `toml:"error"`
	ErrorPanel  ErrorPanelAppearance  `toml:"error_panel"`
	Loading     LoadingAppearance     `toml:"loading"`
	Input       InputAppearance       `toml:"input"`
	Selection   SelectionAppearance   `toml:"selection"`
}

type HeaderAppearance struct {
	Enabled         bool   `toml:"enabled"`
	Title           string `toml:"title"`
	ShowRecordCount bool   `toml:"show_record_count"`
	Foreground      string `toml:"foreground"`
	Background      string `toml:"background"`
	Bold            bool   `toml:"bold"`
	PaddingLeft     int    `toml:"padding_left"`
	PaddingRight    int    `toml:"padding_right"`
}

type StatusBarAppearance struct {
	Enabled            bool   `toml:"enabled"`
	ShowRecordCount    bool   `toml:"show_record_count"`
	ShowScrollPosition bool   `toml:"show_scroll_position"`
	ShowInputHint      bool   `toml:"show_input_hint"`
	ShowQuitHint       bool   `toml:"show_quit_hint"`
	ShowCancelHint     bool   `toml:"show_cancel_hint"`
	Foreground         string `toml:"foreground"`
	Background         string `toml:"background"`
	PaddingLeft        int    `toml:"padding_left"`
	PaddingRight       int    `toml:"padding_right"`
}

type RecordAppearance struct {
	BorderForeground string `toml:"border_foreground"`
	Background       string `toml:"background"`
}

type SourceAppearance struct {
	Foreground string `toml:"foreground"`
	Bold       bool   `toml:"bold"`
}

type TranslationAppearance struct {
	Foreground string `toml:"foreground"`
}

type ErrorAppearance struct {
	Foreground string `toml:"foreground"`
	Bold       bool   `toml:"bold"`
}

type ErrorPanelAppearance struct {
	BorderForeground string `toml:"border_foreground"`
	Foreground       string `toml:"foreground"`
	Background       string `toml:"background"`
}

type LoadingAppearance struct {
	Foreground string `toml:"foreground"`
	Bold       bool   `toml:"bold"`
	Text       string `toml:"text"`
}

type InputAppearance struct {
	BorderForeground string `toml:"border_foreground"`
	Background       string `toml:"background"`
	PromptForeground string `toml:"prompt_foreground"`
	PromptBold       bool   `toml:"prompt_bold"`
}

type SelectionAppearance struct {
	Background string `toml:"background"`
	Foreground string `toml:"foreground"`
}

// DefaultAppearance returns the appearance defaults. They reproduce the
// hard-coded styles of the original TUI exactly, so omitting [appearance]
// changes nothing about the rendered output.
func DefaultAppearance() AppearanceConfig {
	return AppearanceConfig{
		TransparentBackground: false,
		// Non-empty on purpose: with transparent_background = false the
		// surface must be fillable, so there is never a default of "".
		Background: "235",
		Header: HeaderAppearance{
			Enabled:         true,
			Title:           "trans-tui",
			ShowRecordCount: true,
			Foreground:      "229",
			Background:      "",
			Bold:            true,
			PaddingLeft:     1,
			PaddingRight:    1,
		},
		StatusBar: StatusBarAppearance{
			Enabled:            true,
			ShowRecordCount:    true,
			ShowScrollPosition: true,
			ShowInputHint:      true,
			ShowQuitHint:       true,
			ShowCancelHint:     true,
			Foreground:         "241",
			Background:         "235",
			PaddingLeft:        1,
			PaddingRight:       1,
		},
		Record: RecordAppearance{
			BorderForeground: "62",
			Background:       "",
		},
		Source: SourceAppearance{
			Foreground: "241",
			Bold:       true,
		},
		Translation: TranslationAppearance{
			Foreground: "86",
		},
		Error: ErrorAppearance{
			Foreground: "196",
			Bold:       true,
		},
		ErrorPanel: ErrorPanelAppearance{
			BorderForeground: "196",
			Foreground:       "196",
			Background:       "",
		},
		Loading: LoadingAppearance{
			Foreground: "205",
			Bold:       true,
			Text:       "Translating...",
		},
		Input: InputAppearance{
			BorderForeground: "62",
			Background:       "",
			PromptForeground: "86",
			PromptBold:       true,
		},
		Selection: SelectionAppearance{
			Background: "240",
			Foreground: "15",
		},
	}
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
	Provider    ProviderConfig    `toml:"provider"`
	Translation TranslationConfig `toml:"translation"`
	OCR         OCRConfig         `toml:"ocr"`
	KeyBindings KeyBindingsConfig `toml:"keybindings"`
	Appearance  AppearanceConfig  `toml:"appearance"`
	SocketPath  string            `toml:"-"`
}

func DefaultConfig() Config {
	return Config{
		Provider: ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "OPENAI_API_KEY",
			Timeout:   30,
			// true = Go's environment proxy (HTTP_PROXY/HTTPS_PROXY/NO_PROXY);
			// direct connection when no proxy variables are set.
			Proxy: true,
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
		KeyBindings: DefaultKeyBindings(),
		Appearance:  DefaultAppearance(),
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

# Proxy for API requests:
#   true  = use the environment proxy (HTTP_PROXY / HTTPS_PROXY / NO_PROXY)
#           when those are set; without them requests go direct
#   false = always connect directly, ignoring HTTP_PROXY / HTTPS_PROXY
# true does not force traffic through a proxy — no environment proxy means
# a direct connection either way.
proxy = true

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

[appearance]
# Surface-level background of the TUI.
#   false = paint background across the whole terminal surface (width x
#           height) so the window is opaque; cells that belong to a
#           component with its own background keep that color.
#   true  = don't paint a surface-level background; let the terminal/foot
#           background show through. Component backgrounds are dropped and
#           the selection highlight is kept. background is then unused.
transparent_background = false
# Surface background color: ANSI palette number or hex. Must be non-empty
# for an opaque window; the default "235" guarantees that.
background = "235"

[appearance.header]
# The one-line title row at the top of the window.
enabled = true
title = "trans-tui"
show_record_count = true
foreground = "229"
background = ""
bold = true
padding_left = 1
padding_right = 1

[appearance.status_bar]
# The one-line info row at the bottom of the window.
enabled = true
show_record_count = true
show_scroll_position = true
show_input_hint = true
show_quit_hint = true
show_cancel_hint = true
foreground = "241"
background = "235"
padding_left = 1
padding_right = 1

[appearance.record]
# Translation cards in the history list.
border_foreground = "62"
background = ""

[appearance.source]
foreground = "241"
bold = true

[appearance.translation]
foreground = "86"

[appearance.error]
# Error text inside a translation card.
foreground = "196"
bold = true

[appearance.error_panel]
# The bordered warning panel below the history list.
border_foreground = "196"
foreground = "196"
background = ""

[appearance.loading]
foreground = "205"
bold = true
text = "Translating..."

[appearance.input]
# The bordered text input panel.
border_foreground = "62"
background = ""
prompt_foreground = "86"
prompt_bold = true

[appearance.selection]
# Highlight applied to mouse-selected history text.
background = "240"
foreground = "15"

[keybindings]
# Every action needs at least one key; an empty list is a config error.
# Keys are Bubble Tea key names: "q", "esc", "enter", "up", "pgdown",
# "ctrl+c", "ctrl+shift+c", ... Overlapping keys across actions are allowed;
# the TUI resolves them in a fixed order (dismiss error > quit > manual
# input > scrolling > navigation > retry; in input mode copy > cancel >
# submit > typing).
quit = ["q", "ctrl+c", "esc"]
manual_input = [",", "，"]
scroll_up = ["up", "k"]
scroll_down = ["down", "j"]
page_up = ["pgup", "b"]
page_down = ["pgdown", "f"]
goto_top = ["home", "g"]
goto_bottom = ["end", "G"]
previous_record = ["h"]
next_record = ["l"]
retry = ["r"]
dismiss_error = ["esc"]
cancel_input = ["esc"]
submit_input = ["enter"]
copy_selection = ["ctrl+shift+c"]
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
		Quit:           []string{"q", "ctrl+c", "esc"},
		ManualInput:    []string{",", "，"},
		ScrollUp:       []string{"up", "k"},
		ScrollDown:     []string{"down", "j"},
		PageUp:         []string{"pgup", "b"},
		PageDown:       []string{"pgdown", "f"},
		GotoTop:        []string{"home", "g"},
		GotoBottom:     []string{"end", "G"},
		PreviousRecord: []string{"h"},
		NextRecord:     []string{"l"},
		Retry:          []string{"r"},
		DismissError:   []string{"esc"},
		CancelInput:    []string{"esc"},
		SubmitInput:    []string{"enter"},
		CopySelection:  []string{"ctrl+shift+c"},
	}
}

// Validate checks that every configurable action has at least one key.
// Cross-action overlaps (esc on several actions, for example) are legal —
// the TUI resolves them by dispatch precedence — so only empty lists fail.
func (kb KeyBindingsConfig) Validate() error {
	for _, a := range kb.actions() {
		if len(*a.keys) == 0 {
			return fmt.Errorf("keybindings.%s must not be empty", a.name)
		}
	}
	return nil
}

// ResolveKeyBindings merges user-supplied keybindings with the program
// defaults: any action left empty (section omitted, field omitted, or a
// programmatically built Config{}) falls back to its default keys.
func ResolveKeyBindings(kb KeyBindingsConfig) KeyBindingsConfig {
	def := DefaultKeyBindings()
	got, want := kb.actions(), def.actions()
	for i := range got {
		if len(*got[i].keys) == 0 {
			*got[i].keys = *want[i].keys
		}
	}
	return kb
}

// ResolveKeyBindings merges the user-supplied keybindings with the program
// defaults. Empty/missing fields fall back to the program defaults.
func (c Config) ResolveKeyBindings() KeyBindingsConfig {
	return ResolveKeyBindings(c.KeyBindings)
}

type fingerprintInput struct {
	Type                    string `json:"type"`
	APIKeyEnv               string `json:"api_key_env"`
	Timeout                 int    `json:"timeout"`
	Proxy                   bool   `json:"proxy"`
	OpenAIBaseURL           string `json:"openai_base_url,omitempty"`
	OpenAIModel             string `json:"openai_model,omitempty"`
	DeepLBaseURL            string `json:"deepl_base_url,omitempty"`
	LibreTranslateBaseURL   string `json:"libretranslate_base_url,omitempty"`
	LibreTranslateAPIKeyEnv string `json:"libretranslate_api_key_env,omitempty"`
	// Appearance and keybindings are part of the fingerprint: a client whose
	// TUI would render or bind keys differently from the running server must
	// not attach to it. Translation languages are deliberately excluded —
	// they are per-invocation settings, not provider identity.
	Appearance  AppearanceConfig  `json:"appearance"`
	KeyBindings KeyBindingsConfig `json:"keybindings"`
}

func (c Config) Fingerprint() string {
	input := fingerprintInput{
		Type:      c.Provider.Type,
		APIKeyEnv: c.Provider.APIKeyEnv,
		Timeout:   c.Provider.Timeout,
		// Proxy is deliberately part of the fingerprint: proxy=true and
		// proxy=false leave the process through different network egresses,
		// so a client and server disagreeing on it must not share a socket.
		Proxy:                   c.Provider.Proxy,
		OpenAIBaseURL:           c.Provider.OpenAI.BaseURL,
		OpenAIModel:             c.Provider.OpenAI.Model,
		DeepLBaseURL:            c.Provider.DeepL.BaseURL,
		LibreTranslateBaseURL:   c.Provider.LibreTranslate.BaseURL,
		LibreTranslateAPIKeyEnv: c.Provider.LibreTranslate.APIKeyEnv,
		Appearance:              c.Appearance,
		KeyBindings:             c.KeyBindings,
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
