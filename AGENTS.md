# Project Instructions

## Architecture

`internal/core` owns interfaces (`Translator`, `OCRProvider`) and domain types. It must not import `bubbletea` or any UI framework.

`ui/tui` is the Bubble Tea UI. It depends on `core.Service` and `core.AppState`.

`internal/translator` implements `core.Translator`. No provider file may import from `ui/`.

`internal/runtime` is the only package that wires everything together: config → provider → service → IPC server → TUI. It imports both `core` and `ui/tui`.

`internal/ipc` uses a custom length-prefixed JSON protocol (4-byte big-endian length + JSON payload). Protocol version is `ipc.ProtocolVersion` (currently 1). Changing the wire format requires bumping this and updating both client and server.

## Security

API keys are never stored in TOML — only the **env var name** (`api_key_env`) is saved. The config file reads the actual key from the environment at runtime.

Never hardcode API keys. Never log Authorization headers.

## IPC

Socket path: `$XDG_RUNTIME_DIR/trans-tui.sock` (falls back to `$TMPDIR/trans-tui.sock`).

If a socket is already alive, the new invocation becomes a client and sends the text to the running server. The server validates a config **fingerprint** (SHA-256 of provider settings) — mismatched configs are rejected with an error message.

To force a fresh server, remove the socket file or kill the old process.

## Config

Config file: `$XDG_CONFIG_HOME/trans-tui/config.toml` (auto-created on first run with a template). Override with `-c PATH`.

Provider types: `openai-compatible` (default), `google`, `deepl`, `libretranslate`.

Default target language `"auto"`: resolves to English when input is predominantly Chinese, otherwise Chinese. Detection uses Han ideograph ratio and excludes Japanese/Korean scripts.

## Commands

```
go test ./...                    # run all tests
go vet ./...                     # static analysis
go build -o trans-tui ./cmd/trans-tui
go build -o trans-ocr ./cmd/trans-ocr
```

No Makefile, no CI workflows, no linter config. Tests are fast (all cached after first run).

## Go Module

`go.mod` pins `golang.org/x/sync` → v0.15.0 and `golang.org/x/sys` → v0.46.0 via `replace` directives. Do not remove these without checking for compatibility issues.

## Project Layout

- `contrib/` — shell helper scripts (grim-ocr-display, selection-translate, etc.) for Wayland workflows
- `specs/` — design specs and task plans for the MVP implementation

## Platform

Linux + Wayland only. No X11 dependencies. Clipboard uses OSC52 (terminal escape, works over SSH).

## Do not

- Create a systemd service or permanent daemon
- Introduce X11 dependencies
- Store API keys in TOML or any file
