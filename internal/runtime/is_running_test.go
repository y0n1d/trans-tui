package runtime

import (
	"context"
	"github.com/y0n1d/trans-tui/internal/config"
	"github.com/y0n1d/trans-tui/internal/ipc"
	"testing"
	"time"
)

func TestIsRunning_NoServer(t *testing.T) {
	socketPath := tempSocketPath(t)
	cfg := config.Config{
		SocketPath: socketPath,
	}
	if IsRunning(cfg) {
		t.Error("IsRunning should return false when no server is listening")
	}
}

func TestIsRunning_ServerAlive(t *testing.T) {
	socketPath := tempSocketPath(t)
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "TEST_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.test.com",
				Model:   "test-model",
			},
		},
		SocketPath: socketPath,
	}

	fp := cfg.Fingerprint()
	handler := func(ctx context.Context, req ipc.Request) ipc.Response {
		if req.Type == "status" {
			return ipc.Response{
				Version:     ipc.ProtocolVersion,
				RequestID:   req.RequestID,
				OK:          true,
				Translation: fp,
			}
		}
		return ipc.Response{
			Version:   ipc.ProtocolVersion,
			RequestID: req.RequestID,
			OK:        false,
			Error:     "unexpected request type",
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)
	defer cancel()

	if !IsRunning(cfg) {
		t.Error("IsRunning should return true when server is listening")
	}
}

func TestIsRunning_AfterServerStops(t *testing.T) {
	socketPath := tempSocketPath(t)
	cfg := config.Config{
		Provider: config.ProviderConfig{
			Type:      "openai-compatible",
			APIKeyEnv: "TEST_KEY",
			Timeout:   30,
			OpenAI: config.OpenAIConfig{
				BaseURL: "https://api.test.com",
				Model:   "test-model",
			},
		},
		SocketPath: socketPath,
	}

	fp := cfg.Fingerprint()
	handler := func(ctx context.Context, req ipc.Request) ipc.Response {
		return ipc.Response{
			Version:     ipc.ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: fp,
		}
	}

	srv := ipc.NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())

	// Listen is synchronous: once it returns, the socket exists and the
	// kernel accepts connections, so no readiness sleep is needed.
	if err := srv.Listen(ctx); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	defer cancel()

	if !IsRunning(cfg) {
		t.Fatal("IsRunning should return true while server is running")
	}

	// Stop the server. Serve returns only after its shutdown cleanup has
	// closed the listener and removed the socket file, so that return — not
	// a sleep — is the sync point for "server stopped".
	cancel()
	select {
	case <-serveErr:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancellation: server did not shut down")
	}

	if IsRunning(cfg) {
		t.Error("IsRunning should return false after server stops")
	}
}
