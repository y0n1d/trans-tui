package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/runtime"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		if isStdinPiped() {
			text := strings.TrimSpace(readStdin())
			if text != "" {
				cfg, err := config.Load("")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
					os.Exit(1)
				}
				runtime.Run(text, false, cfg)
				return
			}
		}
		fmt.Fprintf(os.Stderr, "Usage: trans-tui [-i] [-c config] <text>\n")
		fmt.Fprintf(os.Stderr, "       echo \"text\" | trans-tui\n")
		os.Exit(1)
	}

	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: trans-tui [-i] [-c config] <text>")
		fmt.Println("       echo \"text\" | trans-tui")
		fmt.Println("Translate text using a pluggable translation provider.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -i, --input        Open input mode on startup")
		fmt.Println("  -c, --config PATH  Path to config file (default: $XDG_CONFIG_HOME/trans-tui/config.toml)")
		fmt.Println("  -v, --version      Show version")
		fmt.Println()
		fmt.Println("Input sources (in order of priority):")
		fmt.Println("  1. Positional argument: trans-tui \"Hello world\"")
		fmt.Println("  2. Piped stdin:         echo \"Hello\" | trans-tui")
		fmt.Println("  3. Interactive input:   trans-tui -i")
		os.Exit(0)
	}

	if args[0] == "-v" || args[0] == "--version" {
		fmt.Println("trans-tui 0.1.0")
		os.Exit(0)
	}

	inputInitial := false
	cfgPath := ""
	var textParts []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-i", "--input":
			inputInitial = true
		case "-c", "--config":
			if i+1 < len(args) {
				cfgPath = args[i+1]
				i++ // skip the value
			}
		default:
			textParts = append(textParts, args[i])
		}
	}

	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		if isStdinPiped() {
			text = strings.TrimSpace(readStdin())
		}
	}

	if text == "" && !inputInitial {
		fmt.Fprintf(os.Stderr, "Usage: trans-tui [-i] [-c config] <text>\n")
		fmt.Fprintf(os.Stderr, "       echo \"text\" | trans-tui\n")
		os.Exit(1)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	runtime.Run(text, inputInitial, cfg)
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
