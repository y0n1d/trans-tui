# Implementation Plan: MVP Core Translation

**Branch**: `001-mvp-core-translation` | **Date**: 2026-09-15 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-mvp-core-translation/spec.md`

## Summary

Build the minimal closed-loop translation tool: CLI receives text, starts a Server if none exists, opens a foot TUI via Bubble Tea, translates via a pluggable provider interface (initially OpenAI-compatible), appends results to in-memory history, and supports scrolling, error display, and retry. Subsequent CLI invocations detect the existing Unix socket and send requests to the running Server instead of spawning a new instance.

## Technical Context

**Language/Version**: Go 1.22+ (minimum; actual version may be higher)

**Primary Dependencies**: Bubble Tea (TUI framework), Lip Gloss (styling), Bubbles (components)

**Storage**: In-memory only (no persistence in MVP)

**Testing**: `go test ./...`, `go vet ./...`

**Target Platform**: Linux, Wayland (Niri compositor), foot terminal

**Project Type**: CLI + TUI desktop application

**Performance Goals**: Translation result displayed within 5 seconds for texts under 500 characters; 50+ history entries scrollable without lag

**Constraints**: Single instance only; no daemon/systemd; API keys via environment variables only; Core must not depend on Bubble Tea; Wayland only (no X11)

**Scale/Scope**: Single-user desktop tool; in-memory history released on TUI close

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Core-UI Separation | PASS | Core modules (translation, IPC, config) have no TUI dependency. UI layer depends on Core interfaces. |
| II. On-Demand Lifecycle | PASS | Server starts only when TUI opens; exits when TUI closes; no daemon. |
| III. Single Instance, State Continuity | PASS | Socket detection prevents duplicate instances; history appends, never overwrites. |
| IV. Provider Extensibility | PASS | Translator interface defined in Core; OpenAI-compatible is one implementation in Infrastructure. |
| V. Security by Environment Variables | PASS | API key read from env var only; config stores env var name, not secret. |
| VI. Stable Contracts | PASS | IPC schema, Translator interface, and data structures defined as shared contracts before implementation. |

No violations. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/001-mvp-core-translation/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── translator.go    # Translator interface contract
│   ├── ipc.go           # IPC message schema
│   └── state.go         # Shared data structures
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/my-trans/
└── main.go              # Entry point: CLI parsing, mode dispatch

internal/
├── core/
│   ├── state.go         # AppState, TranslationRecord
│   ├── service.go       # Translation service (orchestration)
│   └── events.go        # Event types for UI communication
├── translator/
│   ├── provider.go              # Translator interface (contract)
│   └── openai_compatible.go     # OpenAI-compatible implementation
├── ipc/
│   ├── server.go        # Unix socket server (in TUI process)
│   ├── client.go        # Unix socket client (in CLI process)
│   └── schema.go        # Request/response message types
├── config/
│   └── config.go        # TOML config loader
└── runtime/
    └── lifecycle.go     # TUI + Server lifecycle: socket check, instance detection, startup, shutdown, cleanup

ui/tui/
├── model.go             # Bubble Tea model (AppState wrapper)
├── update.go            # Message handlers (translation results, scroll, errors)
├── view.go              # Rendering (history list, status bar, error display)
├── keys.go              # Key bindings (scroll, retry, quit)
└── styles.go            # Lip Gloss styles

configs/
└── example.toml         # Example configuration file
```

**Structure Decision**: Single project with clear internal/ and ui/ separation. Core lives in `internal/core` and `internal/translator` with zero TUI imports. UI lives in `ui/tui` and depends on Core interfaces. `internal/runtime/lifecycle.go` owns the full TUI + Server lifecycle: socket detection, instance check, startup orchestration, and shutdown cleanup. This enforces Principle I (Core-UI Separation) and Principle II (On-Demand Lifecycle).

## Complexity Tracking

No constitution violations to justify.
