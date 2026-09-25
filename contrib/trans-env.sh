#!/bin/sh
# trans-env.sh — Shared environment for trans-tui Niri integration scripts.
#
# Provides:
#   1. PATH including ~/go/bin (where trans-tui / trans-ocr are installed)
#   2. API keys from existing ~/.config/.env.local
#   3. HTTP(S) proxy env for trans-tui (HTTP_PROXY / HTTPS_PROXY / NO_PROXY)
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

# HTTP(S) proxy for trans-tui API requests. Host/port are the same values as
# the proxy1 helper in ~/.config/zsh/proxy.zsh — reuse them, don't redesign.
# Go's net/http reads exactly HTTP_PROXY, HTTPS_PROXY and NO_PROXY, so no
# ALL_PROXY / SOCKS5 is set here.
PROXY_HOST="${PROXY_HOST:-127.0.0.1}"
PROXY_PORT="${PROXY_PORT:-7897}"

export HTTP_PROXY="http://${PROXY_HOST}:${PROXY_PORT}"
export HTTPS_PROXY="http://${PROXY_HOST}:${PROXY_PORT}"
export NO_PROXY="localhost,127.0.0.1,::1"
