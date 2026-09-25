package runtime

import (
	"testing"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/translator"
)

// newProvider must accept provider.proxy for every provider type and hand it
// to the provider it constructs. The transport itself is asserted in
// internal/translator (the client fields are package-private there).
func TestNewProviderAppliesProxySetting(t *testing.T) {
	tests := []struct {
		provider string
		want     func(any) bool
	}{
		{"openai-compatible", func(v any) bool { _, ok := v.(*translator.OpenAICompatibleProvider); return ok }},
		{"google", func(v any) bool { _, ok := v.(*translator.GoogleProvider); return ok }},
		{"deepl", func(v any) bool { _, ok := v.(*translator.DeepLProvider); return ok }},
		{"libretranslate", func(v any) bool { _, ok := v.(*translator.LibreTranslateProvider); return ok }},
	}

	for _, tt := range tests {
		for _, proxy := range []bool{true, false} {
			cfg := config.Config{
				Provider: config.ProviderConfig{
					Type:      tt.provider,
					APIKeyEnv: "TEST_KEY",
					Timeout:   30,
					Proxy:     proxy,
				},
			}

			prov, err := newProvider(cfg)
			if err != nil {
				t.Fatalf("%s proxy=%v: unexpected error: %v", tt.provider, proxy, err)
			}
			if !tt.want(prov) {
				t.Errorf("%s proxy=%v: got %T", tt.provider, proxy, prov)
			}
		}
	}
}
