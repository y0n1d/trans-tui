package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type LibreTranslateConfig struct {
	BaseURL   string `toml:"base_url"`
	APIKeyEnv string `toml:"api_key_env"`
}

type LibreTranslateProvider struct {
	baseURL   string
	apiKeyEnv string
	client    *http.Client
}

func NewLibreTranslateProvider(baseURL, apiKeyEnv string, timeoutSec int) *LibreTranslateProvider {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if baseURL == "" {
		baseURL = "http://localhost:5000"
	}
	return &LibreTranslateProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKeyEnv: apiKeyEnv,
		client:    &http.Client{Timeout: timeout},
	}
}

type libreRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Q      string `json:"q"`
	APIKey string `json:"api_key,omitempty"`
}

type libreResponse struct {
	TranslatedText string `json:"translatedText"`
}

type libreErrorResponse struct {
	Error string `json:"error"`
}

func (p *LibreTranslateProvider) Translate(ctx context.Context, req TranslationRequest) (TranslationResult, error) {
	sourceLang := req.SourceLang
	if sourceLang == "auto" {
		sourceLang = "auto"
	}

	body := libreRequest{
		Source: sourceLang,
		Target: req.TargetLang,
		Q:      req.Text,
	}

	if p.apiKeyEnv != "" {
		apiKey := os.Getenv(p.apiKeyEnv)
		if apiKey != "" {
			body.APIKey = apiKey
		}
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := p.baseURL + "/translate"
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
		var errResp libreErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return TranslationResult{}, fmt.Errorf("LibreTranslate error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return TranslationResult{}, fmt.Errorf("LibreTranslate returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var libreResp libreResponse
	if err := json.Unmarshal(respBody, &libreResp); err != nil {
		return TranslationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	translated := strings.TrimSpace(libreResp.TranslatedText)
	if translated == "" {
		return TranslationResult{}, fmt.Errorf("API returned empty translation")
	}

	return TranslationResult{
		Translation: translated,
		Provider:    "libretranslate",
	}, nil
}
