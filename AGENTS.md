# Project Instructions

## Architecture

Core must not depend on Bubble Tea.

UI must depend on Core interfaces.

Provider implementations must not leak into Core.

## Security

Never hardcode API keys.

Never store API keys in TOML.

Never log Authorization headers.

## Linux

Wayland only.

Do not introduce X11 dependencies.

## Runtime

Do not create a systemd service.

Do not create a permanent daemon.

## IPC

Use Unix Domain Socket.

Socket must live under XDG_RUNTIME_DIR.

## TUI

Use Bubble Tea.

Translations must append to history.

New translation automatically scrolls to bottom.

## Testing

Run:

go test ./...

go vet ./...
