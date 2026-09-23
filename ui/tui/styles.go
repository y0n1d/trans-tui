package tui

// Package-level style aliases of the default theme. Rendering reads the
// model's own theme (Model.themeOr) so a custom [appearance] config takes
// effect; these vars exist for code and tests that work with the default
// look only (view_test's card-height math, input panel frame assertions).
var (
	// RecordStyle is the default style for a translation record.
	RecordStyle = defaultTheme.Record

	// InputPanelStyle is the default style for the input panel.
	InputPanelStyle = defaultTheme.InputPanel
)
