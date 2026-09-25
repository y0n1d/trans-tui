package translator

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

// transportOf returns the concrete transport of a factory-built client.
func transportOf(t *testing.T, c *http.Client) *http.Transport {
	t.Helper()
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport is %T, want *http.Transport", c.Transport)
	}
	return tr
}

// proxy=true must keep the Go environment proxy mechanism, i.e. the transport
// must carry http.ProxyFromEnvironment — exactly what http.DefaultTransport
// uses, so HTTP_PROXY / HTTPS_PROXY / NO_PROXY keep working unchanged.
func TestNewHTTPClientProxyTrueUsesEnvironmentProxy(t *testing.T) {
	tr := transportOf(t, newHTTPClient(5, true))
	if tr.Proxy == nil {
		t.Fatal("proxy=true: Transport.Proxy is nil, want http.ProxyFromEnvironment")
	}
	got := reflect.ValueOf(tr.Proxy).Pointer()
	want := reflect.ValueOf(http.ProxyFromEnvironment).Pointer()
	if got != want {
		t.Errorf("proxy=true: Transport.Proxy = %#x, want http.ProxyFromEnvironment (%#x)", got, want)
	}
}

// proxy=false must clear the Proxy func, which makes the standard library
// dial directly and never consult HTTP_PROXY/HTTPS_PROXY.
func TestNewHTTPClientProxyFalseForcesDirect(t *testing.T) {
	tr := transportOf(t, newHTTPClient(5, false))
	if tr.Proxy != nil {
		t.Fatalf("proxy=false: Transport.Proxy = %#x, want nil", reflect.ValueOf(tr.Proxy).Pointer())
	}
}

// The factory must clone, never mutate, the process-wide defaults.
func TestNewHTTPClientDoesNotModifyGlobals(t *testing.T) {
	def, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatalf("http.DefaultTransport is %T, want *http.Transport", http.DefaultTransport)
	}
	before := reflect.ValueOf(def.Proxy).Pointer()

	client := newHTTPClient(5, false)

	if reflect.ValueOf(def.Proxy).Pointer() != before {
		t.Errorf("http.DefaultTransport.Proxy was modified")
	}
	if http.DefaultClient.Transport != nil {
		t.Errorf("http.DefaultClient.Transport was modified: %v", http.DefaultClient.Transport)
	}
	if client.Transport == http.DefaultTransport {
		t.Errorf("factory returned the global DefaultTransport instead of a clone")
	}
	if transportOf(t, newHTTPClient(5, true)).Proxy == nil {
		t.Errorf("global DefaultTransport lost its Proxy func")
	}
}

func TestNewHTTPClientTimeout(t *testing.T) {
	if got := newHTTPClient(0, true).Timeout; got != 30*time.Second {
		t.Errorf("timeoutSec=0 → %v, want 30s", got)
	}
	if got := newHTTPClient(7, true).Timeout; got != 7*time.Second {
		t.Errorf("timeoutSec=7 → %v, want 7s", got)
	}
}

// Deterministic end-to-end check of the two modes, using local servers only
// and an explicitly injected proxy function — no environment variables and
// no external network.
//
// The control case proves the very same transport honours a non-nil Proxy
// func (request reaches the proxy server); the factory's proxy=false client
// has Proxy == nil, so no proxy is ever consulted and the target answers
// directly.
func TestNewHTTPClientProxyFalseNeverConsultsProxy(t *testing.T) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits++
		w.Write([]byte("target"))
	}))
	defer target.Close()

	proxyHits := 0
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		w.Write([]byte("proxy"))
	}))
	defer proxySrv.Close()

	proxyURL, err := url.Parse(proxySrv.URL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}

	get := func(c *http.Client) string {
		t.Helper()
		resp, err := c.Get(target.URL)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		body := make([]byte, 16)
		n, _ := resp.Body.Read(body)
		return string(body[:n])
	}

	// Control: same factory transport, but with an injected proxy function.
	control := newHTTPClient(5, false)
	control.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	if got := get(control); got != "proxy" {
		t.Errorf("control request went to %q, want %q — transport does not honour Proxy", got, "proxy")
	}
	if proxyHits != 1 {
		t.Errorf("proxy hits = %d, want 1", proxyHits)
	}

	// Factory result: proxy=false → Proxy == nil → direct, even though a
	// proxy function exists for the identical transport shape.
	direct := newHTTPClient(5, false)
	if got := get(direct); got != "target" {
		t.Errorf("proxy=false request went to %q, want %q", got, "target")
	}
	if proxyHits != 1 {
		t.Errorf("proxy=false consulted the proxy: proxy hits = %d, want 1", proxyHits)
	}
	if targetHits != 1 {
		t.Errorf("target hits = %d, want 1", targetHits)
	}
}
