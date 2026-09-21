#!/bin/sh
# input-translate.sh — Open input translation window
#
# Usage: called from Niri keybinding

. "$(dirname "$0")/trans-env.sh"
# Release Ctrl+Shift+C only in this dedicated trans-tui foot instance. Normal
# foot windows keep their configured clipboard-copy binding unchanged.
exec foot --override key-bindings.clipboard-copy=none --title=Translate sh -c 'trans-tui -i'
