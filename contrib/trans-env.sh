#!/bin/sh
# trans-env.sh — Shared environment for trans-tui Niri integration scripts.
#
# Provides:
#   1. PATH including ~/go/bin (where trans-tui / trans-ocr are installed)
#   2. API keys from existing ~/.config/.env.local
#
# Usage: source this file at the top of each Niri script.
#   . "$(dirname "$0")/trans-env.sh"

# Ensure ~/go/bin is in PATH.
export PATH="$HOME/go/bin:$PATH"

# Load API keys from existing .env.local (zsh-compatible format).
# Never log, print, or commit these values.
for _env_file in \
    "$HOME/.config/zsh/.env.local" \
    "$HOME/.config/.env.local"; do
    if [ -r "$_env_file" ]; then
        . "$_env_file"
        break
    fi
done
unset _env_file
