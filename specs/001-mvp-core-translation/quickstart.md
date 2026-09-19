# Quickstart Validation Guide: MVP Core Translation

**Date**: 2026-09-15
**Feature**: 001-mvp-core-translation

## Prerequisites

- Linux system with Wayland (Niri compositor)
- foot terminal emulator installed
- Go 1.22+ installed
- A translation provider API key (e.g., OpenAI API key)
- Environment variable `TRANSLATION_API_KEY` set

```bash
export TRANSLATION_API_KEY="your-api-key-here"
```

## Build

```bash
go build -o trans-tui ./cmd/trans-tui
```

## Validation Scenarios

### Scenario 1: First Translation (P1 — Direct Text Translation)

**Goal**: Verify the core loop works end-to-end.

```bash
./trans-tui "Hello world"
```

**Expected**:
1. foot window opens with app-id `trans-tui`
2. TUI displays the original text "Hello world"
3. TUI displays the Chinese translation (e.g., "你好，世界")
4. TUI remains open (does not exit)
5. Status bar shows "1 record"

### Scenario 2: Single Instance Reuse (P1)

**Goal**: Verify second invocation reuses existing TUI.

```bash
# Terminal 1:
./trans-tui "Hello world"
# (TUI opens)

# Terminal 2 (while TUI is still open):
./trans-tui "How are you?"
```

**Expected**:
1. No second foot window opens
2. "How are you?" translation appears in the existing TUI
3. Status bar shows "2 records"
4. Both translations visible in chronological order

### Scenario 3: History Scrolling (P2)

**Goal**: Verify keyboard and mouse scrolling.

```bash
# Submit multiple translations:
./trans-tui "One"
./trans-tui "Two"
./trans-tui "Three"
./trans-tui "Four"
./trans-tui "Five"
```

**Expected**:
1. All 5 translations visible
2. Pressing ↑ scrolls up to older entries
3. Pressing ↓ scrolls down to newer entries
4. PageUp jumps by a full page
5. Home goes to the first entry
6. End goes to the last entry
7. Mouse wheel scrolls up/down

### Scenario 4: Error Handling and Retry (P2)

**Goal**: Verify error display and retry mechanism.

```bash
# Unset API key:
unset TRANSLATION_API_KEY
./trans-tui "Hello"
```

**Expected**:
1. TUI opens
2. Error message displayed: "API key not set..."
3. Press `r` to retry
4. Error clears, loading state shown
5. Same error reappears (key still unset)
6. Set API key, press `r` again
7. Translation succeeds

### Scenario 5: Stale Socket Recovery (Edge Case)

**Goal**: Verify clean startup after abnormal exit.

```bash
# Create a stale socket:
touch $XDG_RUNTIME_DIR/trans-tui.sock

# Start the program:
./trans-tui "Hello"
```

**Expected**:
1. Stale socket detected and removed
2. New server starts normally
3. TUI opens with translation result

### Scenario 6: Server Cleanup on TUI Close (P1)

**Goal**: Verify no orphaned processes or socket files.

```bash
./trans-tui "Hello"
# (TUI opens)
# Close the foot window (Ctrl+D or close window manager)

# Verify:
ls $XDG_RUNTIME_DIR/trans-tui.sock 2>&1
# Expected: "No such file or directory"

pgrep trans-tui
# Expected: no output (no orphaned process)
```

### Scenario 7: Stdin Input (P1)

**Goal**: Verify piped stdin is read and translated.

```bash
echo "Hello world" | ./trans-tui
```

**Expected**:
1. TUI opens with "Hello world" translation
2. No second TUI window
3. Works identically to `./trans-tui "Hello world"`

```bash
printf 'line one\nline two\nline three\n' | ./trans-tui
```

**Expected**:
1. Multi-line text is preserved
2. Full content is translated

```bash
./trans-tui
```

**Expected**:
1. In interactive terminal: shows usage error (no blocking on stdin)
2. With piped empty input: shows usage error

## Success Criteria Validation

| Criterion | How to Validate                                |
|-----------|------------------------------------------------|
| SC-001    | Scenario 1: time the translation, confirm <5s  |
| SC-002    | Scenario 2: verify one window, two records     |
| SC-003    | Scenario 3: submit 50 translations, scroll     |
| SC-004    | Scenario 4: error appears, retry works         |
| SC-005    | Scenario 6: no orphaned processes/socket       |
| SC-006    | Scenario 4: key not in error message           |
