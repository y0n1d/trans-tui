package ipc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
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

// TestServerSocketCleanup proves the graceful-shutdown lifecycle: while the
// server runs the socket file exists, and once the context is cancelled
// Serve returns only after its cleanup has removed the socket file. Serve's
// return — not a sleep — is the sync point. Serve's error value is
// deliberately not asserted: cleanup()'s os.Remove reports "no such file"
// because net.UnixListener already unlinked the socket on Close, a
// pre-existing condition swallowed in production (see the concurrency tests).
func TestServerSocketCleanup(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}

	// startTestServer listens synchronously: the socket file exists by the
	// time it returns, with no readiness sleep.
	cancel, serveErr := startTestServer(t, socketPath, handler, 0)
	defer cancel()

	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatal("socket file should exist while server is running")
	}

	cancel()

	select {
	case <-serveErr:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation: no graceful shutdown")
	}

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

// TestServerEOFConnectionNoErrorLog covers a client that disconnects without
// sending a request — exactly what runtime's isAlive/--check-running does on
// every launcher invocation. handleConnection must treat that EOF as normal,
// log nothing, skip the handler, and leave the serve loop running. The sync
// point is deterministic: the server closes its side of the connection only
// after the read-error branch finished, so observing EOF here means the log
// decision has already been made.
func TestServerEOFConnectionNoErrorLog(t *testing.T) {
	socketPath := tempSocketPath(t)

	handlerCalled := make(chan struct{}, 1)
	handler := func(ctx context.Context, req Request) Response {
		handlerCalled <- struct{}{}
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}

	cancel, _ := startTestServer(t, socketPath, handler, 0)
	// Cleanup only: broken shutdown is caught by TestServerSocketCleanup,
	// so this defer must never wait on Serve's return.
	defer cancel()

	// Capture the standard logger around the disconnect window.
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		t.Fatalf("connection is %T, want *net.UnixConn", conn)
	}
	// Close only the write side: the server's read gets io.EOF while the
	// read side stays open so the test can observe the server closing.
	if err := uc.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite: %v", err)
	}
	defer conn.Close()

	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 8)
		_, err := conn.Read(buf)
		readDone <- err
	}()

	select {
	case err := <-readDone:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("read after client EOF = %v, want io.EOF (server closed the connection)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never closed the connection whose client sent no request")
	}

	// The server has fully handled the EOF: no error may have been logged
	// for it, and the handler must not have run.
	if got := logBuf.String(); strings.Contains(got, "read request error") {
		t.Errorf("client EOF was logged as an error: %q", got)
	}
	select {
	case <-handlerCalled:
		t.Error("handler ran for a connection that never sent a request")
	default:
	}

	// The EOF must not have torn the server down: the next request on a
	// fresh connection still succeeds (and Serve is still accepting).
	resp, err := SendRequest(socketPath, Request{
		Version:   ProtocolVersion,
		Type:      TypeStatus,
		RequestID: "after-eof",
	})
	if err != nil {
		t.Fatalf("request after EOF: %v", err)
	}
	if !resp.OK || resp.RequestID != "after-eof" {
		t.Errorf("response after EOF = (OK=%v, id=%q), want (true, %q)", resp.OK, resp.RequestID, "after-eof")
	}
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

// ---------------------------------------------------------------------------
// Capability tests
// ---------------------------------------------------------------------------

func TestServerStatusReturnsCapabilities(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		if req.Type == TypeStatus {
			return Response{
				Version:      ProtocolVersion,
				RequestID:    req.RequestID,
				OK:           true,
				Translation:  "fingerprint",
				Capabilities: []string{CapDisplayText},
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

	resp, err := SendRequest(socketPath, Request{
		Version:   ProtocolVersion,
		Type:      TypeStatus,
		RequestID: "cap-001",
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true")
	}
	if len(resp.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(resp.Capabilities))
	}
	if resp.Capabilities[0] != CapDisplayText {
		t.Errorf("capability = %q, want %q", resp.Capabilities[0], CapDisplayText)
	}
}

func TestServerStatusWithoutCapabilities(t *testing.T) {
	socketPath := tempSocketPath(t)

	// Simulate old server that doesn't set capabilities.
	handler := func(ctx context.Context, req Request) Response {
		if req.Type == TypeStatus {
			return Response{
				Version:     ProtocolVersion,
				RequestID:   req.RequestID,
				OK:          true,
				Translation: "fingerprint",
				// No Capabilities field — like an old server.
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

	resp, err := SendRequest(socketPath, Request{
		Version:   ProtocolVersion,
		Type:      TypeStatus,
		RequestID: "cap-002",
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true")
	}
	if len(resp.Capabilities) != 0 {
		t.Errorf("expected 0 capabilities from old server, got %d", len(resp.Capabilities))
	}
}

func TestServerDisplayTextWorks(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		if req.Type == TypeStatus {
			return Response{
				Version:      ProtocolVersion,
				RequestID:    req.RequestID,
				OK:           true,
				Translation:  "fingerprint",
				Capabilities: []string{CapDisplayText},
			}
		}
		if req.Type == TypeDisplayText {
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

	resp, err := SendRequest(socketPath, Request{
		Version:   ProtocolVersion,
		Type:      TypeDisplayText,
		RequestID: "display-001",
		Text:      "OCR result",
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true, got error: %s", resp.Error)
	}
	if resp.RequestID != "display-001" {
		t.Errorf("request_id = %q, want %q", resp.RequestID, "display-001")
	}
}

func TestServerTranslateStillWorksWithCapabilities(t *testing.T) {
	socketPath := tempSocketPath(t)

	handler := func(ctx context.Context, req Request) Response {
		if req.Type == TypeStatus {
			return Response{
				Version:      ProtocolVersion,
				RequestID:    req.RequestID,
				OK:           true,
				Translation:  "fingerprint",
				Capabilities: []string{CapDisplayText},
			}
		}
		return Response{
			Version:     ProtocolVersion,
			RequestID:   req.RequestID,
			OK:          true,
			Translation: "translated: " + req.Text,
			Provider:    "test",
			Model:       "test",
		}
	}

	srv := NewServer(socketPath, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.ListenAndServe(ctx)
	time.Sleep(50 * time.Millisecond)

	// Translate request should still work normally.
	resp, err := SendRequest(socketPath, Request{
		Version:    ProtocolVersion,
		Type:       TypeTranslate,
		RequestID:  "trans-001",
		Text:       "hello",
		SourceLang: "en",
		TargetLang: "fr",
	})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true")
	}
	if resp.Translation != "translated: hello" {
		t.Errorf("translation = %q, want %q", resp.Translation, "translated: hello")
	}
}
