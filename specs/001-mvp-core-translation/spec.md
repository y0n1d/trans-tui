# Feature Specification: MVP Core Translation

**Feature Branch**: `001-mvp-core-translation`

**Created**: 2026-09-15

**Status**: Draft

**Input**: User description: "基于项目根目录的《my-trans：轻量级 Wayland - Niri 终端翻译工具完整需求规格.md》，实现 my-trans 的第一个可独立运行 MVP。MVP 聚焦建立最小完整闭环：用户通过 CLI 提供文本，Core 调用可替换的 Translator Provider 完成翻译，结果显示在 Bubble Tea TUI 中；TUI 运行于 foot，通过 Unix Domain Socket 实现后续 CLI 调用复用已有 TUI/Server，而不是创建新的窗口；翻译结果追加到内存历史并支持基本滚动和错误处理。MVP 使用 Go、Bubble Tea、Lip Gloss、Bubbles，并遵守 .specify/memory/constitution.md 和 AGENTS.md。暂不实现 OCR、截图、TTS、复杂 Provider 管理等后续功能，但架构应为这些功能保留清晰扩展点。请只生成 specification，不要编写代码、plan 或 tasks。"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Direct Text Translation (Priority: P1)

A user opens a terminal and runs the translation command with a text argument. The system starts a Server and opens a terminal UI window. The Server processes the translation request and displays the result in the TUI. The TUI remains open for subsequent translations.

**Why this priority**: This is the fundamental value proposition — without the ability to translate text and see results, no other feature matters. This establishes the minimal closed loop: CLI → IPC → Core → Provider → TUI.

**Independent Test**: Can be fully tested by running the command with a text argument and verifying the TUI window opens with the translated result displayed.

**Acceptance Scenarios**:

1. **Given** no prior translation session exists, **When** the user runs the command with a text argument, **Then** a Server starts, a terminal window opens with the TUI, and the original text and its translation are displayed.
2. **Given** the translation provider is configured, **When** the user submits text, **Then** the Server calls the provider and displays the translated result in the TUI within a reasonable time.
3. **Given** the translation succeeds, **When** the result is displayed, **Then** the original text and translation are both visible in the interface, and the TUI remains open for further use.

---

### User Story 2 - Single Instance Reuse (Priority: P1)

A user runs the translation command a second time while a translation window is already open. Instead of opening a second window, the new text is sent to the existing window and the translation is appended to the history.

**Why this priority**: The single-instance behavior is a core architectural requirement. Without it, each invocation would create a new window, breaking the intended user experience of a persistent translation workspace.

**Independent Test**: Can be tested by running the command twice in succession and verifying only one terminal window exists, with both translations visible.

**Acceptance Scenarios**:

1. **Given** a translation window is already open, **When** the user runs the command with new text, **Then** no new window is created and the new translation appears in the existing window.
2. **Given** multiple translations have been submitted, **When** the user views the window, **Then** all translations are listed in chronological order.
3. **Given** the existing session's server is running, **When** a new translation request arrives, **Then** the request is processed and the result is appended to the history without interrupting existing content.

---

### User Story 3 - History Browsing (Priority: P2)

A user with multiple translations in the window navigates through the history using keyboard keys and mouse scroll to review previous translations.

**Why this priority**: Users need to review past translations. This is essential for the tool to be useful beyond a single lookup.

**Independent Test**: Can be tested by submitting multiple translations and then scrolling up/down to verify all entries are accessible.

**Acceptance Scenarios**:

1. **Given** the window contains multiple translations, **When** the user scrolls down, **Then** the view moves toward the most recent translation.
2. **Given** the window contains multiple translations, **When** the user scrolls up, **Then** the view moves toward older translations.
3. **Given** the window contains many translations that exceed the visible area, **When** the user uses page navigation, **Then** the view jumps by a full page of content.
4. **Given** the user has scrolled away from the bottom, **When** a new translation completes, **Then** the view automatically scrolls to show the newest entry.

---

### User Story 4 - Error Handling and Retry (Priority: P2)

A user submits a translation that fails due to network issues or provider errors. The system displays a clear error message and allows the user to retry the failed translation.

**Why this priority**: Translation failures are inevitable (network issues, provider downtime). Without retry capability, the user must manually re-enter the text.

**Independent Test**: Can be tested by simulating a provider failure and verifying the error message appears and retry succeeds after the issue is resolved.

**Acceptance Scenarios**:

1. **Given** a translation request fails, **When** the error is displayed, **Then** the user sees a human-readable error message explaining what went wrong.
2. **Given** a translation has failed, **When** the user triggers a retry, **Then** the same text is re-submitted to the provider.
3. **Given** a translation is in progress, **When** the user submits a new request, **Then** the previous request is cancelled and the new one takes priority.

---

### Edge Cases

- What happens when the translation provider returns an empty result? The system displays a message indicating no translation was returned and allows retry.
- What happens when the user submits an extremely long text (e.g., 10,000 characters)? The system processes it but may display a warning about response time.
- What happens when the socket file exists but the server is not running (stale socket)? The system detects the stale socket, removes it, and starts a fresh server.
- What happens when the user closes the terminal window while a translation is in progress? The server shuts down gracefully, cancelling any pending requests.
- What happens when the provider API key is not set? The system displays a clear error message indicating the key is missing.
- What happens when the network is unavailable? The system displays a connection error and allows retry.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept text input via a command-line argument and display the translation in a terminal interface.
- **FR-002**: System MUST support a pluggable translation provider interface so that concrete provider implementations can be swapped via configuration without modifying core logic.
- **FR-003**: System MUST display both the original text and the translated result in the terminal interface.
- **FR-004**: System MUST append each new translation to an in-memory history list, never overwriting previous entries.
- **FR-005**: System MUST allow the user to scroll through translation history using keyboard navigation (up, down, page up, page down, home, end).
- **FR-006**: System MUST allow the user to scroll through translation history using mouse wheel.
- **FR-007**: System MUST automatically scroll to the latest translation after a new result arrives.
- **FR-008**: System MUST run as a single instance — only one terminal window may exist at any time. Subsequent invocations MUST send requests to the existing server via local IPC.
- **FR-009**: System MUST use a local socket for communication between the CLI and the running server.
- **FR-010**: System MUST detect and clean up stale socket files from previous abnormal exits before starting a new server.
- **FR-011**: System MUST exit the server and clean up the socket when the terminal window is closed.
- **FR-012**: System MUST display user-friendly error messages for translation failures, network errors, and missing configuration.
- **FR-013**: System MUST allow the user to retry the most recent failed translation without re-entering the text. Retry applies only to the last failed request, not a task queue.
- **FR-014**: System MUST cancel the previous in-progress translation when a new request arrives, prioritizing the latest user intent.
- **FR-015**: System MUST read API keys from environment variables only — never from configuration files, source code, or logs.
- **FR-016**: System MUST NOT create a persistent background service or daemon. The server exists only while the terminal window is open.
- **FR-017**: System MUST support configurable source and target languages with automatic detection as the default for source language.

### Key Entities

- **TranslationRecord**: Represents a single translation entry. Contains the original text, translated text, source language, target language, provider used, and timestamp.
- **TranslationRequest**: Represents a request to translate text. Contains the text to translate, source language, target language, and input source (argument, clipboard, or selection).
- **TranslationResult**: Represents the outcome of a translation attempt. Contains the translated text or error information, along with metadata about the provider and timing.
- **AppState**: Represents the current state of the terminal interface. Contains the list of translation records, the current scroll position, and loading/error state.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can translate a text string and see the result in the terminal interface within 5 seconds for typical short texts (under 500 characters).
- **SC-002**: Submitting a second translation while a window is open adds the result to the existing window without creating a new window, verified 100% of the time.
- **SC-003**: Users can scroll through at least 50 translation entries without noticeable lag or interface corruption.
- **SC-004**: When a translation fails, the user sees an actionable error message within 2 seconds and can retry with a single action.
- **SC-005**: The server process starts and stops cleanly with the terminal window — no orphaned processes remain after the window is closed, verified 100% of the time.
- **SC-006**: API keys are never visible in configuration files, logs, error messages, or the terminal interface.

## Assumptions

- The user is running a Linux system with a Wayland compositor (Niri) and the foot terminal emulator.
- The user has access to a translation provider API that accepts text and returns translations (initially an OpenAI-compatible API).
- The user has an API key for the translation provider, stored as an environment variable.
- The target language defaults to Chinese (zh) and the source language defaults to automatic detection.
- The terminal interface is displayed in a foot window managed by the Niri compositor.
- This MVP does not include OCR, screenshot, TTS, clipboard/selection input, or complex provider management — these are deferred to future phases.
- The architecture preserves extension points for OCR, TTS, clipboard/selection, and additional translation providers through interface-based design.
- History is stored in memory only and is not persisted to disk in this MVP.

## Clarifications

### Session 2026-09-15

- Q: When the user runs `my-trans "Hello"`, should the system print to terminal and exit, or launch the TUI? → A: Always launch TUI. First invocation starts Server + TUI, CLI sends request to Server, TUI persists. Subsequent invocations detect socket and send to existing Server.
- Q: Should clipboard/selection input be part of MVP Phase 1? → A: No. Remove from MVP. Phase 1 only: `my-trans "Hello world"` → CLI → IPC → Core → Translator → TUI → foot. Clipboard/selection deferred to Phase 2.
- Q: Should FR-020 (display provider name) be in MVP? → A: No. Remove FR-020. Keep FR-004 (pluggable interface) as architecture boundary. MVP only needs Translator interface + one OpenAI-compatible implementation.
- Q: What is the scope of retry? → A: Retry applies only to the most recent failed TranslationRequest. No task queue. TUI retries the last failed request.
- Q: What happens with very long text input? → A: No hard limit in the system. Let the provider handle rejection. The TUI shows a loading state and displays whatever error the provider returns.
- Q: Should the error message clear when retry is triggered? → A: Yes. Clear the error message immediately when retry is triggered and show the loading state instead.
