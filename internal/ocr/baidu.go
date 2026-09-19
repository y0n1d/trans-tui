package ocr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// maxImageSize is Baidu's limit for all OCR interfaces:
	// base64 encoding + URL encode must not exceed 4MB.
	// Source: https://cloud.baidu.com/doc/OCR/s/Ck3h7y2ia
	maxImageSize = 4 * 1024 * 1024

	// baiduTokenPath is the OAuth token endpoint path.
	baiduTokenPath = "/oauth/2.0/token"
)

// baiduModelEndpoints maps model names to their API path suffixes.
var baiduModelEndpoints = map[string]string{
	"general_basic":  "/rest/2.0/ocr/v1/general_basic",
	"accurate_basic": "/rest/2.0/ocr/v1/accurate_basic",
	"general":        "/rest/2.0/ocr/v1/general",
	"accurate":       "/rest/2.0/ocr/v1/accurate",
}

// baiduOCR implements OCRProvider for Baidu's OCR service.
type baiduOCR struct {
	baseURL      string
	apiKey       string
	secretKey    string
	endpoint     string
	languageType string
	httpClient   *http.Client
}

// newBaiduOCR creates a new Baidu OCR provider from config.
func newBaiduOCR(cfg Config) (*baiduOCR, error) {
	if cfg.Baidu.BaseURL == "" {
		return nil, fmt.Errorf("baidu base_url is required")
	}
	if cfg.Baidu.APIKeyEnv == "" {
		return nil, fmt.Errorf("baidu api_key_env is required")
	}
	if cfg.Baidu.SecretKeyEnv == "" {
		return nil, fmt.Errorf("baidu secret_key_env is required")
	}

	endpoint, ok := baiduModelEndpoints[cfg.Model]
	if !ok {
		return nil, fmt.Errorf("unsupported OCR model: %s", cfg.Model)
	}

	apiKey := os.Getenv(cfg.Baidu.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s must be set", cfg.Baidu.APIKeyEnv)
	}
	secretKey := os.Getenv(cfg.Baidu.SecretKeyEnv)
	if secretKey == "" {
		return nil, fmt.Errorf("environment variable %s must be set", cfg.Baidu.SecretKeyEnv)
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if cfg.Timeout < 0 {
		return nil, fmt.Errorf("invalid OCR timeout: %d seconds", cfg.Timeout)
	}
	if cfg.Timeout == 0 {
		timeout = 30 * time.Second
	}

	return &baiduOCR{
		baseURL:      strings.TrimRight(cfg.Baidu.BaseURL, "/"),
		apiKey:       apiKey,
		secretKey:    secretKey,
		endpoint:     endpoint,
		languageType: cfg.LanguageType,
		httpClient:   &http.Client{Timeout: timeout},
	}, nil
}

// Recognize sends an image to Baidu OCR and returns the recognized text.
func (b *baiduOCR) Recognize(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 {
		return "", fmt.Errorf("empty image")
	}

	// Encode image to base64 first.
	imageBase64 := base64.StdEncoding.EncodeToString(image)

	// Build form values.
	form := url.Values{}
	form.Set("image", imageBase64)
	form.Set("language_type", b.languageType)

	// Baidu's limit applies to base64 + URL encode of the image parameter.
	// We check the URL-encoded form value to match what actually gets sent.
	encodedImage := url.QueryEscape(imageBase64)
	if len(encodedImage) > maxImageSize {
		return "", fmt.Errorf("image too large after base64 encoding (%d bytes, max %d)", len(encodedImage), maxImageSize)
	}

	// Get access token.
	accessToken, err := b.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Create request.
	ocrURL := b.baseURL + b.endpoint + "?access_token=" + accessToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request.
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status.
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OCR request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response.
	var ocrResp baiduOCRResponse
	if err := json.Unmarshal(body, &ocrResp); err != nil {
		return "", fmt.Errorf("failed to parse OCR response: %w", err)
	}

	// Check for API error.
	if ocrResp.ErrorCode != 0 {
		return "", fmt.Errorf("Baidu OCR error %d: %s", ocrResp.ErrorCode, ocrResp.ErrorMessage)
	}

	// Extract text lines.
	if len(ocrResp.WordsResult) == 0 {
		return "", fmt.Errorf("no text recognized")
	}

	var lines []string
	for _, item := range ocrResp.WordsResult {
		lines = append(lines, item.Words)
	}
	return strings.Join(lines, "\n"), nil
}

// getAccessToken retrieves an access token from Baidu OAuth.
func (b *baiduOCR) getAccessToken(ctx context.Context) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	params.Set("client_id", b.apiKey)
	params.Set("client_secret", b.secretKey)

	tokenURL := b.baseURL + baiduTokenPath + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp baiduTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}

// baiduTokenResponse represents the response from Baidu OAuth token endpoint.
type baiduTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// baiduOCRResponse represents the response from Baidu OCR endpoint.
type baiduOCRResponse struct {
	ErrorCode      int             `json:"error_code"`
	ErrorMessage   string          `json:"error_msg"`
	WordsResultNum int             `json:"words_result_num"`
	WordsResult    []baiduWordItem `json:"words_result"`
}

// baiduWordItem represents a single line of recognized text.
type baiduWordItem struct {
	Words string `json:"words"`
}
