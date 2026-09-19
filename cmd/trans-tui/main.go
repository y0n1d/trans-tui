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
		fmt.Fprintf(os.Stderr, "Usage: trans-tui <text>\n")
		os.Exit(1)
	}

	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: trans-tui <text>")
		fmt.Println("Translate text using a pluggable translation provider.")
		os.Exit(0)
	}

	if args[0] == "-v" || args[0] == "--version" {
		fmt.Println("trans-tui 0.1.0")
		os.Exit(0)
	}

	text := strings.Join(args, " ")

	cfgPath := ""
	for i, arg := range args {
		if arg == "-c" || arg == "--config" {
			if i+1 < len(args) {
				cfgPath = args[i+1]
				text = strings.Join(args[:i], " ")
				if i+2 < len(args) {
					text += " " + strings.Join(args[i+2:], " ")
				}
				text = strings.TrimSpace(text)
			}
			break
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	runtime.Run(text, cfg)
}
