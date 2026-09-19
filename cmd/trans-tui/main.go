package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/runtime"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: trans-tui [-i] <text>\n")
		os.Exit(1)
	}

	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: trans-tui [-i] <text>")
		fmt.Println("Translate text using a pluggable translation provider.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -i, --input    Open input mode on startup")
		fmt.Println("  -c, --config   Path to config file")
		fmt.Println("  -v, --version  Show version")
		os.Exit(0)
	}

	if args[0] == "-v" || args[0] == "--version" {
		fmt.Println("trans-tui 0.1.0")
		os.Exit(0)
	}

	inputInitial := false
	var remaining []string

	for _, arg := range args {
		if arg == "-i" || arg == "--input" {
			inputInitial = true
		} else {
			remaining = append(remaining, arg)
		}
	}

	cfgPath := ""
	var textParts []string
	for i, arg := range remaining {
		if arg == "-c" || arg == "--config" {
			if i+1 < len(remaining) {
				cfgPath = remaining[i+1]
			}
			textParts = append(textParts, remaining[:i]...)
			if i+2 < len(remaining) {
				textParts = append(textParts, remaining[i+2:]...)
			}
			break
		}
		textParts = append(textParts, arg)
	}

	text := strings.TrimSpace(strings.Join(textParts, " "))

	if text == "" && !inputInitial {
		fmt.Fprintf(os.Stderr, "Usage: trans-tui [-i] <text>\n")
		os.Exit(1)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	runtime.Run(text, inputInitial, cfg)
}
