package core

// TranslationResultMsg is a message for a successful translation.
type TranslationResultMsg struct {
	RequestID   string
	Source      string
	Translation string
	SourceLang  string
	TargetLang  string
	Provider    string
	Model       string
}

// TranslationErrorMsg is a message for a failed translation.
type TranslationErrorMsg struct {
	RequestID  string
	Source     string
	Error      string
	SourceLang string
	TargetLang string
}

// DisplayTextMsg signals the TUI to display text without translation.
type DisplayTextMsg struct {
	RequestID string
	Text      string
}

// InitialTranslationMsg signals the TUI to send its initial translation.
type InitialTranslationMsg struct{}

// EnterInputModeMsg signals the TUI to open the input panel.
type EnterInputModeMsg struct{}
