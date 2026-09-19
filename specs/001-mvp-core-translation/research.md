# Research: MVP Core Translation

**Date**: 2026-09-15
**Feature**: 001-mvp-core-translation

## R1: IPC Protocol Design

**Decision**: Versioned JSON over Unix Domain Socket with length-prefixed framing.

**Rationale**: JSON is human-readable for debugging. Version field allows backward-compatible evolution. Length-prefix framing avoids delimiter issues with arbitrary text content. Unix socket provides low-latency local-only communication.

**Alternatives considered**:
- gRPC: Rejected — heavyweight dependency for a single-user desktop tool.
- Raw strings: Rejected — no structured error handling or versioning.
- TCP with localhost: Rejected — unnecessary network stack; Constitution requires Unix socket.

**Socket path**: `$XDG_RUNTIME_DIR/trans-tui.sock`

**Message format**:
```json
{
  "version": 1,
  "type": "translate",
  "request_id": "uuid",
  "text": "Hello world",
  "source_lang": "auto",
  "target_lang": "zh"
}
```

**Response format**:
```json
{
  "version": 1,
  "request_id": "uuid",
  "ok": true,
  "translation": "你好，世界",
  "provider": "openai-compatible",
  "error": null
}
```

## R2: Stale Socket Detection

**Decision**: Try to connect to existing socket. If connection fails, remove the stale file and create a new server.

**Rationale**: The Constitution (Principle III) requires single instance. A stale socket from a previous crash would block new instances. Connection attempt is the only reliable way to distinguish a live server from a stale file.

**Alternatives considered**:
- PID file: Rejected — PID reuse is possible; additional complexity for cleanup.
- Lock file with `flock`: Rejected — Unix socket connection test is simpler and already needed for the "is server running?" check.

## R3: Server Lifecycle and foot Integration

**Decision**: The CLI process starts the Server as an in-process goroutine, then launches foot with the TUI. The Server listens on the socket. When foot closes, the Server detects the disconnection (stdin EOF or signal) and shuts down.

**Rationale**: Constitution Principle II requires no daemon. The Server exists only while the TUI is active. Using an in-process goroutine (rather than a separate server process) simplifies lifecycle management — one process, one foot window.

**Flow**:
1. CLI detects no socket → starts Server goroutine → starts foot → sends initial request
2. CLI detects existing socket → sends request via client → exits
3. foot closes → Server receives signal/EOF → cleans up socket → exits

**Alternatives considered**:
- Separate server process: Rejected — adds process management complexity; harder to guarantee clean shutdown.
- foot as child process with process group: Considered, but in-process goroutine is simpler for MVP.

## R4: TUI Scroll Behavior

**Decision**: Bubble Tea viewport component handles scrolling. Keyboard (↑↓, PageUp/PageDown, Home/End) and mouse wheel events are mapped to viewport scroll commands. Auto-scroll to bottom on new translation arrival.

**Rationale**: Bubbles provides a mature `viewport` component that handles scroll math, content overflow, and mouse events. Reimplementing would be unnecessary complexity.

**Auto-scroll rule**: If the user was at the bottom before the new entry arrived, stay at the bottom. If the user had scrolled up, remain at their position (do not force-scroll).

## R5: Error Display and Retry

**Decision**: Error messages are stored in AppState. The TUI view renders errors in a styled error block. Retry (key: `r`) clears the error, shows a loading state, and re-submits the last failed request.

**Rationale**: Spec Clarification Q6 confirmed: clear error on retry, show loading. This gives clean visual feedback.

**Error categories and messages**:
- Missing API key: "API key not set. Export TRANSLATION_API_KEY environment variable."
- Network error: "Connection failed. Check your network and retry."
- Provider error: "Translation failed: <provider error message>"
- Empty result: "No translation returned. Retry or check your input."

## R6: Configuration

**Decision**: TOML config file auto-discovered via `os.UserConfigDir()` (Linux: `$XDG_CONFIG_HOME/trans-tui/config.toml`). On first run without `-c/--config`, a commented template is created automatically. Explicit `-c/--config` overrides auto-discovery. API key referenced by environment variable name, not stored directly.

**Example**:
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

**Rationale**: Constitution Principle V requires API keys via environment variables only. The config stores the env var name (`api_key_env`), and the program reads it via `os.Getenv()`. Auto-discovery (Principle VII) eliminates the need for users to pass `-c` on every invocation.

## R7: Provider Interface Contract

**Decision**: Minimal interface with single Translate method.

```go
type Translator interface {
    Translate(ctx context.Context, req TranslationRequest) (TranslationResult, error)
}
```

**Rationale**: Constitution Principle IV requires interface-based provider extensibility. This minimal contract allows future providers (DeepSeek, Gemini, etc.) to be added by implementing one method.

## R8: foot Window Management

**Decision**: The program launches foot with `foot --app-id=trans-tui`. Window positioning and sizing are handled by Niri via window rules, not by the program.

**Rationale**: Constitution and spec both state that window management is Niri's responsibility. The program only needs to set the correct `app-id`.

**foot launch command**: `foot --app-id=trans-tui` (starts foot running the TUI binary or a shell that runs the TUI)

**Alternative**: Use `foot` with a custom foot.ini section. Not needed for MVP — default foot config is sufficient.
