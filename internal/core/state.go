package core

import "time"

// TranslationRecord represents a single translation attempt.
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

// AppState holds the current state of the application.
type AppState struct {
	Records      []TranslationRecord
	Cursor       int
	ScrollOffset int
	Loading      bool
	Error        string
	LastFailed   *TranslationRecord
}
