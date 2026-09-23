package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/y0n1d/trans-tui/internal/core"
)

// pressKey routes a key press through Update exactly like the Bubble Tea
// runtime does in normal mode and returns the resulting model.
func pressKey(t *testing.T, m Model, msg tea.KeyPressMsg) Model {
	t.Helper()
	r, _ := m.Update(msg)
	next, ok := r.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", r)
	}
	return next
}

// scrollTestModel returns a normal-mode model whose history is several
// viewports tall, so every default scroll key has room to move in its own
// direction.
func scrollTestModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel(t)
	m.viewport.SetContent(strings.Repeat("row\n", 4*m.viewport.Height()))
	return m
}

// bottomOffset returns the YOffset of a fully scrolled-down history.
func bottomOffset(m Model) int {
	m.viewport.GotoBottom()
	return m.viewport.YOffset()
}

// TestDefaultScrollBindings pins the real behavior of the non-configurable
// scroll keys. Their defaults are declared only by the switch in
// handleKeyPress, so this is the test that would notice a dropped or
// remapped key.
func TestDefaultScrollBindings(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyPressMsg
		want string // "up", "top", "down" or "bottom"
	}{
		{"up arrow", tea.KeyPressMsg{Code: tea.KeyUp}, "up"},
		{"k", tea.KeyPressMsg{Text: "k", Code: 'k'}, "up"},
		{"pgup", tea.KeyPressMsg{Code: tea.KeyPgUp}, "up"},
		{"b", tea.KeyPressMsg{Text: "b", Code: 'b'}, "up"},
		{"home", tea.KeyPressMsg{Code: tea.KeyHome}, "top"},
		{"g", tea.KeyPressMsg{Text: "g", Code: 'g'}, "top"},
		{"down arrow", tea.KeyPressMsg{Code: tea.KeyDown}, "down"},
		{"j", tea.KeyPressMsg{Text: "j", Code: 'j'}, "down"},
		{"pgdown", tea.KeyPressMsg{Code: tea.KeyPgDown}, "down"},
		{"f", tea.KeyPressMsg{Text: "f", Code: 'f'}, "down"},
		{"end", tea.KeyPressMsg{Code: tea.KeyEnd}, "bottom"},
		{"G", tea.KeyPressMsg{Text: "G", Code: 'G'}, "bottom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bottom := bottomOffset(scrollTestModel(t))
			if bottom == 0 {
				t.Fatal("test history must be taller than the viewport")
			}

			upward := tt.want == "up" || tt.want == "top"
			m := scrollTestModel(t)
			if upward {
				m.viewport.GotoBottom()
			}
			before := m.viewport.YOffset()

			m = pressKey(t, m, tt.msg)
			after := m.viewport.YOffset()

			switch tt.want {
			case "up":
				if after >= before {
					t.Errorf("YOffset = %d, want < %d (scroll up)", after, before)
				}
			case "top":
				if after != 0 {
					t.Errorf("YOffset = %d, want 0 (go to top)", after)
				}
			case "down":
				if after <= before {
					t.Errorf("YOffset = %d, want > %d (scroll down)", after, before)
				}
			case "bottom":
				if after != bottom {
					t.Errorf("YOffset = %d, want %d (go to bottom)", after, bottom)
				}
			}
		})
	}
}

// TestDefaultRetryKeyRetriesLastFailed pins "r" to the retry path when a
// failed translation exists: it clears the error, shows loading and returns
// the re-translate command.
func TestDefaultRetryKeyRetriesLastFailed(t *testing.T) {
	m := newTestModel(t)
	m.Error = "provider exploded"
	m.LastFailed = &core.TranslationRecord{
		ID:         "1",
		Source:     "hello",
		SourceLang: "auto",
		TargetLang: "zh-CN",
	}

	next, cmd := m.Update(tea.KeyPressMsg{Text: "r", Code: 'r'})
	model := next.(Model)
	if !model.Loading {
		t.Error("'r' with a failed request should set Loading")
	}
	if model.Error != "" {
		t.Errorf("Error = %q, want cleared before retry", model.Error)
	}
	if model.LastFailed == nil {
		t.Error("'r' must keep LastFailed so a later retry can repeat it")
	}
	if cmd == nil {
		t.Fatal("'r' must return the re-translate command")
	}
}

// TestDefaultRetryKeyWithoutFailureIsNoOp pins "r" when nothing failed:
// no loading state and no command.
func TestDefaultRetryKeyWithoutFailureIsNoOp(t *testing.T) {
	m := newTestModel(t)
	r, cmd := m.Update(tea.KeyPressMsg{Text: "r", Code: 'r'})
	next := r.(Model)
	if next.Loading {
		t.Error("'r' without a failed request must not start loading")
	}
	if cmd != nil {
		t.Error("'r' without a failed request must not return a command")
	}
}
