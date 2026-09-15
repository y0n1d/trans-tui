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

// OpenAICompatibleConfig holds configuration for the OpenAI-compatible provider.
type OpenAICompatibleConfig struct {
	BaseURL    string // API base URL (e.g., "https://api.openai.com/v1")
	Model      string // Model name (e.g., "gpt-4o-mini")
	APIKeyEnv  string // Environment variable name containing the API key
	TimeoutSec int    // Request timeout in seconds (default: 30)
}

// OpenAICompatibleProvider implements Translator using an OpenAI-compatible API.
type OpenAICompatibleProvider struct {
	config OpenAICompatibleConfig
	client *http.Client
}

// NewOpenAICompatibleProvider creates a new OpenAI-compatible translator.
func NewOpenAICompatibleProvider(config OpenAICompatibleConfig) *OpenAICompatibleProvider {
	timeout := time.Duration(config.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &OpenAICompatibleProvider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// openaiMessage represents a chat completion message.
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiRequest represents the chat completion request body.
type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

// openaiChoice represents a single choice in the response.
type openaiChoice struct {
	Message openaiMessage `json:"message"`
}

// openaiResponse represents the chat completion response body.
type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
}

// buildSystemPrompt creates the translation system prompt based on request parameters.
func buildSystemPrompt(req TranslationRequest) string {
	var sb strings.Builder
	sb.WriteString("You are a professional translator. Translate the user's text accurately and naturally.")

	if req.SourceLang != "" && req.SourceLang != "auto" {
		fmt.Fprintf(&sb, " The source language is %s.", req.SourceLang)
	}
	if req.TargetLang != "" {
		fmt.Fprintf(&sb, " Translate to %s.", req.TargetLang)
	}

	sb.WriteString(" Return ONLY the translated text, nothing else.")
	return sb.String()
}

// Translate sends a translation request to the OpenAI-compatible API.
func (p *OpenAICompatibleProvider) Translate(ctx context.Context, req TranslationRequest) (TranslationResult, error) {
	apiKey := os.Getenv(p.config.APIKeyEnv)
	if apiKey == "" {
		return TranslationResult{}, fmt.Errorf("API key not set: environment variable %s is empty or missing", p.config.APIKeyEnv)
	}

	requestBody := openaiRequest{
		Model: p.config.Model,
		Messages: []openaiMessage{
			{Role: "system", Content: buildSystemPrompt(req)},
			{Role: "user", Content: req.Text},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return TranslationResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

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
		return TranslationResult{}, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return TranslationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return TranslationResult{}, fmt.Errorf("API returned no choices")
	}

	translated := strings.TrimSpace(openaiResp.Choices[0].Message.Content)
	if translated == "" {
		return TranslationResult{}, fmt.Errorf("API returned empty translation")
	}

	return TranslationResult{
		Translation: translated,
		Provider:    "openai-compatible",
		Model:       p.config.Model,
	}, nil
}
