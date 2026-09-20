[English](README.md) | [简体中文](README.zh-CN.md)

# trans-tui

A lightweight terminal translation tool with interactive TUI, stdin support, and optional OCR integration.

## Features

- **Terminal-first** — works in any terminal emulator (foot, kitty, alacritty, etc.)
- **Bubble Tea TUI** — interactive interface with mouse selection and scrollback history
- **Single-instance** — multiple invocations share one TUI window via Unix socket IPC
- **stdin pipeline** — `echo "text" | trans-tui` for scripting and piping
- **Interactive input** — `trans-tui -i` opens an input panel in the TUI
- **Mouse selection** — select translated text in the TUI, auto-copied to clipboard via OSC52
- **Pluggable providers** — OpenAI-compatible, Google Translate, DeepL, LibreTranslate
- **Auto language detection** — Chinese input translates to English, everything else to Chinese
- **OCR companion CLI** — `trans-ocr` extracts text from images, pipe into `trans-tui`

## Installation

```bash
git clone https://github.com/y0n1d/trans-tui.git
cd trans-tui
go build -o trans-tui ./cmd/trans-tui
go build -o trans-ocr ./cmd/trans-ocr
```

Or install via `go install`:

```bash
go install github.com/y0n1d/trans-tui/cmd/trans-tui@latest
go install github.com/y0n1d/trans-tui/cmd/trans-ocr@latest
```

## Usage

Translate text directly:

```bash
trans-tui "Hello world"
```

Pipe text via stdin:

```bash
echo "Hello world" | trans-tui
cat file.txt | trans-tui
```

Open interactive input mode:

```bash
trans-tui -i
```

OCR pipeline — extract text from an image and translate it:

```bash
trans-ocr screenshot.png | trans-tui
cat image.png | trans-ocr - | trans-tui
```

Use a custom config file:

```bash
trans-tui -c /path/to/config.toml "Hello world"
```

## OCR

`trans-ocr` is a standalone CLI for OCR. It is **not** a built-in TUI mode.

Current OCR providers:

- **Baidu OCR** — requires `BAIDU_OCR_API_KEY` and `BAIDU_OCR_SECRET_KEY` environment variables

## Configuration

Default config location: `$XDG_CONFIG_HOME/trans-tui/config.toml`
(typically `~/.config/trans-tui/config.toml`)

The config file is auto-created on first run with a default template. Override with `-c PATH`.

Minimal config:

```toml
[provider]
type = "openai-compatible"
api_key_env = "OPENAI_API_KEY"
timeout = 30

[provider.openai]
base_url = "https://api.openai.com/v1"
model = "gpt-4o-mini"

[translation]
source_lang = "auto"
target_lang = "auto"
```

API keys are read from environment variables — never store actual keys in the config file.

## Providers

| Type | Env Variable | Notes |
|------|-------------|-------|
| `openai-compatible` | `OPENAI_API_KEY` | Default. Works with OpenAI, DeepSeek, SiliconFlow, etc. |
| `google` | `GOOGLE_TRANSLATE_API_KEY` | Google Cloud Translation API |
| `deepl` | `DEEPL_API_KEY` | Free plan: `api-free.deepl.com`, Pro: `api.deepl.com` |
| `libretranslate` | (optional) | Self-hosted. Set `base_url` in `[provider.libretranslate]` |

## Architecture

```
trans-tui
├── CLI (cmd/trans-tui)       — argument parsing, stdin detection
├── runtime                   — single-instance orchestration, IPC server lifecycle
├── IPC (internal/ipc)        — Unix socket protocol (length-prefixed JSON)
├── core (internal/core)      — service layer, language detection, state
├── translator providers      — pluggable translation backends
└── TUI (ui/tui)              — Bubble Tea interface

trans-ocr (cmd/trans-ocr)     — standalone OCR CLI
└── OCR providers (internal/ocr)
```

Key design principles:

- **On-demand** — no background daemon. The TUI process starts on first invocation and exits when closed.
- **Pipeline-friendly** — stdin/stdout works naturally with shell pipes.
- **Decoupled** — OCR and translation are independent. `trans-ocr` outputs text, `trans-tui` translates it.

## License

Not yet specified.
