package translator

import "context"

// TranslationRequest represents a request to translate text.
type TranslationRequest struct {
	Text       string
	SourceLang string // "auto" for automatic detection
	TargetLang string // target language code
}

// TranslationResult represents the outcome of a translation attempt.
type TranslationResult struct {
	Translation string // translated text (empty on error)
	Provider    string // provider name (e.g., "openai-compatible")
	Model       string // model name (e.g., "gpt-4o-mini")
}

// Translator is the contract for all translation providers.
// Implementations must not depend on TUI or UI frameworks.
type Translator interface {
	Translate(ctx context.Context, req TranslationRequest) (TranslationResult, error)
}
