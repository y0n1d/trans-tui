#!/bin/sh
# grim-ocr-display.sh — Screenshot → OCR → Display (no translation)
#
# Stage 1: slurp + grim (no foot window)
# Stage 2: If trans-tui server is running, pipe directly via IPC.
#          Otherwise open a dedicated foot instance.
#
# Usage: called from Niri keybinding

. "$(dirname "$0")/trans-env.sh"

set -e

TMPDIR="${XDG_RUNTIME_DIR:-/tmp}/trans-tui"
mkdir -p "$TMPDIR"
IMG="$(mktemp "$TMPDIR/screenshot-XXXXXX.png")"
trap 'rm -f "$IMG"' EXIT

# Stage 1: capture region to file (no foot window)
GEOM="$(slurp)" || exit 0  # slurp cancelled → exit silently
grim -g "$GEOM" - > "$IMG"

# Stage 2: OCR → display, reusing existing TUI if possible
if trans-tui --check-running >/dev/null 2>&1; then
    cat "$IMG" | trans-ocr - | trans-tui --display
else
    foot --override key-bindings.clipboard-copy=none --title=Translate sh -c "cat \"$IMG\" | trans-ocr - | trans-tui --display"
fi
