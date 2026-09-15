# Data Model: MVP Core Translation

**Date**: 2026-09-15
**Feature**: 001-mvp-core-translation

## Entities

### TranslationRecord

Represents a single completed or failed translation entry in history.

| Field          | Type       | Description                                      |
|----------------|------------|--------------------------------------------------|
| ID             | string     | Unique identifier (UUID)                         |
| Source         | string     | Original text input                              |
| Translation    | string     | Translated text (empty if failed)                |
| SourceLang     | string     | Source language code (e.g., "auto", "en", "zh")  |
| TargetLang     | string     | Target language code (e.g., "zh", "en")          |
| Provider       | string     | Provider name used (e.g., "openai-compatible")   |
| Model          | string     | Model name used (e.g., "gpt-4o-mini")            |
| CreatedAt      | time.Time  | Timestamp when the record was created            |
| Error          | string     | Error message if translation failed (empty if ok)|

**Validation rules**:
- ID MUST be non-empty and unique within the session.
- Source MUST be non-empty.
- Either Translation OR Error MUST be non-empty (mutually exclusive).
- CreatedAt MUST be set at creation time.

**State transitions**:
```
[Created] → [Loading] → [Completed] (Translation non-empty)
                      → [Failed]    (Error non-empty)
```

### TranslationRequest

Represents an incoming request to translate text.

| Field          | Type       | Description                                      |
|----------------|------------|--------------------------------------------------|
| ID             | string     | Unique identifier (UUID)                         |
| Text           | string     | Text to translate                                |
| SourceLang     | string     | Source language (default: "auto")                 |
| TargetLang     | string     | Target language (default: "zh")                   |

**Validation rules**:
- Text MUST be non-empty.
- SourceLang defaults to "auto" if not specified.
- TargetLang defaults to "zh" if not specified.

### TranslationResult

Represents the outcome of a translation attempt returned by the provider.

| Field          | Type       | Description                                      |
|----------------|------------|--------------------------------------------------|
| Translation    | string     | Translated text (empty on error)                 |
| Provider       | string     | Provider name                                    |
| Model          | string     | Model name                                       |
| Error          | error      | Error if translation failed (nil on success)     |

### AppState

Represents the current state of the TUI.

| Field          | Type              | Description                                 |
|----------------|-------------------|---------------------------------------------|
| Records        | []TranslationRecord | All translation entries in chronological order |
| Cursor         | int               | Current scroll position (index into Records) |
| ScrollOffset   | int               | Pixel offset within the viewport             |
| Loading        | bool              | True while a translation is in progress      |
| Error          | string            | Current error message (empty if none)        |
| LastFailed     | *TranslationRecord | Reference to the most recent failed record (for retry) |

**State transitions**:
```
[Idle] → [Loading] (user submits text)
      → [Error]    (translation fails, Error set, LastFailed set)
      → [Idle]     (translation succeeds, record appended)
```

### IPC Messages

#### Request (CLI → Server)

| Field          | Type       | Description                                      |
|----------------|------------|--------------------------------------------------|
| Version        | int        | Protocol version (currently 1)                   |
| Type           | string     | Message type: "translate"                        |
| RequestID      | string     | Unique request identifier (UUID)                 |
| Text           | string     | Text to translate                                |
| SourceLang     | string     | Source language                                   |
| TargetLang     | string     | Target language                                   |

#### Response (Server → CLI)

| Field          | Type       | Description                                      |
|----------------|------------|--------------------------------------------------|
| Version        | int        | Protocol version (currently 1)                   |
| RequestID      | string     | Matches the request ID                           |
| OK             | bool       | True if translation succeeded                    |
| Translation    | string     | Translated text (empty on error)                 |
| Provider       | string     | Provider name                                    |
| Error          | string     | Error message (empty on success)                 |

## Relationships

```
TranslationRequest → (processed by) → Translator → TranslationResult → (stored as) → TranslationRecord
TranslationRecord → (accumulated in) → AppState.Records
AppState.LastFailed → (references) → TranslationRecord (when Error != "")
```
