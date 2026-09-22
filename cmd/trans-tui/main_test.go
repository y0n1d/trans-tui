package main

import (
	"io"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// stdin helpers (real os.Stdin, real isStdinPiped / readStdin)
// ---------------------------------------------------------------------------

// pipedStdin replaces os.Stdin with a pipe pre-filled with data. The returned
// function reads whatever the code under test left unconsumed, so tests can
// assert not only what was read but also that stdin was never touched.
func pipedStdin(t *testing.T, data string) (unread func() string) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, data); err != nil {
		t.Fatal(err)
	}
	w.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})

	return func() string {
		rest, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read leftover stdin: %v", err)
		}
		return string(rest)
	}
}

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
			pipedStdin(t, tc.input)

			got := readStdin()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadStdinWhitespaceOnly(t *testing.T) {
	pipedStdin(t, "   \n\t  ")

	got := readStdin()
	if got != "   \n\t  " {
		t.Errorf("got %q, want %q", got, "   \n\t  ")
	}
}

// ---------------------------------------------------------------------------
// parseArgs: the real parser in main.go
// ---------------------------------------------------------------------------

func TestParseArgsPositionalText(t *testing.T) {
	opts := parseArgs([]string{"Hello", "world"})
	if opts.action != actionRun {
		t.Errorf("action = %v, want actionRun", opts.action)
	}
	if got := opts.text(); got != "Hello world" {
		t.Errorf("text = %q, want %q", got, "Hello world")
	}
	if opts.inputInitial || opts.displayMode {
		t.Error("plain text must not set any flag")
	}
	if opts.cfgPath != "" {
		t.Errorf("cfgPath = %q, want empty", opts.cfgPath)
	}
}

func TestParseArgsInputShortFlag(t *testing.T) {
	opts := parseArgs([]string{"-i"})
	if !opts.inputInitial {
		t.Error("-i flag should set inputInitial")
	}
	if got := opts.text(); got != "" {
		t.Errorf("text = %q, want empty", got)
	}
}

func TestParseArgsInputLongFlag(t *testing.T) {
	opts := parseArgs([]string{"--input"})
	if !opts.inputInitial {
		t.Error("--input flag should set inputInitial")
	}
	if got := opts.text(); got != "" {
		t.Errorf("text = %q, want empty", got)
	}
}

func TestParseArgsInputFlagWithText(t *testing.T) {
	opts := parseArgs([]string{"-i", "Hello"})
	if !opts.inputInitial {
		t.Error("-i flag should set inputInitial")
	}
	if got := opts.text(); got != "Hello" {
		t.Errorf("text = %q, want %q", got, "Hello")
	}
}

func TestParseArgsDisplayFlag(t *testing.T) {
	opts := parseArgs([]string{"--display", "OCR text"})
	if !opts.displayMode {
		t.Error("--display should set displayMode")
	}
	if opts.inputInitial {
		t.Error("--display should not set inputInitial")
	}
	if got := opts.text(); got != "OCR text" {
		t.Errorf("text = %q, want %q", got, "OCR text")
	}
}

func TestParseArgsDisplayFlagWithConfig(t *testing.T) {
	opts := parseArgs([]string{"--display", "-c", "/cfg", "OCR text"})
	if !opts.displayMode {
		t.Error("--display should set displayMode")
	}
	if opts.cfgPath != "/cfg" {
		t.Errorf("cfgPath = %q, want %q", opts.cfgPath, "/cfg")
	}
	if got := opts.text(); got != "OCR text" {
		t.Errorf("text = %q, want %q", got, "OCR text")
	}
}

func TestParseArgsConfigFlag(t *testing.T) {
	opts := parseArgs([]string{"-c", "/path/to/config.toml"})
	if opts.cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", opts.cfgPath, "/path/to/config.toml")
	}
	if opts.action != actionRun {
		t.Errorf("action = %v, want actionRun", opts.action)
	}
	if got := opts.text(); got != "" {
		t.Errorf("text = %q, want empty (-c consumes its value)", got)
	}
}

func TestParseArgsLastConfigFlagWins(t *testing.T) {
	opts := parseArgs([]string{"-c", "/cfg", "-i", "-c", "/other"})
	if opts.cfgPath != "/other" {
		t.Errorf("cfgPath = %q, want %q (last -c should win)", opts.cfgPath, "/other")
	}
	if !opts.inputInitial {
		t.Error("inputInitial should be true")
	}
}

func TestParseArgsConfigFlagWithoutValue(t *testing.T) {
	opts := parseArgs([]string{"-c"})
	if opts.cfgPath != "" {
		t.Errorf("cfgPath = %q, want empty when -c has no value", opts.cfgPath)
	}
	if got := opts.text(); got != "" {
		t.Errorf("text = %q, want empty (-c itself is not positional text)", got)
	}
}

// Unknown arguments are positional text: the CLI has no flag validation, and
// this test documents that real behavior instead of inventing a new one.
func TestParseArgsUnknownArgumentIsPositionalText(t *testing.T) {
	opts := parseArgs([]string{"--bogus", "hello"})
	if opts.action != actionRun {
		t.Errorf("action = %v, want actionRun", opts.action)
	}
	if got := opts.text(); got != "--bogus hello" {
		t.Errorf("text = %q, want %q", got, "--bogus hello")
	}
}

func TestParseArgsCheckRunning(t *testing.T) {
	opts := parseArgs([]string{"--check-running"})
	if opts.action != actionCheckRunning {
		t.Errorf("action = %v, want actionCheckRunning", opts.action)
	}
	if got := opts.text(); got != "" {
		t.Errorf("text = %q, want empty", got)
	}
}

func TestParseArgsCheckRunningWithConfig(t *testing.T) {
	opts := parseArgs([]string{"--check-running", "-c", "/cfg"})
	if opts.action != actionCheckRunning {
		t.Errorf("action = %v, want actionCheckRunning", opts.action)
	}
	if opts.cfgPath != "/cfg" {
		t.Errorf("cfgPath = %q, want %q", opts.cfgPath, "/cfg")
	}
}

func TestParseArgsCheckRunningOnlyAsFirstArgument(t *testing.T) {
	opts := parseArgs([]string{"hello", "--check-running"})
	if opts.action != actionRun {
		t.Errorf("action = %v, want actionRun when --check-running is not first", opts.action)
	}
	if got := opts.text(); got != "hello --check-running" {
		t.Errorf("text = %q, want %q", got, "hello --check-running")
	}
}

func TestParseArgsHelpAndVersion(t *testing.T) {
	if got := parseArgs([]string{"-h"}).action; got != actionHelp {
		t.Errorf(`parseArgs(["-h"]).action = %v, want actionHelp`, got)
	}
	if got := parseArgs([]string{"--version"}).action; got != actionVersion {
		t.Errorf(`parseArgs(["--version"]).action = %v, want actionVersion`, got)
	}
	// Only the first argument is a control flag; later ones are text.
	opts := parseArgs([]string{"hello", "-h"})
	if opts.action != actionRun {
		t.Errorf("action = %v, want actionRun when -h is not first", opts.action)
	}
	if got := opts.text(); got != "hello -h" {
		t.Errorf("text = %q, want %q", got, "hello -h")
	}
}

// ---------------------------------------------------------------------------
// resolveText: the real input-source priority in main.go
// ---------------------------------------------------------------------------

func TestArgumentTakesPrecedenceOverStdin(t *testing.T) {
	unread := pipedStdin(t, "Hello from stdin")

	// trans-tui "World"
	text := resolveText(parseArgs([]string{"World"}))

	if text != "World" {
		t.Errorf("text = %q, want %q (argument should take precedence)", text, "World")
	}
	if got := unread(); got != "Hello from stdin" {
		t.Errorf("stdin = %q, want untouched stdin", got)
	}
}

func TestInputModeNotAffectedByStdin(t *testing.T) {
	unread := pipedStdin(t, "Hello from stdin")

	// trans-tui -i
	text := resolveText(parseArgs([]string{"-i"}))

	if text != "" {
		t.Errorf("text = %q, want empty when -i is set", text)
	}
	if got := unread(); got != "Hello from stdin" {
		t.Errorf("stdin = %q, want untouched stdin when -i is set", got)
	}
}

func TestEmptyPipedStdinProducesEmptyText(t *testing.T) {
	pipedStdin(t, "   \n\t  ")

	text := resolveText(parseArgs(nil))
	if text != "" {
		t.Errorf("whitespace-only stdin should produce empty text, got %q", text)
	}
}

func TestStdinWithConfigFlag(t *testing.T) {
	unread := pipedStdin(t, "Hello from stdin")

	// trans-tui -c /path/to/config.toml
	opts := parseArgs([]string{"-c", "/path/to/config.toml"})
	text := resolveText(opts)

	if opts.cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", opts.cfgPath, "/path/to/config.toml")
	}
	if text != "Hello from stdin" {
		t.Errorf("text = %q, want %q (stdin should be read when only -c is given)", text, "Hello from stdin")
	}
	if got := unread(); got != "" {
		t.Errorf("stdin = %q, want fully consumed", got)
	}
}

func TestStdinConfigFlagAndInputMode(t *testing.T) {
	unread := pipedStdin(t, "should be ignored")

	// trans-tui -c /path/to/config.toml -i
	opts := parseArgs([]string{"-c", "/path/to/config.toml", "-i"})
	text := resolveText(opts)

	if opts.cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", opts.cfgPath, "/path/to/config.toml")
	}
	if !opts.inputInitial {
		t.Error("inputInitial should be true")
	}
	if text != "" {
		t.Errorf("text should be empty when -i is set, got %q", text)
	}
	if got := unread(); got != "should be ignored" {
		t.Errorf("stdin = %q, want untouched stdin when -i is set", got)
	}
}

func TestInputModeBeforeConfigFlag(t *testing.T) {
	unread := pipedStdin(t, "should be ignored")

	// trans-tui -i -c /path/to/config.toml
	opts := parseArgs([]string{"-i", "-c", "/path/to/config.toml"})
	text := resolveText(opts)

	if opts.cfgPath != "/path/to/config.toml" {
		t.Errorf("cfgPath = %q, want %q", opts.cfgPath, "/path/to/config.toml")
	}
	if !opts.inputInitial {
		t.Error("inputInitial should be true")
	}
	if text != "" {
		t.Errorf("text should be empty when -i is set, got %q", text)
	}
	if got := unread(); got != "should be ignored" {
		t.Errorf("stdin = %q, want untouched stdin when -i is set", got)
	}
}

func TestDisplayModeNotAffectedByStdin(t *testing.T) {
	unread := pipedStdin(t, "Hello from stdin")

	// trans-tui --display "OCR text"
	opts := parseArgs([]string{"--display", "OCR text"})
	text := resolveText(opts)

	if !opts.displayMode {
		t.Error("displayMode should be true")
	}
	if text != "OCR text" {
		t.Errorf("text = %q, want %q", text, "OCR text")
	}
	if got := unread(); got != "Hello from stdin" {
		t.Errorf("stdin = %q, want untouched stdin when positional text is present", got)
	}
}

func TestDisplayModeWithStdinPiped(t *testing.T) {
	pipedStdin(t, "OCR result from stdin")

	// trans-tui --display  (piped stdin, no positional text)
	opts := parseArgs([]string{"--display"})
	text := resolveText(opts)

	if !opts.displayMode {
		t.Error("displayMode should be true")
	}
	if text != "OCR result from stdin" {
		t.Errorf("text = %q, want %q (stdin should be read in display mode)", text, "OCR result from stdin")
	}
}
