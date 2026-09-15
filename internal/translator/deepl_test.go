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

func TestDeepL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/translate" {
			t.Errorf("expected /v2/translate, got %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "DeepL-Auth-Key test-key-123" {
			t.Errorf("expected 'DeepL-Auth-Key test-key-123', got '%s'", auth)
		}

		var reqBody deeplRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(reqBody.Text) != 1 || reqBody.Text[0] != "Hello world" {
			t.Errorf("expected text ['Hello world'], got %v", reqBody.Text)
		}
		if reqBody.TargetLang != "ZH" {
			t.Errorf("expected target_lang ZH, got %s", reqBody.TargetLang)
		}
		if reqBody.SourceLang != "EN" {
			t.Errorf("expected source_lang EN, got %s", reqBody.SourceLang)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []deeplTranslation{
				{Text: "你好世界", DetectedSourceLanguage: "EN"},
			},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key-123")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	result, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello world",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "你好世界" {
		t.Errorf("expected '你好世界', got '%s'", result.Translation)
	}
	if result.Provider != "deepl" {
		t.Errorf("expected provider 'deepl', got '%s'", result.Provider)
	}
}

func TestDeepL_AutoSource(t *testing.T) {
	var receivedReq deeplRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []deeplTranslation{
				{Text: "Hallo Welt", DetectedSourceLanguage: "EN"},
			},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello world",
		SourceLang: "auto",
		TargetLang: "de",
	})

	if receivedReq.SourceLang != "" {
		t.Errorf("expected empty source_lang for auto, got '%s'", receivedReq.SourceLang)
	}
}

func TestDeepL_SpecificSource(t *testing.T) {
	var receivedReq deeplRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []deeplTranslation{
				{Text: "Hallo Welt"},
			},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello world",
		SourceLang: "en",
		TargetLang: "de",
	})

	if receivedReq.SourceLang != "EN" {
		t.Errorf("expected source_lang 'EN', got '%s'", receivedReq.SourceLang)
	}
}

func TestDeepL_HTTP4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(deeplErrorResponse{
			Message: "Authorization failed",
			Code:    "AUTHORIZATION_FAILED",
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "bad-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !containsSubstr(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got: %s", err.Error())
	}
	if !containsSubstr(err.Error(), "Authorization failed") {
		t.Errorf("expected 'Authorization failed' in error, got: %s", err.Error())
	}
}

func TestDeepL_HTTP5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(deeplErrorResponse{
			Message: "Internal server error",
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !containsSubstr(err.Error(), "500") {
		t.Errorf("expected status 500 in error, got: %s", err.Error())
	}
}

func TestDeepL_APIErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(deeplErrorResponse{
			Message: "Invalid target_lang: XYZ",
			Code:    "INVALID_TARGET_LANGUAGE",
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "xyz",
	})

	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !containsSubstr(err.Error(), "Invalid target_lang") {
		t.Errorf("expected 'Invalid target_lang' in error, got: %s", err.Error())
	}
}

func TestDeepL_InfraErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(deeplInfraErrorResponse{
			Error: struct {
				Message string `json:"message"`
			}{Message: "Bad Gateway"},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for infra error response")
	}
	if !containsSubstr(err.Error(), "Bad Gateway") {
		t.Errorf("expected 'Bad Gateway' in error, got: %s", err.Error())
	}
}

func TestDeepL_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for malformed response")
	}
	if !containsSubstr(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode' in error, got: %s", err.Error())
	}
}

func TestDeepL_EmptyTranslation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []deeplTranslation{
				{Text: "   "},
			},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for empty translation after trim")
	}
	if !containsSubstr(err.Error(), "empty translation") {
		t.Errorf("expected 'empty translation' in error, got: %s", err.Error())
	}
}

func TestDeepL_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 10 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Translate(ctx, TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if ctx.Err() == nil {
		t.Error("expected context error")
	}
}

func TestDeepL_NetworkError(t *testing.T) {
	provider := &DeepLProvider{
		baseURL:   "http://localhost:99999",
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 2 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestDeepL_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 1 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for timeout")
	}
}

func TestDeepL_APIKeyNotSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Unsetenv("NONEXISTENT_DEEPL_KEY")

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "NONEXISTENT_DEEPL_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !containsSubstr(err.Error(), "API key not set") {
		t.Errorf("expected 'API key not set' in error, got: %s", err.Error())
	}
	if !containsSubstr(err.Error(), "NONEXISTENT_DEEPL_KEY") {
		t.Errorf("expected env var name in error, got: %s", err.Error())
	}
}

func TestDeepL_CustomBaseURL(t *testing.T) {
	provider := NewDeepLProvider("http://my-deepl-proxy:8080/", "MY_KEY", 30)
	if provider.baseURL != "http://my-deepl-proxy:8080" {
		t.Errorf("expected 'http://my-deepl-proxy:8080', got '%s'", provider.baseURL)
	}
}

func TestDeepL_TargetLangUpperCase(t *testing.T) {
	var receivedReq deeplRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []deeplTranslation{
				{Text: "translated"},
			},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "en-us",
	})

	if receivedReq.TargetLang != "EN-US" {
		t.Errorf("expected target_lang 'EN-US', got '%s'", receivedReq.TargetLang)
	}
}

func TestDeepL_NoTranslationsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deeplResponse{
			Translations: []deeplTranslation{},
		})
	}))
	defer server.Close()

	provider := &DeepLProvider{
		baseURL:   server.URL,
		apiKeyEnv: "DEEPL_TEST_KEY",
		client:    &http.Client{Timeout: 5 * time.Second},
	}
	os.Setenv("DEEPL_TEST_KEY", "test-key")
	defer os.Unsetenv("DEEPL_TEST_KEY")

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "de",
	})

	if err == nil {
		t.Fatal("expected error for no translations")
	}
	if !containsSubstr(err.Error(), "no translations") {
		t.Errorf("expected 'no translations' in error, got: %s", err.Error())
	}
}
