package translator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestLibreTranslate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/translate" {
			t.Errorf("expected /translate, got %s", r.URL.Path)
		}

		var reqBody libreRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if reqBody.Source != "en" {
			t.Errorf("expected source en, got %s", reqBody.Source)
		}
		if reqBody.Target != "zh" {
			t.Errorf("expected target zh, got %s", reqBody.Target)
		}
		if reqBody.Q != "Hello world" {
			t.Errorf("expected q 'Hello world', got %s", reqBody.Q)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libreResponse{
			TranslatedText: "你好，世界",
		})
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	result, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello world",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "你好，世界" {
		t.Errorf("expected '你好，世界', got '%s'", result.Translation)
	}
	if result.Provider != "libretranslate" {
		t.Errorf("expected provider 'libretranslate', got '%s'", result.Provider)
	}
}

func TestLibreTranslate_WithAPIKey(t *testing.T) {
	var receivedReq libreRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libreResponse{
			TranslatedText: "translated",
		})
	}))
	defer server.Close()

	os.Setenv("LIBRE_API_KEY", "my-secret-key")
	defer os.Unsetenv("LIBRE_API_KEY")

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "LIBRE_API_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if receivedReq.APIKey != "my-secret-key" {
		t.Errorf("expected api_key='my-secret-key', got '%s'", receivedReq.APIKey)
	}
}

func TestLibreTranslate_NoAPIKeyOptional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody libreRequest
		json.NewDecoder(r.Body).Decode(&reqBody)

		if reqBody.APIKey != "" {
			t.Errorf("expected empty api_key, got '%s'", reqBody.APIKey)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libreResponse{
			TranslatedText: "translated",
		})
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	result, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "translated" {
		t.Errorf("expected 'translated', got '%s'", result.Translation)
	}
}

func TestLibreTranslate_AutoDetect(t *testing.T) {
	var receivedReq libreRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libreResponse{
			TranslatedText: "translated",
		})
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Bonjour",
		SourceLang: "auto",
		TargetLang: "en",
	})

	if receivedReq.Source != "auto" {
		t.Errorf("expected source='auto', got '%s'", receivedReq.Source)
	}
}

func TestLibreTranslate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(libreErrorResponse{
			Error: "Invalid format. Expected 'q' parameter",
		})
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !containsSubstr(err.Error(), "400") {
		t.Errorf("expected status 400 in error, got: %s", err.Error())
	}
	if !containsSubstr(err.Error(), "Invalid format") {
		t.Errorf("expected 'Invalid format' in error, got: %s", err.Error())
	}
}

func TestLibreTranslate_EmptyTranslation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libreResponse{
			TranslatedText: "   ",
		})
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for empty translation after trim")
	}
	if !containsSubstr(err.Error(), "empty translation") {
		t.Errorf("expected 'empty translation' in error, got: %s", err.Error())
	}
}

func TestLibreTranslate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Translate(ctx, TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if ctx.Err() == nil {
		t.Error("expected context error")
	}
}

func TestLibreTranslate_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for malformed response")
	}
	if !containsSubstr(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode' in error, got: %s", err.Error())
	}
}

func TestLibreTranslate_NetworkError(t *testing.T) {
	provider := &LibreTranslateProvider{
		baseURL:   "http://localhost:99999",
		apiKeyEnv: "",
		client:    &http.Client{Timeout: 2 * time.Second},
	}

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestLibreTranslate_DefaultBaseURL(t *testing.T) {
	provider := NewLibreTranslateProvider("", "", 30)
	if provider.baseURL != "http://localhost:5000" {
		t.Errorf("expected default base URL 'http://localhost:5000', got '%s'", provider.baseURL)
	}
}

func TestLibreTranslate_CustomBaseURL(t *testing.T) {
	provider := NewLibreTranslateProvider("http://my-server:8080/", "", 30)
	if provider.baseURL != "http://my-server:8080" {
		t.Errorf("expected 'http://my-server:8080', got '%s'", provider.baseURL)
	}
}

func TestLibreTranslate_APIKeyEnvNotSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody libreRequest
		json.NewDecoder(r.Body).Decode(&reqBody)

		if reqBody.APIKey != "" {
			t.Errorf("expected empty api_key when env not set, got '%s'", reqBody.APIKey)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(libreResponse{
			TranslatedText: "ok",
		})
	}))
	defer server.Close()

	os.Unsetenv("NONEXISTENT_KEY")

	provider := &LibreTranslateProvider{
		baseURL:   server.URL,
		apiKeyEnv: "NONEXISTENT_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	result, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "ok" {
		t.Errorf("expected 'ok', got '%s'", result.Translation)
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
