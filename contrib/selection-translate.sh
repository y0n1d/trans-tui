#!/bin/sh
# selection-translate.sh — Translate primary selection text
#
# If a trans-tui server is already running, pipe selection directly via IPC
# (no new foot window). Otherwise launch a dedicated foot instance.
#
# Usage: called from Niri keybinding

. "$(dirname "$0")/trans-env.sh"

set -e

TMPDIR="${XDG_RUNTIME_DIR:-/tmp}/trans-tui"
mkdir -p "$TMPDIR"
SEL="$(mktemp "$TMPDIR/selection-XXXXXX.txt")"
trap 'rm -f "$SEL"' EXIT

wl-paste --primary --type "text/plain;charset=utf-8" --no-newline > "$SEL" 2>/dev/null || exit 0
[ -s "$SEL" ] || exit 0

if trans-tui --check-running >/dev/null 2>&1; then
    cat "$SEL" | trans-tui
else
    foot --override key-bindings.clipboard-copy=none --title=Translate sh -c "cat \"$SEL\" | trans-tui"
fi
