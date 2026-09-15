package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestTranslate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
		}

		var reqBody openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if reqBody.Model != "gpt-4o-mini" {
			t.Errorf("expected model gpt-4o-mini, got %s", reqBody.Model)
		}
		if len(reqBody.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(reqBody.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Role: "assistant", Content: "你好，世界"}},
			},
		})
	}))
	defer server.Close()

	os.Setenv("TEST_TRANSLATE_KEY", "test-api-key")
	defer os.Unsetenv("TEST_TRANSLATE_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "TEST_TRANSLATE_KEY",
		TimeoutSec: 5,
	})

	result, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello world",
		SourceLang: "en",
		TargetLang: "zh",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Translation != "你好，世界" {
		t.Errorf("expected translation '你好，世界', got '%s'", result.Translation)
	}
	if result.Provider != "openai-compatible" {
		t.Errorf("expected provider 'openai-compatible', got '%s'", result.Provider)
	}
	if result.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got '%s'", result.Model)
	}
}

func TestTranslate_MissingAPIKey(t *testing.T) {
	os.Unsetenv("MISSING_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    "http://localhost",
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "MISSING_KEY",
		TimeoutSec: 5,
	})

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !contains(err.Error(), "API key not set") {
		t.Errorf("expected error about missing API key, got: %s", err.Error())
	}
}

func TestTranslate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("CANCEL_TEST_KEY", "test-key")
	defer os.Unsetenv("CANCEL_TEST_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "CANCEL_TEST_KEY",
		TimeoutSec: 10,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

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

func TestTranslate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	os.Setenv("ERROR_TEST_KEY", "bad-key")
	defer os.Unsetenv("ERROR_TEST_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "ERROR_TEST_KEY",
		TimeoutSec: 5,
	})

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !contains(err.Error(), "status 401") {
		t.Errorf("expected status 401 in error, got: %s", err.Error())
	}
}

func TestTranslate_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []openaiChoice{},
		})
	}))
	defer server.Close()

	os.Setenv("EMPTY_TEST_KEY", "test-key")
	defer os.Unsetenv("EMPTY_TEST_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "EMPTY_TEST_KEY",
		TimeoutSec: 5,
	})

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !contains(err.Error(), "no choices") {
		t.Errorf("expected 'no choices' in error, got: %s", err.Error())
	}
}

func TestTranslate_EmptyTranslation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Role: "assistant", Content: "   "}},
			},
		})
	}))
	defer server.Close()

	os.Setenv("WHITESPACE_TEST_KEY", "test-key")
	defer os.Unsetenv("WHITESPACE_TEST_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "WHITESPACE_TEST_KEY",
		TimeoutSec: 5,
	})

	_, err := provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		TargetLang: "zh",
	})

	if err == nil {
		t.Fatal("expected error for empty translation after trim")
	}
	if !contains(err.Error(), "empty translation") {
		t.Errorf("expected 'empty translation' in error, got: %s", err.Error())
	}
}

func TestTranslate_SystemPrompt_AutoSourceLang(t *testing.T) {
	var receivedReq openaiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Role: "assistant", Content: "translated"}},
			},
		})
	}))
	defer server.Close()

	os.Setenv("PROMPT_TEST_KEY", "test-key")
	defer os.Unsetenv("PROMPT_TEST_KEY")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "PROMPT_TEST_KEY",
		TimeoutSec: 5,
	})

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Hello",
		SourceLang: "auto",
		TargetLang: "ja",
	})

	systemMsg := receivedReq.Messages[0].Content
	if !contains(systemMsg, "Translate to ja") {
		t.Errorf("expected system prompt to contain target lang, got: %s", systemMsg)
	}
	if contains(systemMsg, "source language is auto") {
		t.Errorf("system prompt should not mention auto as source language")
	}
}

func TestTranslate_SystemPrompt_SpecificSourceLang(t *testing.T) {
	var receivedReq openaiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			Choices: []openaiChoice{
				{Message: openaiMessage{Role: "assistant", Content: "translated"}},
			},
		})
	}))
	defer server.Close()

	os.Setenv("PROMPT_TEST_KEY2", "test-key")
	defer os.Unsetenv("PROMPT_TEST_KEY2")

	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL,
		Model:      "gpt-4o-mini",
		APIKeyEnv:  "PROMPT_TEST_KEY2",
		TimeoutSec: 5,
	})

	provider.Translate(context.Background(), TranslationRequest{
		Text:       "Bonjour",
		SourceLang: "fr",
		TargetLang: "en",
	})

	systemMsg := receivedReq.Messages[0].Content
	if !contains(systemMsg, "source language is fr") {
		t.Errorf("expected system prompt to mention source lang, got: %s", systemMsg)
	}
	if !contains(systemMsg, "Translate to en") {
		t.Errorf("expected system prompt to contain target lang, got: %s", systemMsg)
	}
}

func TestTranslate_DefaultTimeout(t *testing.T) {
	provider := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:   "http://localhost",
		Model:     "gpt-4o-mini",
		APIKeyEnv: "KEY",
	})

	if provider.client.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", provider.client.Timeout)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && fmt.Sprintf("%s", s) != "" && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
