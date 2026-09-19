# trans-tui Constitution

## Core Principles

### I. Core-UI Separation

Core business logic MUST NOT depend on Bubble Tea or any TUI framework.
UI depends on Core interfaces, not the reverse. Core modules (translation, OCR, TTS,
IPC, config) MUST remain independently testable and replaceable.
Future UI replacements (GTK, Web, other TUI) MUST NOT require rewriting core logic.

### II. On-Demand Lifecycle

No process runs when the tool is idle. The Server exists only while the TUI is active.
When the TUI closes, the Server MUST exit and clean up the socket. No systemd service,
no daemon, no boot-time startup. Resources are allocated only on demand and released
immediately after use.

### III. Single Instance, State Continuity

Only one window instance MAY exist at any time. Subsequent invocations MUST detect the
existing socket and send requests to the running Server rather than spawning a new
instance. Translations MUST append to history, never overwrite. State persists across
requests within a session and is released entirely when the session ends.

### IV. Provider Extensibility

Core depends only on Provider interfaces for translation, OCR, and TTS.
Concrete implementations belong to the Infrastructure layer and MUST NOT leak into Core.
Adding or swapping a provider MUST require only configuration changes, not code
modifications to Core.

### V. Security by Environment Variables

API keys MUST be provided via environment variables only. They MUST NOT appear in
config files, source code, git commits, logs, error messages, or TUI output.
The TOML config stores only the environment variable name, never the secret itself.

### VI. Stable Contracts for Parallel Development

Public interfaces, shared data structures, IPC schemas, and provider contracts
MUST be treated as shared contracts between modules.
Agents MUST NOT silently change shared contracts when implementing an isolated task.
Contract changes MUST be reflected in the relevant specification, plan, and tasks,
and affected modules MUST be updated together.
Independent modules SHOULD be developed in isolated Git branches or worktrees.

## Platform Constraints

- **Target**: Arch Linux, Wayland, Niri compositor.
- **Window**: foot terminal with `app-id = "trans-tui"`.
- **IPC**: Unix Domain Socket at `$XDG_RUNTIME_DIR/trans-tui.sock`.
- **Clipboard**: `wl-paste` / `wl-copy` only. No X11 clipboard tools.
- **Screenshot**: `grim` + `slurp` only. No custom screenshot protocol.
- **TTS**: `espeak-ng` via `os/exec`. No persistent TTS daemon.
- **No X11 dependencies**. Wayland only.

## Development Workflow

- **Language**: Go with Bubble Tea, Lip Gloss, Bubbles for TUI.
- **MVP First**: Phase 1 covers text translation, OpenAI-compatible provider,
  clipboard, selection, TUI, socket IPC, single instance, history, scroll, copy, retry.
- **Incremental**: Each phase MUST be independently runnable. No half-built features.
- **Testing**: `go test ./...` and `go vet ./...` before each commit.
- **Error Handling**: Errors MUST be layered and user-friendly. No panics in production paths.
- **Concurrency**: Use `context.Context` and goroutines. New requests MAY cancel previous ones.
- **Network**: All HTTP requests MUST support timeout and context cancellation.

## Governance

This constitution is the authoritative reference for architectural decisions.
All code reviews MUST verify compliance with these principles.
Amendments require documentation, approval, and a migration plan.
Complexity MUST be justified against simplicity principles.

**Version**: 1.1.0 | **Ratified**: 2026-09-15 | **Last Amended**: 2026-09-15
