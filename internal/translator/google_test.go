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

func TestGoogle_Translate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Query().Get("key") != "test-google-key" {
			t.Errorf("expected key=test-google-key, got %s", r.URL.Query().Get("key"))
		}

		var reqBody googleRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if reqBody.Target != "zh" {
			t.Errorf("expected target zh, got %s", reqBody.Target)
		}
		if reqBody.Q != "Hello world" {
			t.Errorf("expected q='Hello world', got %s", reqBody.Q)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(googleResponse{
			Data: struct {
				Translations []googleTranslation `json:"translations"`
			}{
				Translations: []googleTranslation{
					{TranslatedText: "你好，世界"},
				},
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_KEY", "test-google-key")
	defer os.Unsetenv("TEST_GOOGLE_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_KEY", 5, true)

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
	if result.Provider != "google" {
		t.Errorf("expected provider 'google', got '%s'", result.Provider)
	}
}

func TestGoogle_Translate_AutoDetect(t *testing.T) {
	var receivedReq googleRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(googleResponse{
			Data: struct {
				Translations []googleTranslation `json:"translations"`
			}{
				Translations: []googleTranslation{
					{TranslatedText: "translated", DetectedSourceLanguage: "fr"},
				},
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_KEY2", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_KEY2")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_KEY2", 5, true)

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Bonjour",
		SourceLang: "auto",
		TargetLang: "en",
	})

	if receivedReq.Source != "" {
		t.Errorf("auto source should be omitted, got source='%s'", receivedReq.Source)
	}
}

func TestGoogle_Translate_MissingAPIKey(t *testing.T) {
	os.Unsetenv("MISSING_GOOGLE_KEY")

	provider := NewGoogleProvider("", "MISSING_GOOGLE_KEY", 5, true)

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !containsSubstr(err.Error(), "API key not set") {
		t.Errorf("expected 'API key not set' in error, got: %s", err.Error())
	}
}

func TestGoogle_Translate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(googleErrorResponse{
			Error: struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			}{
				Code:    403,
				Message: "Google API key not valid",
				Status:  "PERMISSION_DENIED",
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_ERR_KEY", "bad-key")
	defer os.Unsetenv("TEST_GOOGLE_ERR_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_ERR_KEY", 5, true)

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !containsSubstr(err.Error(), "403") {
		t.Errorf("expected status 403 in error, got: %s", err.Error())
	}
}

func TestGoogle_Translate_EmptyTranslations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(googleResponse{
			Data: struct {
				Translations []googleTranslation `json:"translations"`
			}{
				Translations: []googleTranslation{},
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_EMPTY_KEY", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_EMPTY_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_EMPTY_KEY", 5, true)

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for empty translations")
	}
	if !containsSubstr(err.Error(), "no translations") {
		t.Errorf("expected 'no translations' in error, got: %s", err.Error())
	}
}

func TestGoogle_Translate_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(googleResponse{
			Data: struct {
				Translations []googleTranslation `json:"translations"`
			}{
				Translations: []googleTranslation{
					{TranslatedText: "   "},
				},
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_WS_KEY", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_WS_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_WS_KEY", 5, true)

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

func TestGoogle_Translate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_CANCEL_KEY", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_CANCEL_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_CANCEL_KEY", 10, true)

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

func TestGoogle_Translate_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"invalid json`))
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_MAL_KEY", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_MAL_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_MAL_KEY", 5, true)

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

func TestGoogle_Translate_NetworkError(t *testing.T) {
	os.Setenv("TEST_GOOGLE_NET_KEY", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_NET_KEY")

	provider := NewGoogleProvider("http://localhost:99999", "TEST_GOOGLE_NET_KEY", 2, true)

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestGoogle_Translate_SourceSpecificLang(t *testing.T) {
	var receivedReq googleRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(googleResponse{
			Data: struct {
				Translations []googleTranslation `json:"translations"`
			}{
				Translations: []googleTranslation{
					{TranslatedText: "translated"},
				},
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_GOOGLE_SRC_KEY", "test-key")
	defer os.Unsetenv("TEST_GOOGLE_SRC_KEY")

	provider := NewGoogleProvider(server.URL, "TEST_GOOGLE_SRC_KEY", 5, true)

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Bonjour",
		SourceLang: "fr",
		TargetLang: "en",
	})

	if receivedReq.Source != "fr" {
		t.Errorf("expected source='fr', got '%s'", receivedReq.Source)
	}
	if receivedReq.Target != "en" {
		t.Errorf("expected target='en', got '%s'", receivedReq.Target)
	}
}
