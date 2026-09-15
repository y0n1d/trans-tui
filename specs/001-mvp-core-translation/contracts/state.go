//go:build ignore

# Contract: Shared State Structures

**Module**: `internal/core`
**Type**: Go types shared between Core and UI

## Types

### TranslationRecord

```go
package core

import "time"

type TranslationRecord struct {
    ID          string    `json:"id"`
    Source      string    `json:"source"`
    Translation string    `json:"translation,omitempty"`
    SourceLang  string    `json:"source_lang"`
    TargetLang  string    `json:"target_lang"`
    Provider    string    `json:"provider,omitempty"`
    Model       string    `json:"model,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    Error       string    `json:"error,omitempty"`
}
```

### AppState

```go
type AppState struct {
    Records    []TranslationRecord
    Cursor     int
    ScrollOffset int
    Loading    bool
    Error      string
    LastFailed *TranslationRecord
}
```

## Contract Rules

1. **Immutable records**: Once a `TranslationRecord` is appended to `AppState.Records`, it MUST NOT be modified.
2. **Append-only**: New records are always appended to the end of `Records`. No insertion or deletion in MVP.
3. **Error mutual exclusion**: A record has either a non-empty `Translation` or a non-empty `Error`, never both.
4. **LastFailed reference**: `LastFailed` points to the most recent record with a non-empty `Error`. It is cleared when a new request starts or when retry succeeds.
5. **No UI dependency**: These types MUST NOT import Bubble Tea or any UI framework. They are pure data structures.
