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
	httpTimeout  = 30 * time.Second
	maxImageSize = 10 * 1024 * 1024 // 10 MB, based on Baidu's limit
)

var (
	baiduTokenURL = "https://aip.baidubce.com/oauth/2.0/token"
	baiduOCRURL   = "https://aip.baidubce.com/rest/2.0/ocr/v1/general"
)

// BaiduOCR implements OCRProvider for Baidu's OCR service.
type BaiduOCR struct {
	apiKey     string
	secretKey  string
	httpClient *http.Client
}

// NewBaiduOCR creates a new BaiduOCR provider.
// It reads API key and secret key from environment variables.
func NewBaiduOCR() (*BaiduOCR, error) {
	apiKey := os.Getenv("BAIDU_OCR_API_KEY")
	secretKey := os.Getenv("BAIDU_OCR_SECRET_KEY")
	if apiKey == "" || secretKey == "" {
		return nil, fmt.Errorf("BAIDU_OCR_API_KEY and BAIDU_OCR_SECRET_KEY environment variables must be set")
	}
	return &BaiduOCR{
		apiKey:    apiKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}, nil
}

// Recognize sends an image to Baidu OCR and returns the recognized text.
func (b *BaiduOCR) Recognize(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 {
		return "", fmt.Errorf("empty image")
	}
	if len(image) > maxImageSize {
		return "", fmt.Errorf("image too large (%d bytes, max %d)", len(image), maxImageSize)
	}

	// Get access token
	accessToken, err := b.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Encode image to base64
	imageBase64 := base64.StdEncoding.EncodeToString(image)

	// Prepare form data
	form := url.Values{}
	form.Set("image", imageBase64)
	form.Set("language_type", "CHN_ENG") // Support Chinese and English

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baiduOCRURL+"?access_token="+accessToken, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OCR request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ocrResp baiduOCRResponse
	if err := json.Unmarshal(body, &ocrResp); err != nil {
		return "", fmt.Errorf("failed to parse OCR response: %w", err)
	}

	// Check for API error
	if ocrResp.ErrorCode != 0 {
		return "", fmt.Errorf("Baidu OCR error %d: %s", ocrResp.ErrorCode, ocrResp.ErrorMessage)
	}

	// Extract text lines
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
func (b *BaiduOCR) getAccessToken(ctx context.Context) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	params.Set("client_id", b.apiKey)
	params.Set("client_secret", b.secretKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baiduTokenURL+"?"+params.Encode(), nil)
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
