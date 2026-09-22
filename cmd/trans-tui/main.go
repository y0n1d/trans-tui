package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/runtime"
)

// cliAction selects what main does after the command line is parsed.
type cliAction int

const (
	actionRun          cliAction = iota // translate / display / input mode
	actionHelp                          // -h or --help as first argument
	actionVersion                       // -v or --version as first argument
	actionCheckRunning                  // --check-running as first argument
)

// cliOptions is the parsed command line. parseArgs is its only producer, so
// main() and the tests exercise the same parser.
type cliOptions struct {
	action       cliAction
	inputInitial bool
	displayMode  bool
	cfgPath      string
	textParts    []string
}

// text returns the positional text joined into one string, as the CLI
// documents: trans-tui Hello world  →  "Hello world".
func (o cliOptions) text() string {
	return strings.TrimSpace(strings.Join(o.textParts, " "))
}

// parseArgs parses the command line exactly as the shipped CLI behaves:
//   - no arguments means "piped stdin or usage" (handled by resolveText),
//   - -h/--help, -v/--version and --check-running are recognized only when
//     they are the first argument,
//   - -i/--input, --display and -c/--config (which consumes the next
//     argument) are flags; every other argument is positional text.
func parseArgs(args []string) cliOptions {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help":
			return cliOptions{action: actionHelp}
		case "-v", "--version":
			return cliOptions{action: actionVersion}
		}
	}

	opts := cliOptions{action: actionRun}
	start := 0
	// --check-running still accepts -c/--config after itself; its remaining
	// arguments are ignored rather than treated as text.
	if len(args) > 0 && args[0] == "--check-running" {
		opts.action = actionCheckRunning
		start = 1
	}

	for i := start; i < len(args); i++ {
		switch args[i] {
		case "-i", "--input":
			opts.inputInitial = true
		case "--display":
			opts.displayMode = true
		case "-c", "--config":
			if i+1 < len(args) {
				opts.cfgPath = args[i+1]
				i++ // skip the value
			}
		default:
			opts.textParts = append(opts.textParts, args[i])
		}
	}
	return opts
}

// resolveText applies the documented input-source priority: positional text
// first, otherwise piped stdin — and stdin is only read when there is no text
// and input mode was not requested. An empty result means "no input".
func resolveText(opts cliOptions) string {
	text := opts.text()
	if text != "" || opts.inputInitial {
		return text
	}
	if !isStdinPiped() {
		return ""
	}
	return strings.TrimSpace(readStdin())
}

func main() {
	opts := parseArgs(os.Args[1:])

	if opts.action == actionHelp {
		fmt.Println("Usage: trans-tui [-i] [--display] [-c config] <text>")
		fmt.Println("       echo \"text\" | trans-tui")
		fmt.Println("Translate text using a pluggable translation provider.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -i, --input        Open input mode on startup")
		fmt.Println("  --display          Display text without translation (for OCR results)")
		fmt.Println("  --check-running    Exit 0 if server is running, 1 otherwise (for launchers)")
		fmt.Println("  -c, --config PATH  Path to config file (default: $XDG_CONFIG_HOME/trans-tui/config.toml)")
		fmt.Println("  -v, --version      Show version")
		fmt.Println()
		fmt.Println("Input sources (in order of priority):")
		fmt.Println("  1. Positional argument: trans-tui \"Hello world\"")
		fmt.Println("  2. Piped stdin:         echo \"Hello\" | trans-tui")
		fmt.Println("  3. Interactive input:   trans-tui -i")
		fmt.Println()
		fmt.Println("Display mode (for OCR/pipeline use):")
		fmt.Println("  trans-ocr - | trans-tui --display")
		fmt.Println("  Shows text without translating. Use with grim/slurp/trans-ocr.")
		os.Exit(0)
	}

	if opts.action == actionVersion {
		fmt.Println("trans-tui 0.1.0")
		os.Exit(0)
	}

	// --check-running: exit 0 if a server is reachable, exit 1 otherwise.
	// Used by launchers to avoid creating a new foot window when the TUI is
	// already open. Accepts -c/--config for consistent config resolution.
	if opts.action == actionCheckRunning {
		cfg, err := config.Load(opts.cfgPath)
		if err != nil {
			os.Exit(1)
		}
		if runtime.IsRunning(cfg) {
			os.Exit(0)
		}
		os.Exit(1)
	}

	text := resolveText(opts)

	if text == "" && !opts.inputInitial {
		fmt.Fprintf(os.Stderr, "Usage: trans-tui [-i] [--display] [-c config] <text>\n")
		fmt.Fprintf(os.Stderr, "       echo \"text\" | trans-tui\n")
		os.Exit(1)
	}

	cfg, err := config.Load(opts.cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	runtime.Run(text, opts.inputInitial, opts.displayMode, cfg)
}

// isStdinPiped returns true if stdin is connected to a pipe or file
// (not an interactive terminal).
func isStdinPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// readStdin reads all remaining data from stdin until EOF.
func readStdin() string {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}
	return string(data)
}
