//go:build ignore

# Contract: Translator Interface

**Module**: `internal/translator`
**Type**: Go interface

## Interface Definition

```go
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
```

## Contract Rules

1. **No TUI dependency**: The `translator` package MUST NOT import Bubble Tea, Lip Gloss, or any UI framework.
2. **Context cancellation**: Implementations MUST respect `ctx.Done()` and abort translation if the context is cancelled.
3. **Error returns**: On failure, return a non-nil error. The error message MUST be user-readable (will be displayed in TUI).
4. **Provider identification**: The `Provider` field MUST be set to a consistent identifier (e.g., "openai-compatible").
5. **No side effects**: `Translate` MUST NOT modify global state. It is safe to call concurrently (though the MVP serializes calls).

## Implementations (MVP)

| Provider          | Package                              | Description                         |
|-------------------|--------------------------------------|-------------------------------------|
| openai-compatible | `internal/translator/openai_compatible` | OpenAI-compatible API client        |

## Future Extensions

Additional providers (DeepSeek, Gemini, local Ollama, etc.) implement the same `Translator` interface. The `openai_compatible` package can serve any OpenAI-compatible API by changing the base URL and model in configuration. No changes to Core required.
