package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.sock")
}

func TestServerClientRoundTrip(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:     ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: "translated: " + req.Text,
			Provider:    "test-provider",
			Model:       "test-model",
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	req := Request{
		Version:    ProtocolVersion,
		Type:       "translate",
		RequestID:  "test-req-001",
		Text:       "Hello world",
		SourceLang: "en",
		TargetLang: "fr",
	}

	resp, err := SendRequest(socketPath, req)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if resp.RequestID != req.RequestID {
		t.Errorf("request_id = %q, want %q", resp.RequestID, req.RequestID)
	}
	if !resp.OK {
		t.Error("expected OK=true")
	}
	if resp.Translation != "translated: Hello world" {
		t.Errorf("translation = %q, want %q", resp.Translation, "translated: Hello world")
	}
	if resp.Provider != "test-provider" {
		t.Errorf("provider = %q, want %q", resp.Provider, "test-provider")
	}
	if resp.Model != "test-model" {
		t.Errorf("model = %q, want %q", resp.Model, "test-model")
	}

	cancel()
}

func TestServerErrorHandler(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        false,
			Error:     "provider unavailable",
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	req := Request{
		Version:    ProtocolVersion,
		Type:       "translate",
		RequestID:  "error-req-001",
		Text:       "fail",
		SourceLang: "en",
		TargetLang: "de",
	}

	resp, err := SendRequest(socketPath, req)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}

	if resp.OK {
		t.Error("expected OK=false")
	}
	if resp.Error != "provider unavailable" {
		t.Errorf("error = %q, want %q", resp.Error, "provider unavailable")
	}

	cancel()
}

func TestServerMultipleSequentialRequests(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:     ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: req.Text,
			Provider:    "test",
			Model:       "test",
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 3; i++ {
		req := Request{
			Version:    ProtocolVersion,
			Type:       "translate",
			RequestID:  "seq-req",
			Text:       "test",
			SourceLang: "en",
			TargetLang: "fr",
		}

		resp, err := SendRequest(socketPath, req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if !resp.OK {
			t.Errorf("request %d: expected OK=true", i)
		}
	}

	cancel()
}

func TestServerSocketCleanup(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatal("socket file should exist while server is running")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after shutdown")
	}
}

func TestClientConnectToNonexistentServer(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "nonexistent.sock")

	req := Request{
		Version:    ProtocolVersion,
		Type:       "translate",
		RequestID:  "no-server",
		Text:       "hello",
		SourceLang: "en",
		TargetLang: "fr",
	}

	_, err := SendRequest(socketPath, req)
	if err == nil {
		t.Fatal("expected error connecting to nonexistent server")
	}
}

func TestServerRequestIDEcho(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	ids := []string{"id-aaa", "id-bbb", "id-ccc"}
	for _, id := range ids {
		req := Request{
			Version:    ProtocolVersion,
			Type:       "translate",
			RequestID:  id,
			Text:       "hello",
			SourceLang: "en",
			TargetLang: "fr",
		}

		resp, err := SendRequest(socketPath, req)
		if err != nil {
			t.Fatalf("SendRequest(%q): %v", id, err)
		}
		if resp.RequestID != id {
			t.Errorf("response request_id = %q, want %q", resp.RequestID, id)
		}
	}

	cancel()
}

func TestServerEOFConnectionNoErrorLog(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()

	time.Sleep(50 * time.Millisecond)
}

func TestServerStatusRequest(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:     ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: "test-fingerprint",
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	req := Request{
		Version:   ProtocolVersion,
		Type:      "status",
		RequestID: "status-001",
	}

	resp, err := SendRequest(socketPath, req)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true")
	}
	if resp.Translation != "test-fingerprint" {
		t.Errorf("translation = %q, want %q", resp.Translation, "test-fingerprint")
	}

	cancel()
}

func TestServerEnterInputModeRequest(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		if req.Type == "enter_input_mode" {
			return Response{
				Version:   ProtocolVersion,
				RequestID: req.RequestID,
				OK:        true,
			}
		}
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        false,
			Error:     "unexpected type",
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	req := Request{
		Version:   ProtocolVersion,
		Type:      "enter_input_mode",
		RequestID: "input-001",
	}

	resp, err := SendRequest(socketPath, req)
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true, got error: %s", resp.Error)
	}
	if resp.RequestID != "input-001" {
		t.Errorf("request_id = %q, want %q", resp.RequestID, "input-001")
	}
}
