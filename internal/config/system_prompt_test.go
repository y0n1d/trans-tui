package config

import (
	"path/filepath"
	"testing"
)

// --- [translation].system_prompt ---

// The default prompt is the byte-identical text that used to be hard-coded
// in the OpenAI-compatible provider; configs without the field must keep it.
func TestDefaultSystemPrompt(t *testing.T) {
	const want = "You are a professional translator. Translate the user's text accurately and naturally."

	if DefaultSystemPrompt != want {
		t.Errorf("DefaultSystemPrompt = %q, want %q", DefaultSystemPrompt, want)
	}
	if got := DefaultConfig().Translation.SystemPrompt; got != want {
		t.Errorf("DefaultConfig().Translation.SystemPrompt = %q, want %q", got, want)
	}
}

// Legacy configs predate the field; loading one must keep the historical
// prompt (mirrors TestLoadLegacyConfigWithoutProxyIsTrue).
func TestLoadLegacyConfigWithoutSystemPrompt(t *testing.T) {
	path := writeTempConfig(t, "[translation]\nsource_lang = \"auto\"\ntarget_lang = \"auto\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Translation.SystemPrompt != DefaultSystemPrompt {
		t.Errorf("legacy config system_prompt = %q, want %q", cfg.Translation.SystemPrompt, DefaultSystemPrompt)
	}
}

// An explicitly empty system_prompt means "unconfigured": it loads as the
// default prompt, not as "no system prompt at all".
func TestLoadEmptySystemPromptUsesDefault(t *testing.T) {
	path := writeTempConfig(t, "[translation]\nsystem_prompt = \"\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Translation.SystemPrompt != DefaultSystemPrompt {
		t.Errorf("empty system_prompt loaded as %q, want %q", cfg.Translation.SystemPrompt, DefaultSystemPrompt)
	}
}

// Omitting the field and writing the default prompt explicitly must produce
// the same fingerprint: the fingerprint hashes the resolved value, not
// whether the TOML key appeared.
func TestSystemPromptOmittedEqualsExplicitDefaultFingerprint(t *testing.T) {
	omitted := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n[translation]\nsource_lang = \"auto\"\ntarget_lang = \"auto\"\n")
	explicit := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n[translation]\nsource_lang = \"auto\"\ntarget_lang = \"auto\"\nsystem_prompt = \""+DefaultSystemPrompt+"\"\n")
	empty := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\n[translation]\nsystem_prompt = \"\"\n")

	cfgOmitted, err := Load(omitted)
	if err != nil {
		t.Fatalf("Load(omitted): %v", err)
	}
	cfgExplicit, err := Load(explicit)
	if err != nil {
		t.Fatalf("Load(explicit): %v", err)
	}
	cfgEmpty, err := Load(empty)
	if err != nil {
		t.Fatalf("Load(empty): %v", err)
	}

	if cfgOmitted.Fingerprint() != cfgExplicit.Fingerprint() {
		t.Error("omitted and explicitly-default system_prompt must share one fingerprint")
	}
	if cfgOmitted.Fingerprint() != cfgEmpty.Fingerprint() {
		t.Error("empty system_prompt must fingerprint like the default")
	}
}

// A custom prompt changes the fingerprint (a client with a different prompt
// must not attach to a running server), and restoring the default restores
// the original fingerprint.
func TestFingerprintDiffersOnSystemPrompt(t *testing.T) {
	base := DefaultConfig()

	custom := base
	custom.Translation.SystemPrompt = "You are a concise bilingual subtitle translator."
	if base.Fingerprint() == custom.Fingerprint() {
		t.Error("custom system_prompt did not change the fingerprint")
	}

	restored := custom
	restored.Translation.SystemPrompt = DefaultSystemPrompt
	if base.Fingerprint() != restored.Fingerprint() {
		t.Error("fingerprint not restored after reverting to the default prompt")
	}

	// Translation languages stay out of the fingerprint even now that the
	// prompt is in it.
	langs := base
	langs.Translation.SourceLang = "en"
	langs.Translation.TargetLang = "ja"
	if base.Fingerprint() != langs.Fingerprint() {
		t.Error("translation languages must stay out of the fingerprint")
	}
}

// The auto-created config template must document the field and round-trip to
// exactly the default value it shows.
func TestCreatedTemplateSystemPromptRoundTrips(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if cfg.Translation.SystemPrompt != DefaultSystemPrompt {
		t.Errorf("template system_prompt = %q, want %q", cfg.Translation.SystemPrompt, DefaultSystemPrompt)
	}

	data, err := filepath.Glob(filepath.Join(tmp, "trans-tui", "config.toml"))
	if err != nil || len(data) != 1 {
		t.Fatalf("default config not created: %v", err)
	}
}
