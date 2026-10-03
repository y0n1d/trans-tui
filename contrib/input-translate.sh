#!/bin/sh
# input-translate.sh — Open input translation window
#
# If a trans-tui server is already running, send -i directly via IPC
# (no new foot window). Otherwise open a dedicated footclient window.
#
# Usage: called from Niri keybinding

. "$(dirname "$0")/trans-env.sh"

if trans-tui --check-running >/dev/null 2>&1; then
    exec trans-tui -i
fi

# Release Ctrl+Shift+C only in this dedicated trans-tui footclient window.
# Normal foot windows keep their configured clipboard-copy binding unchanged.
exec footclient -E --override key-bindings.clipboard-copy=none --title=Translate sh -c 'trans-tui -i'
