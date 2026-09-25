package config

import "testing"

// --- [provider].proxy ---

func TestDefaultConfigProxyIsTrue(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Provider.Proxy {
		t.Errorf("default provider.proxy = false, want true")
	}
}

// Legacy configs predate the proxy field; loading one must keep the
// historical behaviour (environment proxy honoured), i.e. Proxy == true.
func TestLoadLegacyConfigWithoutProxyIsTrue(t *testing.T) {
	path := writeTempConfig(t, "[provider]\ntype = \"openai-compatible\"\napi_key_env = \"TEST_KEY\"\ntimeout = 30\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Provider.Proxy {
		t.Errorf("legacy config provider.proxy = false, want true")
	}
}

func TestLoadExplicitProxy(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want bool
	}{
		{"explicit true", "[provider]\ntype = \"openai-compatible\"\nproxy = true\n", true},
		{"explicit false", "[provider]\ntype = \"openai-compatible\"\nproxy = false\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeTempConfig(t, tt.toml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Provider.Proxy != tt.want {
				t.Errorf("provider.proxy = %v, want %v", cfg.Provider.Proxy, tt.want)
			}
		})
	}
}

// proxy changes the network egress of the running server, so it must be
// part of the config fingerprint: a proxy=true client must not attach to a
// proxy=false server (or vice versa).
func TestFingerprintDiffersOnProxy(t *testing.T) {
	on := DefaultConfig()
	on.Provider.Proxy = true
	off := on
	off.Provider.Proxy = false

	if on.Fingerprint() == off.Fingerprint() {
		t.Errorf("proxy=true and proxy=false produce the same fingerprint")
	}

	// Changing proxy back must reproduce the original fingerprint.
	again := on
	again.Provider.Proxy = true
	if on.Fingerprint() != again.Fingerprint() {
		t.Errorf("fingerprint not deterministic for identical proxy settings")
	}

	// Existing behaviour to preserve: translation settings stay out of the
	// fingerprint even when proxy is in it.
	translated := on
	translated.Translation.SourceLang = "en"
	translated.Translation.TargetLang = "ja"
	if on.Fingerprint() != translated.Fingerprint() {
		t.Errorf("translation config must stay out of the fingerprint")
	}
}
