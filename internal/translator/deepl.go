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
)

type DeepLConfig struct {
	BaseURL string `toml:"base_url"`
}

type DeepLProvider struct {
	baseURL   string
	apiKeyEnv string
	client    *http.Client
}

const deeplAPIURL = "https://api-free.deepl.com"

// NewDeepLProvider creates a DeepL translator. proxy=false forces a direct
// connection (HTTP_PROXY/HTTPS_PROXY are ignored); proxy=true keeps the Go
// environment proxy mechanism.
func NewDeepLProvider(baseURL, apiKeyEnv string, timeoutSec int, proxy bool) *DeepLProvider {
	if baseURL == "" {
		baseURL = deeplAPIURL
	}
	return &DeepLProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKeyEnv: apiKeyEnv,
		client:    newHTTPClient(timeoutSec, proxy),
	}
}

type deeplRequest struct {
	Text       []string `json:"text"`
	SourceLang string   `json:"source_lang,omitempty"`
	TargetLang string   `json:"target_lang"`
}

type deeplTranslation struct {
	Text                   string `json:"text"`
	DetectedSourceLanguage string `json:"detected_source_language,omitempty"`
}

type deeplResponse struct {
	Translations []deeplTranslation `json:"translations"`
}

type deeplErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type deeplInfraErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *DeepLProvider) Translate(ctx context.Context, req TranslationRequest) (TranslationResult, error) {
	apiKey := os.Getenv(p.apiKeyEnv)
	if apiKey == "" {
		return TranslationResult{}, fmt.Errorf("API key not set: environment variable %s is empty or missing", p.apiKeyEnv)
	}

	body := deeplRequest{
		Text:       []string{req.Text},
		TargetLang: strings.ToUpper(req.TargetLang),
	}
	if req.SourceLang != "auto" && req.SourceLang != "" {
		body.SourceLang = strings.ToUpper(req.SourceLang)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := p.baseURL + "/v2/translate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "DeepL-Auth-Key "+apiKey)

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
		var errResp deeplErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			return TranslationResult{}, fmt.Errorf("DeepL API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		var infraResp deeplInfraErrorResponse
		if json.Unmarshal(respBody, &infraResp) == nil && infraResp.Error.Message != "" {
			return TranslationResult{}, fmt.Errorf("DeepL API error (%d): %s", resp.StatusCode, infraResp.Error.Message)
		}
		return TranslationResult{}, fmt.Errorf("DeepL API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var deeplResp deeplResponse
	if err := json.Unmarshal(respBody, &deeplResp); err != nil {
		return TranslationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(deeplResp.Translations) == 0 {
		return TranslationResult{}, fmt.Errorf("API returned no translations")
	}

	translated := strings.TrimSpace(deeplResp.Translations[0].Text)
	if translated == "" {
		return TranslationResult{}, fmt.Errorf("API returned empty translation")
	}

	return TranslationResult{
		Translation: translated,
		Provider:    "deepl",
	}, nil
}
