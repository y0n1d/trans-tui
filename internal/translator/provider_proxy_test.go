package translator

import (
	"net/http"
	"reflect"
	"testing"
)

// All four translator providers must apply the proxy setting to the client
// they actually use for requests.
func TestProvidersApplyProxySetting(t *testing.T) {
	tests := []struct {
		provider string
		client   func(proxy bool) *http.Client
	}{
		{
			provider: "openai-compatible",
			client: func(proxy bool) *http.Client {
				return NewOpenAICompatibleProvider(OpenAICompatibleConfig{
					BaseURL:    "https://api.openai.com/v1",
					Model:      "gpt-4o-mini",
					APIKeyEnv:  "TEST_KEY",
					TimeoutSec: 5,
					Proxy:      proxy,
				}).client
			},
		},
		{
			provider: "google",
			client:   func(proxy bool) *http.Client { return NewGoogleProvider("", "TEST_KEY", 5, proxy).client },
		},
		{
			provider: "deepl",
			client:   func(proxy bool) *http.Client { return NewDeepLProvider("", "TEST_KEY", 5, proxy).client },
		},
		{
			provider: "libretranslate",
			client:   func(proxy bool) *http.Client { return NewLibreTranslateProvider("", "TEST_KEY", 5, proxy).client },
		},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			enabled := transportOf(t, tt.client(true))
			if enabled.Proxy == nil {
				t.Errorf("proxy=true: Transport.Proxy = nil, want http.ProxyFromEnvironment")
			} else if got, want := reflect.ValueOf(enabled.Proxy).Pointer(), reflect.ValueOf(http.ProxyFromEnvironment).Pointer(); got != want {
				t.Errorf("proxy=true: Transport.Proxy = %#x, want http.ProxyFromEnvironment (%#x)", got, want)
			}

			disabled := transportOf(t, tt.client(false))
			if disabled.Proxy != nil {
				t.Errorf("proxy=false: Transport.Proxy = %#x, want nil", reflect.ValueOf(disabled.Proxy).Pointer())
			}
		})
	}
}

// proxy=false must stay direct for loopback targets too — the Go standard
// library never proxies localhost/loopback, and this feature must not
// change that (LibreTranslate's default base_url is http://localhost:5000).
func TestProvidersKeepLoopbackDirect(t *testing.T) {
	for _, proxy := range []bool{true, false} {
		tr := transportOf(t, NewLibreTranslateProvider("http://localhost:5000", "TEST_KEY", 5, proxy).client)
		if tr.Proxy == nil {
			// proxy=false: no proxy func at all — direct by construction.
			continue
		}
		req, err := http.NewRequest(http.MethodGet, "http://localhost:5000/translate", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		u, err := tr.Proxy(req)
		if err != nil {
			t.Fatalf("Proxy: %v", err)
		}
		if u != nil {
			t.Errorf("proxy=%v: loopback destination resolved to proxy %v, want direct", proxy, u)
		}
	}
}
