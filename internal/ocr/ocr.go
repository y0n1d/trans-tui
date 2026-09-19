package ocr

import "context"

// OCRProvider is the contract for all OCR providers.
// Implementations must not depend on TUI or UI frameworks.
type OCRProvider interface {
	// Recognize sends an image to the OCR service and returns the recognized text.
	// The image parameter should contain the raw image bytes (e.g., PNG, JPEG).
	// The returned string is the plain text recognized from the image.
	Recognize(ctx context.Context, image []byte) (string, error)
}

// ProviderName is a type for OCR provider names.
type ProviderName string

const (
	ProviderBaidu ProviderName = "baidu"
)

// Config holds configuration for OCR providers.
type Config struct {
	Provider ProviderName
	// Additional provider-specific config can be added here.
}
