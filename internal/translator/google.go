package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type GoogleConfig struct {
	APIKeyEnv string `toml:"api_key_env"`
}

type GoogleProvider struct {
	baseURL   string
	apiKeyEnv string
	client    *http.Client
}

const googleAPIURL = "https://translation.googleapis.com/language/translate/v2"

func NewGoogleProvider(baseURL, apiKeyEnv string, timeoutSec int) *GoogleProvider {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if baseURL == "" {
		baseURL = googleAPIURL
	}
	return &GoogleProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKeyEnv: apiKeyEnv,
		client:    &http.Client{Timeout: timeout},
	}
}

type googleRequest struct {
	Q      string `json:"q"`
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
	Format string `json:"format"`
}

type googleTranslation struct {
	TranslatedText         string `json:"translatedText"`
	DetectedSourceLanguage string `json:"detectedSourceLanguage,omitempty"`
}

type googleResponse struct {
	Data struct {
		Translations []googleTranslation `json:"translations"`
	} `json:"data"`
}

type googleErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (p *GoogleProvider) Translate(ctx context.Context, req TranslationRequest) (TranslationResult, error) {
	apiKey := os.Getenv(p.apiKeyEnv)
	if apiKey == "" {
		return TranslationResult{}, fmt.Errorf("API key not set: environment variable %s is empty or missing", p.apiKeyEnv)
	}

	sourceLang := req.SourceLang
	if sourceLang == "auto" {
		sourceLang = ""
	}

	params := url.Values{}
	params.Set("key", apiKey)

	body := googleRequest{
		Q:      req.Text,
		Target: req.TargetLang,
		Format: "text",
	}
	if sourceLang != "" {
		body.Source = sourceLang
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := p.baseURL + "?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return TranslationResult{}, ctx.Err()
		}
		return TranslationResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp googleErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return TranslationResult{}, fmt.Errorf("Google API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return TranslationResult{}, fmt.Errorf("Google API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var googleResp googleResponse
	if err := json.Unmarshal(respBody, &googleResp); err != nil {
		return TranslationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(googleResp.Data.Translations) == 0 {
		return TranslationResult{}, fmt.Errorf("API returned no translations")
	}

	translated := strings.TrimSpace(googleResp.Data.Translations[0].TranslatedText)
	if translated == "" {
		return TranslationResult{}, fmt.Errorf("API returned empty translation")
	}

	return TranslationResult{
		Translation: translated,
		Provider:    "google",
	}, nil
}
