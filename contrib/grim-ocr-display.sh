#!/bin/sh
# grim-ocr-display.sh — Screenshot → OCR → Display (no translation)
#
# Stage 1: slurp + grim (no foot window)
# Stage 2: foot --title=Translate with trans-ocr | trans-tui --display
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

# Stage 2: open foot only after successful capture
foot --title=Translate sh -c "cat \"$IMG\" | trans-ocr - | trans-tui --display"
