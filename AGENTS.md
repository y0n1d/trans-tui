# Project Instructions

## Architecture

Entry points: `cmd/trans-tui` (CLI → `internal/runtime`) and `cmd/trans-ocr` (standalone OCR, wires `config → internal/ocr` itself, no runtime/IPC).

Dependency directions (verified with `go list`, keep them this way):

- `internal/translator` defines the `Translator` interface (`translator.Translator`) and the provider implementations. It imports nothing from this repo.
- `internal/ocr` defines `OCRProvider` (Baidu only). Also a leaf package.
- `internal/core` owns domain types (`AppState`, `Service`, the plain-struct event messages in `events.go`) and language detection. It imports `internal/translator` (`core.NewService` takes a `translator.Translator`). It must not import `bubbletea` or any UI framework — the event message structs are deliberately framework-free so `ui/tui` can use them as Tea messages.
- `ui/tui` is the Bubble Tea UI. Depends on `core` and `translator` types; must not import `internal/ipc`. TUI ↔ backend communication goes through `core` event messages only.
- `internal/runtime` is the only package that wires everything: config → provider → service → IPC server → TUI. It imports `config`, `core`, `ipc`, `translator`, `ui/tui`, and bubbletea.

Bubble Tea **v2**: import path is `charm.land/bubbletea/v2` (likewise `bubbles/v2`, `lipgloss/v2`) — not `github.com/charmbracelet/bubbletea`. Go 1.27.1.

## IPC

`internal/ipc` uses a custom length-prefixed JSON protocol (4-byte big-endian length + JSON payload), defined in `internal/ipc/schema.go`. Protocol version is `ipc.ProtocolVersion` (currently 1). Changing the wire format requires bumping this and updating both client and server.

Socket path: `$XDG_RUNTIME_DIR/trans-tui.sock`, fallback `$TMPDIR/trans-tui/trans-tui.sock` (i.e. `os.TempDir()/trans-tui/trans-tui.sock`).

If a socket is already alive, the new invocation becomes a client and sends the text to the running server. The server validates a config **fingerprint** (SHA-256 of provider, appearance, and keybinding settings — `Config.Fingerprint()` in `internal/config`) — mismatched configs are rejected with an error message. Translation settings do not affect the fingerprint (asserted by tests).

To force a fresh server, remove the socket file or kill the old process. `internal/runtime` tests spin up real servers on temp sockets — they rely on the `SocketPath` config field (`toml:"-"`), not the real user socket.

## Security

API keys are never stored in TOML — only the **env var name** (`api_key_env`) is saved. The config file reads the actual key from the environment at runtime.

Never hardcode API keys. Never log Authorization headers.

## Config

Config file: `$XDG_CONFIG_HOME/trans-tui/config.toml` (auto-created on first run with a template). Override with `-c PATH`.

Provider types: `openai-compatible` (default), `google`, `deepl`, `libretranslate`. Provider selection lives in `newProvider` in `internal/runtime/lifecycle.go`.

Default target language `"auto"`: resolves to English when input is predominantly Chinese, otherwise Chinese (`core.ResolveTargetLang` / `core.IsChinese`). Detection uses Han ideograph ratio and rejects any text containing hiragana/katakana/hangul.

CLI parsing quirk (`cmd/trans-tui/main.go`): `-h/--help` and `-v/--version` are recognized only as the *first* argument; `--check-running` (exit 0/1 for launchers, used by `contrib/*.sh`) consumes following `-c` and ignores the rest. All other args are positional text. Input priority: positional text > piped stdin > `-i`.

## Commands

```
go test ./...                    # run all tests
go test ./internal/core -run TestX   # single test, standard go test flags
go vet ./...                     # static analysis
go build -o trans-tui ./cmd/trans-tui
go build -o trans-ocr ./cmd/trans-ocr
```

No Makefile, no CI workflows, no linter config — `go test ./...` + `go vet ./...` is the whole verification. Tests need no network or API keys (provider tests use `httptest`); full suite runs in ~10s.

## Go Module

`go.mod` pins `golang.org/x/sync` → v0.15.0 and `golang.org/x/sys` → v0.46.0 via `replace` directives. Do not remove these without checking for compatibility issues.

## Project Layout

- `contrib/` — shell helper scripts (grim-ocr-display, selection-translate, etc.) for Wayland/Niri workflows; they call `trans-tui --check-running` before launching a new `foot` window
- `specs/` — spec-driven feature specs (spec/plan/tasks per feature, e.g. `001-mvp-core-translation`)
- `.specify/` + `.opencode/commands/speckit.*` — spec-kit workflow scripts/templates and matching OpenCode commands

## Platform

Linux + Wayland only. No X11 dependencies. Clipboard uses OSC52 (`ui/tui/selection.go`, written to stderr, works over SSH).

## Do not

- Create a systemd service or permanent daemon
- Introduce X11 dependencies
- Store API keys in TOML or any file
