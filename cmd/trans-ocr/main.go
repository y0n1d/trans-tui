package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/ocr"
)

const usage = `Usage: trans-ocr [-c config] <image-file | ->

Perform OCR on an image and output recognized text to stdout.

Arguments:
  <image-file>  Path to an image file (PNG, JPEG, etc.)
  -             Read image from stdin (binary)

Options:
  -c, --config PATH  Path to config file (default: $XDG_CONFIG_HOME/trans-tui/config.toml)
  -h, --help         Show this help message

Examples:
  trans-ocr screenshot.png
  cat image.png | trans-ocr -
  grim -g "$(slurp)" - | trans-ocr - | trans-tui

Configuration:
  OCR settings are read from [ocr] section of the config file.
  API keys are provided via environment variables (names configurable).
`

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfgPath := ""
	var positional []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		case "-c", "--config":
			if i+1 < len(args) {
				cfgPath = args[i+1]
				i++
			} else {
				fmt.Fprint(os.Stderr, "Error: -c requires an argument\n")
				os.Exit(1)
			}
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 1 {
		fmt.Fprint(os.Stderr, "Error: exactly one argument required (image file path or -)\n")
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	arg := positional[0]

	// Load config.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Convert config.OCR to ocr.Config.
	ocrCfg := ocr.Config{
		Provider:     cfg.OCR.Provider,
		Model:        cfg.OCR.Model,
		LanguageType: cfg.OCR.LanguageType,
		Timeout:      cfg.OCR.Timeout,
		Baidu: ocr.BaiduConfig{
			BaseURL:      cfg.OCR.Baidu.BaseURL,
			APIKeyEnv:    cfg.OCR.Baidu.APIKeyEnv,
			SecretKeyEnv: cfg.OCR.Baidu.SecretKeyEnv,
		},
	}

	// Create OCR provider.
	provider, err := ocr.NewProvider(ocrCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Read image data.
	var imageData []byte
	if arg == "-" {
		imageData, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read from stdin: %v\n", err)
			os.Exit(1)
		}
	} else {
		imageData, err = os.ReadFile(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read file: %v\n", err)
			os.Exit(1)
		}
	}

	// Perform OCR.
	text, err := provider.Recognize(ctx, imageData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: OCR failed: %v\n", err)
		os.Exit(1)
	}

	// Output recognized text to stdout.
	fmt.Println(text)
}
