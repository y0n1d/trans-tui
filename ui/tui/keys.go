package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/config"
)

// KeyMap defines the configurable key bindings for the TUI: one key.Binding
// per application-level action, all dispatched through key.Matches. Textarea
// editing keys (cursor movement, backspace, ...) stay owned by Bubbles'
// textarea keymap — only copy_selection is pushed into it, at Model
// construction, because the app intercepts it for its OSC52 clipboard.
type KeyMap struct {
	Quit           key.Binding
	ManualInput    key.Binding
	ScrollUp       key.Binding
	ScrollDown     key.Binding
	PageUp         key.Binding
	PageDown       key.Binding
	GotoTop        key.Binding
	GotoBottom     key.Binding
	PreviousRecord key.Binding
	NextRecord     key.Binding
	Retry          key.Binding
	DismissError   key.Binding
	CancelInput    key.Binding
	SubmitInput    key.Binding
	CopySelection  key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return NewKeyMapFromBindings(config.DefaultKeyBindings())
}

// NewKeyMapFromBindings creates a KeyMap from config key bindings. Actions
// left empty in kb fall back to their defaults (config.ResolveKeyBindings),
// so a partial config never leaves an action unbound.
func NewKeyMapFromBindings(kb config.KeyBindingsConfig) KeyMap {
	resolved := config.ResolveKeyBindings(kb)
	return KeyMap{
		Quit:           newBinding(resolved.Quit),
		ManualInput:    newBinding(resolved.ManualInput),
		ScrollUp:       newBinding(resolved.ScrollUp),
		ScrollDown:     newBinding(resolved.ScrollDown),
		PageUp:         newBinding(resolved.PageUp),
		PageDown:       newBinding(resolved.PageDown),
		GotoTop:        newBinding(resolved.GotoTop),
		GotoBottom:     newBinding(resolved.GotoBottom),
		PreviousRecord: newBinding(resolved.PreviousRecord),
		NextRecord:     newBinding(resolved.NextRecord),
		Retry:          newBinding(resolved.Retry),
		DismissError:   newBinding(resolved.DismissError),
		CancelInput:    newBinding(resolved.CancelInput),
		SubmitInput:    newBinding(resolved.SubmitInput),
		CopySelection:  newBinding(resolved.CopySelection),
	}
}

func newBinding(keys []string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...))
}

// Matches returns true if the key message matches the given binding.
func (km KeyMap) Matches(msg tea.KeyPressMsg, b key.Binding) bool {
	return key.Matches(msg, b)
}

// firstKey returns the primary key string for the status bar hints.
func firstKey(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}
