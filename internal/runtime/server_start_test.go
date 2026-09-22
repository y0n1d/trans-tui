package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/ipc"
)

func echoHandler(ctx context.Context, req ipc.Request) ipc.Response {
	return ipc.Response{
		Version:     ipc.ProtocolVersion,
		RequestID:   req.RequestID,
		OK:          true,
		Translation: "fingerprint",
	}
}

// TestStartIPCServerReturnsListenFailure proves the runtime observes a failed
// bind instead of swallowing it: startIPCServer returns the error, so runServer
// can stop before a TUI pretends a server is running. Both failures are
// deterministic (no permissions, no sleeps, no foot).
func TestStartIPCServerReturnsListenFailure(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{
			// MkdirAll fails: the socket directory is a regular file.
			name: "socket directory unusable",
			path: func(t *testing.T) string {
				t.Helper()
				notADir := filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(notADir, "trans-tui.sock")
			},
		},
		{
			// net.Listen fails: the socket path is occupied by a non-empty
			// directory, so neither the stale-socket removal nor bind works.
			name: "net.Listen fails",
			path: func(t *testing.T) string {
				t.Helper()
				occupied := filepath.Join(t.TempDir(), "occupied")
				if err := os.Mkdir(occupied, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(occupied, "keep"), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				return occupied
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			socketPath := tc.path(t)
			cfg := config.Config{SocketPath: socketPath}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := startIPCServer(ctx, socketPath, echoHandler)
			if err == nil {
				t.Fatal("startIPCServer must return the listen error to the runtime")
			}

			if IsRunning(cfg) {
				t.Error("no server must be reachable after a listen failure")
			}
		})
	}
}

// TestStartIPCServerNormalStart covers the unchanged happy path: a successful
// bind makes the socket immediately visible, and cancelling the context stops
// serving. Bind and stat are synchronous, so no sleep is needed.
func TestStartIPCServerNormalStart(t *testing.T) {
	socketPath := tempSocketPath(t)
	cfg := config.Config{SocketPath: socketPath}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := startIPCServer(ctx, socketPath, echoHandler); err != nil {
		t.Fatalf("startIPCServer: %v", err)
	}
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("socket should exist after a successful start: %v", err)
	}
	if !IsRunning(cfg) {
		t.Error("server should be reachable after a successful start")
	}

	cancel()
}
