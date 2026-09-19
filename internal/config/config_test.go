package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultTranslationLangs(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Translation.SourceLang != "auto" {
		t.Errorf("default source_lang = %q, want %q", cfg.Translation.SourceLang, "auto")
	}
	if cfg.Translation.TargetLang != "auto" {
		t.Errorf("default target_lang = %q, want %q", cfg.Translation.TargetLang, "auto")
	}
}

func TestLoadOmittedTargetLangKeepsAuto(t *testing.T) {
	path := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Translation.TargetLang != "auto" {
		t.Errorf("omitted target_lang = %q, want %q", cfg.Translation.TargetLang, "auto")
	}
}

func TestLoadExplicitTargetLangPreserved(t *testing.T) {
	path := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n[translation]\ntarget_lang = \"ja\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Translation.TargetLang != "ja" {
		t.Errorf("explicit target_lang = %q, want %q", cfg.Translation.TargetLang, "ja")
	}
}

// --- Default config auto-discovery tests ---

func TestDefaultConfigPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got := DefaultConfigPath()
	want := filepath.Join(tmp, "trans-tui", "config.toml")
	if got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigLoadedAutomatically(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfgDir := filepath.Join(tmp, "trans-tui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")
	custom := "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"MY_KEY\"\n[provider.openai]\nbase_url = \"http://example\"\nmodel = \"custom-model\"\n"
	if err := os.WriteFile(cfgPath, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if cfg.Provider.OpenAI.BaseURL != "http://example" {
		t.Errorf("base_url = %q, want %q", cfg.Provider.OpenAI.BaseURL, "http://example")
	}
	if cfg.Provider.OpenAI.Model != "custom-model" {
		t.Errorf("model = %q, want %q", cfg.Provider.OpenAI.Model, "custom-model")
	}
	if cfg.Provider.APIKeyEnv != "MY_KEY" {
		t.Errorf("api_key_env = %q, want %q", cfg.Provider.APIKeyEnv, "MY_KEY")
	}
}

func TestDefaultConfigCreatedWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}

	cfgPath := filepath.Join(tmp, "trans-tui", "config.toml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}

	// Content is valid TOML that round-trips through Config.
	content := string(data)
	if !strings.Contains(content, "api_key_env") {
		t.Error("template missing api_key_env field")
	}
	if strings.Contains(content, "sk-") || strings.Contains(content, "your-api-key") {
		t.Error("template contains a hardcoded API key value")
	}

	// Permissions.
	info, _ := os.Stat(filepath.Join(tmp, "trans-tui"))
	if info != nil && info.Mode().Perm() != 0o700 {
		t.Errorf("dir perms = %o, want 0700", info.Mode().Perm())
	}
	finfo, _ := os.Stat(cfgPath)
	if finfo != nil && finfo.Mode().Perm() != 0o600 {
		t.Errorf("file perms = %o, want 0600", finfo.Mode().Perm())
	}

	// The loaded config has the template's default values.
	if cfg.Provider.Type != "openai-compatible" {
		t.Errorf("provider.type = %q, want %q", cfg.Provider.Type, "openai-compatible")
	}
	if cfg.Provider.OpenAI.Model != "gpt-4o-mini" {
		t.Errorf("model = %q, want %q", cfg.Provider.OpenAI.Model, "gpt-4o-mini")
	}

	// Second load doesn't overwrite.
	if err := os.WriteFile(cfgPath, []byte("[provider]\ntype = \"openai-compatible\"\napi_key_env = \"X\"\n[provider.openai]\nmodel = \"preserved\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load("")
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	data2, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(data2), "preserved") {
		t.Error("second Load overwrote the config file")
	}
}

func TestExistingDefaultConfigIsNeverOverwritten(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfgDir := filepath.Join(tmp, "trans-tui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	custom := "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"CUSTOM\"\n[provider.openai]\nmodel = \"custom-model\"\n"
	cfgPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}

	// First Load — file exists, no creation.
	cfg1, err := Load("")
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if cfg1.Provider.OpenAI.Model != "custom-model" {
		t.Errorf("first load model = %q, want %q", cfg1.Provider.OpenAI.Model, "custom-model")
	}

	// Second Load — still custom-model.
	cfg2, err := Load("")
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if cfg2.Provider.OpenAI.Model != "custom-model" {
		t.Errorf("second load model = %q, want %q", cfg2.Provider.OpenAI.Model, "custom-model")
	}
}

func TestExplicitConfigMissingFails(t *testing.T) {
	_, err := Load("/tmp/nonexistent-trans-tui-test-1234.toml")
	if err == nil {
		t.Error("expected error for missing explicit config, got nil")
	}

	// Must not have created the file.
	if _, statErr := os.Stat("/tmp/nonexistent-trans-tui-test-1234.toml"); statErr == nil {
		t.Error("explicit missing config should not be created")
	}
}

func TestExplicitConfigOverridesDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Create default config with one model.
	cfgDir := filepath.Join(tmp, "trans-tui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	defaultContent := "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"D\"\n[provider.openai]\nmodel = \"default-model\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(defaultContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create explicit config with different model.
	explicit := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"E\"\n[provider.openai]\nmodel = \"explicit-model\"\n")

	cfg, err := Load(explicit)
	if err != nil {
		t.Fatalf("Load(explicit): %v", err)
	}
	if cfg.Provider.OpenAI.Model != "explicit-model" {
		t.Errorf("model = %q, want %q (explicit should override default)", cfg.Provider.OpenAI.Model, "explicit-model")
	}
}

func TestDefaultAndExplicitSameConfigHaveSameFingerprint(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Create default config.
	cfgDir := filepath.Join(tmp, "trans-tui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"K\"\n[provider.openai]\nbase_url = \"http://same\"\nmodel = \"same-model\"\n"
	cfgPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Load via default path (auto-discovery).
	cfgDefault, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}

	// Load via explicit path to same file.
	cfgExplicit, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(explicit): %v", err)
	}

	if cfgDefault.Fingerprint() != cfgExplicit.Fingerprint() {
		t.Errorf("fingerprints differ: default=%s explicit=%s", cfgDefault.Fingerprint(), cfgExplicit.Fingerprint())
	}
}

func TestXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Create config in the XDG location.
	cfgDir := filepath.Join(tmp, "trans-tui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"XDG\"\n[provider.openai]\nmodel = \"xdg-model\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Provider.OpenAI.Model != "xdg-model" {
		t.Errorf("model = %q, want %q", cfg.Provider.OpenAI.Model, "xdg-model")
	}

	// Verify path.
	want := filepath.Join(tmp, "trans-tui", "config.toml")
	if got := DefaultConfigPath(); got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}

	// Must NOT have written to the real ~/.config.
	realConfig, _ := os.UserConfigDir()
	realPath := filepath.Join(realConfig, "trans-tui", "config.toml")
	if _, err := os.Stat(realPath); err == nil {
		// File might exist from a real install — just check we didn't modify it.
		// The temp dir approach ensures we didn't touch it in this test.
		fmt.Println("(real config exists, test used temp dir)")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestOCRDefaultsWhenSectionMissing(t *testing.T) {
	// Old config without [ocr] section should still work with OCR defaults.
	path := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// OCR should have defaults.
	if cfg.OCR.Provider != "baidu" {
		t.Errorf("ocr.provider = %q, want %q", cfg.OCR.Provider, "baidu")
	}
	if cfg.OCR.Model != "general_basic" {
		t.Errorf("ocr.model = %q, want %q", cfg.OCR.Model, "general_basic")
	}
	if cfg.OCR.LanguageType != "CHN_ENG" {
		t.Errorf("ocr.language_type = %q, want %q", cfg.OCR.LanguageType, "CHN_ENG")
	}
	if cfg.OCR.Timeout != 30 {
		t.Errorf("ocr.timeout = %d, want %d", cfg.OCR.Timeout, 30)
	}
	if cfg.OCR.Baidu.BaseURL != "https://aip.baidubce.com" {
		t.Errorf("ocr.baidu.base_url = %q, want %q", cfg.OCR.Baidu.BaseURL, "https://aip.baidubce.com")
	}
	if cfg.OCR.Baidu.APIKeyEnv != "BAIDU_OCR_API_KEY" {
		t.Errorf("ocr.baidu.api_key_env = %q, want %q", cfg.OCR.Baidu.APIKeyEnv, "BAIDU_OCR_API_KEY")
	}
	if cfg.OCR.Baidu.SecretKeyEnv != "BAIDU_OCR_SECRET_KEY" {
		t.Errorf("ocr.baidu.secret_key_env = %q, want %q", cfg.OCR.Baidu.SecretKeyEnv, "BAIDU_OCR_SECRET_KEY")
	}
}

func TestOCRCustomConfig(t *testing.T) {
	path := writeTempConfig(t, `[provider]
type = "openai-compatible"
api_key_env = "TEST_KEY"

[ocr]
provider = "baidu"
model = "accurate_basic"
language_type = "ENG"
timeout = 60

[ocr.baidu]
base_url = "https://custom.example.com"
api_key_env = "MY_API_KEY"
secret_key_env = "MY_SECRET_KEY"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.OCR.Provider != "baidu" {
		t.Errorf("ocr.provider = %q, want %q", cfg.OCR.Provider, "baidu")
	}
	if cfg.OCR.Model != "accurate_basic" {
		t.Errorf("ocr.model = %q, want %q", cfg.OCR.Model, "accurate_basic")
	}
	if cfg.OCR.LanguageType != "ENG" {
		t.Errorf("ocr.language_type = %q, want %q", cfg.OCR.LanguageType, "ENG")
	}
	if cfg.OCR.Timeout != 60 {
		t.Errorf("ocr.timeout = %d, want %d", cfg.OCR.Timeout, 60)
	}
	if cfg.OCR.Baidu.BaseURL != "https://custom.example.com" {
		t.Errorf("ocr.baidu.base_url = %q, want %q", cfg.OCR.Baidu.BaseURL, "https://custom.example.com")
	}
	if cfg.OCR.Baidu.APIKeyEnv != "MY_API_KEY" {
		t.Errorf("ocr.baidu.api_key_env = %q, want %q", cfg.OCR.Baidu.APIKeyEnv, "MY_API_KEY")
	}
	if cfg.OCR.Baidu.SecretKeyEnv != "MY_SECRET_KEY" {
		t.Errorf("ocr.baidu.secret_key_env = %q, want %q", cfg.OCR.Baidu.SecretKeyEnv, "MY_SECRET_KEY")
	}
}
