package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/y0n1d/trans-tui/internal/ocr"
)

const usage = `Usage: trans-ocr <image-file | ->

Perform OCR on an image and output recognized text to stdout.

Arguments:
  <image-file>  Path to an image file (PNG, JPEG, etc.)
  -             Read image from stdin (binary)

Examples:
  trans-ocr screenshot.png
  cat image.png | trans-ocr -
  grim -g "$(slurp)" - | trans-ocr - | trans-tui

Environment variables:
  BAIDU_OCR_API_KEY      Baidu OCR API key
  BAIDU_OCR_SECRET_KEY   Baidu OCR secret key
`

func main() {
	// Set up signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(os.Args) != 2 {
		fmt.Fprint(os.Stderr, "Error: exactly one argument required\n")
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	arg := os.Args[1]
	if arg == "--help" || arg == "-h" {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Read image data
	var imageData []byte
	var err error

	if arg == "-" {
		// Read from stdin
		imageData, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read from stdin: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Read from file
		imageData, err = os.ReadFile(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read file: %v\n", err)
			os.Exit(1)
		}
	}

	// Create OCR provider
	provider, err := ocr.NewBaiduOCR()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Perform OCR
	text, err := provider.Recognize(ctx, imageData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: OCR failed: %v\n", err)
		os.Exit(1)
	}

	// Output recognized text to stdout
	fmt.Println(text)
}
