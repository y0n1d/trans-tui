package ocr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testConfig(tokenURL, ocrURL string) Config {
	return Config{
		Provider:     "baidu",
		Model:        "general_basic",
		LanguageType: "CHN_ENG",
		Timeout:      30,
		Baidu: BaiduConfig{
			BaseURL:      tokenURL, // tokenURL and base_url share the same host for testing
			APIKeyEnv:    "TEST_BAIDU_API_KEY",
			SecretKeyEnv: "TEST_BAIDU_SECRET_KEY",
		},
	}
}

// testBaiduConfig creates a config pointing to the given mock servers.
func testBaiduConfig(baseURL string) Config {
	return Config{
		Provider:     "baidu",
		Model:        "general_basic",
		LanguageType: "CHN_ENG",
		Timeout:      30,
		Baidu: BaiduConfig{
			BaseURL:      baseURL,
			APIKeyEnv:    "TEST_BAIDU_API_KEY",
			SecretKeyEnv: "TEST_BAIDU_SECRET_KEY",
		},
	}
}

func TestNewProvider_Baidu(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{Provider: "baidu", Model: "general_basic", Timeout: 30, Baidu: BaiduConfig{BaseURL: "http://localhost", APIKeyEnv: "TEST_BAIDU_API_KEY", SecretKeyEnv: "TEST_BAIDU_SECRET_KEY"}}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, ok := p.(*baiduOCR); !ok {
		t.Error("expected *baiduOCR")
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	cfg := Config{Provider: "unknown"}
	_, err := NewProvider(cfg)
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported OCR provider") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewBaiduOCR_UnsupportedModel(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{Provider: "baidu", Model: "unknown_model", Baidu: BaiduConfig{BaseURL: "http://localhost", APIKeyEnv: "TEST_BAIDU_API_KEY", SecretKeyEnv: "TEST_BAIDU_SECRET_KEY"}}
	_, err := newBaiduOCR(cfg)
	if err == nil {
		t.Error("expected error for unsupported model")
	}
	if !strings.Contains(err.Error(), "unsupported OCR model") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewBaiduOCR_MissingEnv(t *testing.T) {
	os.Unsetenv("TEST_MISSING_KEY")
	os.Unsetenv("TEST_MISSING_SECRET")

	cfg := Config{Provider: "baidu", Model: "general_basic", Baidu: BaiduConfig{BaseURL: "http://localhost", APIKeyEnv: "TEST_MISSING_KEY", SecretKeyEnv: "TEST_MISSING_SECRET"}}
	_, err := newBaiduOCR(cfg)
	if err == nil {
		t.Error("expected error for missing env vars")
	}
}

func TestNewBaiduOCR_EmptyBaseURL(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{Provider: "baidu", Model: "general_basic", Baidu: BaiduConfig{BaseURL: "", APIKeyEnv: "TEST_BAIDU_API_KEY", SecretKeyEnv: "TEST_BAIDU_SECRET_KEY"}}
	_, err := newBaiduOCR(cfg)
	if err == nil {
		t.Error("expected error for empty base_url")
	}
}

func TestModelEndpointMapping(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"general_basic", "/rest/2.0/ocr/v1/general_basic"},
		{"accurate_basic", "/rest/2.0/ocr/v1/accurate_basic"},
		{"general", "/rest/2.0/ocr/v1/general"},
		{"accurate", "/rest/2.0/ocr/v1/accurate"},
	}
	for _, tt := range tests {
		endpoint, ok := baiduModelEndpoints[tt.model]
		if !ok {
			t.Errorf("model %q not found in mapping", tt.model)
			continue
		}
		if endpoint != tt.expected {
			t.Errorf("model %q: got %q, want %q", tt.model, endpoint, tt.expected)
		}
	}
}

func TestRecognize_Success(t *testing.T) {
	// Mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(baiduTokenResponse{AccessToken: "test_token", ExpiresIn: 2592000})
	}))
	defer tokenServer.Close()

	// Mock OCR server
	ocrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Query().Get("access_token") != "test_token" {
			t.Errorf("unexpected access token: %s", r.URL.Query().Get("access_token"))
		}
		if !strings.HasSuffix(r.URL.Path, "/general_basic") {
			t.Errorf("expected endpoint to end with /general_basic, got %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		lang := r.FormValue("language_type")
		if lang != "CHN_ENG" {
			t.Errorf("expected language_type CHN_ENG, got %s", lang)
		}
		json.NewEncoder(w).Encode(baiduOCRResponse{
			WordsResultNum: 2,
			WordsResult:    []baiduWordItem{{Words: "Hello"}, {Words: "World"}},
		})
	}))
	defer ocrServer.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	// Both token and OCR share the same base URL in this mock setup
	cfg := testBaiduConfig(ocrServer.URL)
	// Override token URL by creating a combined server
	combined := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/oauth/") {
			json.NewEncoder(w).Encode(baiduTokenResponse{AccessToken: "test_token", ExpiresIn: 2592000})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		json.NewEncoder(w).Encode(baiduOCRResponse{
			WordsResultNum: 2,
			WordsResult:    []baiduWordItem{{Words: "Hello"}, {Words: "World"}},
		})
	}))
	defer combined.Close()

	cfg.Baidu.BaseURL = combined.URL
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	text, err := p.Recognize(context.Background(), []byte("fake image"))
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if text != "Hello\nWorld" {
		t.Errorf("got %q, want %q", text, "Hello\nWorld")
	}
}

func TestRecognize_EmptyImage(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig("http://localhost")
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	_, err = p.Recognize(context.Background(), []byte{})
	if err == nil {
		t.Error("expected error for empty image")
	}
}

func TestRecognize_ImageTooLarge(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig("http://localhost")
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	// Baidu limit: base64 + URL encode <= 4MB.
	// base64: 3 bytes -> 4 chars, so ~3MB raw -> ~4MB base64.
	// Use 3.1MB raw to exceed 4MB after base64 + URL encode.
	rawSize := 3*1024*1024 + 100*1024 // ~3.1MB
	largeImage := make([]byte, rawSize)
	for i := range largeImage {
		largeImage[i] = byte(i % 256)
	}

	_, err = p.Recognize(context.Background(), largeImage)
	if err == nil {
		t.Error("expected error for oversized image")
	}
	if !strings.Contains(err.Error(), "image too large") {
		t.Errorf("expected size-related error, got: %v", err)
	}
}

func TestRecognize_NoNetworkForOversizedImage(t *testing.T) {
	// Server that fails if called
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called for oversized image")
	}))
	defer server.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig(server.URL)
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	// 3.1MB raw -> ~4.1MB base64+urlencode, exceeds 4MB limit
	rawSize := 3*1024*1024 + 100*1024
	largeImage := make([]byte, rawSize)
	for i := range largeImage {
		largeImage[i] = byte(i % 256)
	}

	_, err = p.Recognize(context.Background(), largeImage)
	if err == nil {
		t.Error("expected error for oversized image")
	}
}

func TestRecognize_TokenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig(server.URL)
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	_, err = p.Recognize(context.Background(), []byte("image"))
	if err == nil {
		t.Error("expected error for token failure")
	}
}

func TestRecognize_OCRError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/oauth/") {
			json.NewEncoder(w).Encode(baiduTokenResponse{AccessToken: "tok", ExpiresIn: 2592000})
			return
		}
		json.NewEncoder(w).Encode(baiduOCRResponse{ErrorCode: 216201, ErrorMessage: "image format error"})
	}))
	defer server.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig(server.URL)
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	_, err = p.Recognize(context.Background(), []byte("image"))
	if err == nil {
		t.Error("expected error for OCR failure")
	}
}

func TestRecognize_EmptyWordsResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/oauth/") {
			json.NewEncoder(w).Encode(baiduTokenResponse{AccessToken: "tok", ExpiresIn: 2592000})
			return
		}
		json.NewEncoder(w).Encode(baiduOCRResponse{WordsResultNum: 0, WordsResult: []baiduWordItem{}})
	}))
	defer server.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig(server.URL)
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	_, err = p.Recognize(context.Background(), []byte("image"))
	if err == nil {
		t.Error("expected error for empty result")
	}
}

func TestRecognize_CustomLanguageType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/oauth/") {
			json.NewEncoder(w).Encode(baiduTokenResponse{AccessToken: "tok", ExpiresIn: 2592000})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		lang := r.FormValue("language_type")
		if lang != "ENG" {
			t.Errorf("expected language_type ENG, got %s", lang)
		}
		json.NewEncoder(w).Encode(baiduOCRResponse{
			WordsResultNum: 1,
			WordsResult:    []baiduWordItem{{Words: "Hello"}},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig(server.URL)
	cfg.LanguageType = "ENG"
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	text, err := p.Recognize(context.Background(), []byte("image"))
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if text != "Hello" {
		t.Errorf("got %q, want %q", text, "Hello")
	}
}

func TestRecognize_AccurateBasicEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/oauth/") {
			json.NewEncoder(w).Encode(baiduTokenResponse{AccessToken: "tok", ExpiresIn: 2592000})
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/accurate_basic") {
			t.Errorf("expected /accurate_basic endpoint, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(baiduOCRResponse{
			WordsResultNum: 1,
			WordsResult:    []baiduWordItem{{Words: "High precision"}},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig(server.URL)
	cfg.Model = "accurate_basic"
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	text, err := p.Recognize(context.Background(), []byte("image"))
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if text != "High precision" {
		t.Errorf("got %q, want %q", text, "High precision")
	}
}

func TestRecognize_BaseURLTrailingSlash(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{
		Provider:     "baidu",
		Model:        "general_basic",
		LanguageType: "CHN_ENG",
		Timeout:      30,
		Baidu: BaiduConfig{
			BaseURL:      "http://localhost/",
			APIKeyEnv:    "TEST_BAIDU_API_KEY",
			SecretKeyEnv: "TEST_BAIDU_SECRET_KEY",
		},
	}

	b, err := newBaiduOCR(cfg)
	if err != nil {
		t.Fatalf("newBaiduOCR: %v", err)
	}

	// Verify trailing slash is trimmed
	if strings.HasSuffix(b.baseURL, "/") {
		t.Errorf("baseURL should not have trailing slash: %s", b.baseURL)
	}
}

func TestNewBaiduOCR_NegativeTimeout(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{
		Provider: "baidu",
		Model:    "general_basic",
		Timeout:  -1,
		Baidu:    BaiduConfig{BaseURL: "http://localhost", APIKeyEnv: "TEST_BAIDU_API_KEY", SecretKeyEnv: "TEST_BAIDU_SECRET_KEY"},
	}
	_, err := newBaiduOCR(cfg)
	if err == nil {
		t.Error("expected error for negative timeout")
	}
	if !strings.Contains(err.Error(), "invalid OCR timeout") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewBaiduOCR_ZeroTimeout(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{
		Provider: "baidu",
		Model:    "general_basic",
		Timeout:  0,
		Baidu:    BaiduConfig{BaseURL: "http://localhost", APIKeyEnv: "TEST_BAIDU_API_KEY", SecretKeyEnv: "TEST_BAIDU_SECRET_KEY"},
	}
	b, err := newBaiduOCR(cfg)
	if err != nil {
		t.Fatalf("unexpected error for zero timeout: %v", err)
	}
	// Zero timeout should default to 30s
	if b.httpClient.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", b.httpClient.Timeout)
	}
}

func TestNewBaiduOCR_PositiveTimeout(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := Config{
		Provider: "baidu",
		Model:    "general_basic",
		Timeout:  60,
		Baidu:    BaiduConfig{BaseURL: "http://localhost", APIKeyEnv: "TEST_BAIDU_API_KEY", SecretKeyEnv: "TEST_BAIDU_SECRET_KEY"},
	}
	b, err := newBaiduOCR(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.httpClient.Timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", b.httpClient.Timeout)
	}
}

func TestRecognize_SizeCheckUsesURLEncoding(t *testing.T) {
	os.Setenv("TEST_BAIDU_API_KEY", "key")
	os.Setenv("TEST_BAIDU_SECRET_KEY", "secret")
	defer os.Unsetenv("TEST_BAIDU_API_KEY")
	defer os.Unsetenv("TEST_BAIDU_SECRET_KEY")

	cfg := testBaiduConfig("http://localhost")
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	// Create an image whose base64 is under 4MB but URL-encoded exceeds 4MB.
	// Base64 uses A-Z, a-z, 0-9, +, /, = — only +, /, = get URL-encoded.
	// Fill with bytes that produce many / and + in base64 to maximize URL encoding overhead.
	rawSize := 3*1024*1024 + 50*1024 // ~3.05MB raw
	largeImage := make([]byte, rawSize)
	for i := range largeImage {
		largeImage[i] = 0xFF // all 0xFF produces lots of / in base64
	}

	_, err = p.Recognize(context.Background(), largeImage)
	if err == nil {
		t.Error("expected error for oversized image")
	}
	if !strings.Contains(err.Error(), "image too large") {
		t.Errorf("expected size-related error, got: %v", err)
	}
}
