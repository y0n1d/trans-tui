package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/key"
)

// Action identifiers for TUI key bindings.
type Action int

const (
	ActionQuit Action = iota
	ActionManualInput
)

// KeyMap defines the key bindings for the TUI.
type KeyMap struct {
	Quit      key.Binding
	Up        key.Binding
	Down      key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	Home      key.Binding
	End       key.Binding
	Retry     key.Binding
	InputMode key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c", "esc"),
			key.WithHelp("q/ctrl+c/esc", "quit"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup/b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("pgdown/f", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		Retry: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry last failed"),
		),
		InputMode: key.NewBinding(
			key.WithKeys(",", "，"),
			key.WithHelp(",/，", "input text"),
		),
	}
}

// NewKeyMapFromBindings creates a KeyMap from resolved config key bindings.
// Non-configurable keys (scroll, retry, etc.) use defaults.
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

// FirstKey returns the primary key string for help display.
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
