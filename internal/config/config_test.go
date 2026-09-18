package config

import (
	"os"
	"path/filepath"
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

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
