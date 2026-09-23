package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// KeyMap defines the configurable key bindings for the TUI. Only keys whose
// behavior is dispatched through key.Matches live here: the quit and manual
// input bindings (both overridable via [keybindings] config). Scroll and
// retry keys are dispatched by the literal switch in handleKeyPress — their
// defaults are declared there, next to the handling, so this struct must not
// redeclare keys nothing reads.
type KeyMap struct {
	Quit      key.Binding
	InputMode key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c", "esc"),
			key.WithHelp("q/ctrl+c/esc", "quit"),
		),
		InputMode: key.NewBinding(
			key.WithKeys(",", "，"),
			key.WithHelp(",/，", "input text"),
		),
	}
}

// NewKeyMapFromBindings creates a KeyMap from resolved config key bindings.
// Unset (empty) slices fall back to the defaults in DefaultKeyMap; keys that
// are not configurable (scroll, retry) are handled in handleKeyPress.
func NewKeyMapFromBindings(quitKeys, manualInputKeys []string) KeyMap {
	km := DefaultKeyMap()
	if len(quitKeys) > 0 {
		km.Quit = key.NewBinding(
			key.WithKeys(quitKeys...),
			key.WithHelp(formatKeys(quitKeys), "quit"),
		)
	}
	if len(manualInputKeys) > 0 {
		km.InputMode = key.NewBinding(
			key.WithKeys(manualInputKeys...),
			key.WithHelp(formatKeys(manualInputKeys), "input text"),
		)
	}
	return km
}

// Matches returns true if the key message matches the given binding.
func (km KeyMap) Matches(msg tea.KeyPressMsg, b key.Binding) bool {
	return key.Matches(msg, b)
}

// firstKey returns the primary key string for help display.
func firstKey(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// formatKeys joins keys with "/" for help display.
func formatKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	result := keys[0]
	for _, k := range keys[1:] {
		result += "/" + k
	}
	return result
}
