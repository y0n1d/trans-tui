#!/bin/sh
# input-translate.sh — Open input translation window
#
# Usage: called from Niri keybinding

. "$(dirname "$0")/trans-env.sh"
exec foot --title=Translate sh -c 'trans-tui -i'
