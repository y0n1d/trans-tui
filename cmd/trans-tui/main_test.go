package main

import (
	"os"
	"strings"
	"testing"
)

func TestIsStdinPiped(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
		w.Close()
	}()

	if !isStdinPiped() {
		t.Error("expected piped stdin to be detected")
	}
}

func TestReadStdin(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"single line", "Hello world", "Hello world"},
		{"chinese", "你好世界", "你好世界"},
		{"multiline", "line1\nline2\nline3", "line1\nline2\nline3"},
		{"trailing newline", "Hello\n", "Hello\n"},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, w, _ := os.Pipe()
			old := os.Stdin
			os.Stdin = r
			defer func() {
				os.Stdin = old
				r.Close()
			}()

			go func() {
				w.Write([]byte(tc.input))
				w.Close()
			}()

			got := readStdin()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadStdinWhitespaceOnly(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("   \n\t  "))
		w.Close()
	}()

	got := readStdin()
	if got != "   \n\t  " {
		t.Errorf("got %q, want %q", got, "   \n\t  ")
	}
}

// parseArgs simulates the CLI parsing logic from main().
func parseArgs(args []string) (inputInitial bool, cfgPath string, textParts []string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i", "--input":
			inputInitial = true
		case "-c", "--config":
			if i+1 < len(args) {
				cfgPath = args[i+1]
				i++
			}
		default:
			textParts = append(textParts, args[i])
		}
	}
	return
}

func TestArgumentTakesPrecedenceOverStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("Hello from stdin"))
		w.Close()
	}()

	// Simulate: trans-tui "World"
	inputInitial, _, textParts := parseArgs([]string{"World"})
	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if text != "World" {
		t.Errorf("text = %q, want %q (argument should take precedence)", text, "World")
	}
}

func TestInputModeNotAffectedByStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("Hello from stdin"))
		w.Close()
	}()

	// Simulate: trans-tui -i
	inputInitial, _, textParts := parseArgs([]string{"-i"})
	text := strings.TrimSpace(strings.Join(textParts, " "))

	stdinRead := false
	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
			stdinRead = true
		}
	}

	if stdinRead {
		t.Error("stdin should not be read when -i is set")
	}
	if !inputInitial {
		t.Error("inputInitial should be true")
	}
}

func TestEmptyPipedStdinProducesEmptyText(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Close()
	}()

	text := ""
	inputInitial := false

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if text != "" {
		t.Errorf("empty stdin should produce empty text, got %q", text)
	}
}

func TestStdinWithConfigFlag(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("Hello from stdin"))
		w.Close()
	}()

	// Simulate: trans-tui -c /path/to/config.toml
	inputInitial, cfgPath, textParts := parseArgs([]string{"-c", "/path/to/config.toml"})
	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", cfgPath, "/path/to/config.toml")
	}
	if text != "Hello from stdin" {
		t.Errorf("text = %q, want %q (stdin should be read when only -c is given)", text, "Hello from stdin")
	}
}

func TestStdinConfigFlagAndInputMode(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("should be ignored"))
		w.Close()
	}()

	// Simulate: trans-tui -c /path/to/config.toml -i
	inputInitial, cfgPath, textParts := parseArgs([]string{"-c", "/path/to/config.toml", "-i"})
	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", cfgPath, "/path/to/config.toml")
	}
	if !inputInitial {
		t.Error("inputInitial should be true")
	}
	if text != "" {
		t.Errorf("text should be empty when -i is set, got %q", text)
	}
}

func TestInputModeBeforeConfigFlag(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("should be ignored"))
		w.Close()
	}()

	// Simulate: trans-tui -i -c /path/to/config.toml
	inputInitial, cfgPath, textParts := parseArgs([]string{"-i", "-c", "/path/to/config.toml"})
	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", cfgPath, "/path/to/config.toml")
	}
	if !inputInitial {
		t.Error("inputInitial should be true")
	}
	if text != "" {
		t.Errorf("text should be empty when -i is set, got %q", text)
	}
}

func TestConfigFlagSkipsStdinWhenInputAlsoPresent(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()

	go func() {
		w.Write([]byte("from stdin"))
		w.Close()
	}()

	// Simulate: trans-tui -c /cfg -i -c /other (last -c wins)
	inputInitial, cfgPath, textParts := parseArgs([]string{"-c", "/cfg", "-i", "-c", "/other"})
	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if cfgPath != "/other" {
		t.Errorf("cfgPath = %q, want %q (last -c should win)", cfgPath, "/other")
	}
	if !inputInitial {
		t.Error("inputInitial should be true")
	}
	if text != "" {
		t.Errorf("text should be empty, got %q", text)
	}
}
