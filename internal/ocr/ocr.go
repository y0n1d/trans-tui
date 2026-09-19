package ocr

import (
	"context"
	"fmt"
)

// OCRProvider is the contract for all OCR providers.
// Implementations must not depend on TUI or UI frameworks.
type OCRProvider interface {
	// Recognize sends an image to the OCR service and returns the recognized text.
	// The image parameter should contain the raw image bytes (e.g., PNG, JPEG).
	// The returned string is the plain text recognized from the image.
	Recognize(ctx context.Context, image []byte) (string, error)
}

// BaiduConfig holds configuration for the Baidu OCR provider.
type BaiduConfig struct {
	BaseURL      string
	APIKeyEnv    string
	SecretKeyEnv string
}

// Config holds configuration for OCR providers.
type Config struct {
	Provider     string
	Model        string
	LanguageType string
	Timeout      int
	Baidu        BaiduConfig
}

// NewProvider creates an OCR provider based on the given configuration.
func NewProvider(cfg Config) (OCRProvider, error) {
	switch cfg.Provider {
	case "baidu":
		return newBaiduOCR(cfg)
	default:
		return nil, fmt.Errorf("unsupported OCR provider: %s", cfg.Provider)
	}
}
