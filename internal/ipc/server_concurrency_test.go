package ipc

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// These tests cover P1-1: Serve() must handle each accepted connection
// independently so one blocked handler cannot stall other IPC clients, and
// both sides of the exchange must carry deadlines. Blocking and release are
// driven purely by test-handler channels — no ordering sleeps, no foot, no
// TUI, no real provider.

type sendResult struct {
	resp Response
	err  error
}

func startTestServer(t *testing.T, socketPath string, handler Handler, connDeadline time.Duration) (context.CancelFunc, <-chan error) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	srv := NewServer(socketPath, handler)
	srv.connDeadline = connDeadline

	// Listen runs synchronously: once it returns, the socket exists and the
	// kernel accepts connections into the backlog, so clients can dial
	// immediately without any readiness sleep.
	if err := srv.Listen(ctx); err != nil {
		cancel()
		t.Fatalf("Listen: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	return cancel, serveErr
}

// TestServeHandlesConnectionsConcurrently proves the accept loop is not
// serial: the first connection is parked inside the handler, and while it is
// still parked a second request must be accepted and answered. On the old
// serial Serve() the second SendRequest could never complete because the
// accept loop was stuck in handleConnection — this test then fails via the
// watchdog with that exact message.
func TestServeHandlesConnectionsConcurrently(t *testing.T) {
	socketPath := tempSocketPath(t)

	blockedStarted := make(chan struct{})
	release := make(chan struct{})

	handler := func(ctx context.Context, req Request) Response {
		if req.RequestID == "blocking" {
			close(blockedStarted)
			<-release // hold connection 1 inside the handler
		}
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}

	cancel, serveErr := startTestServer(t, socketPath, handler, 0)
	defer cancel()

	// Connection 1: enters the handler and blocks there.
	firstDone := make(chan sendResult, 1)
	go func() {
		resp, err := SendRequest(socketPath, Request{
			Version:   ProtocolVersion,
			Type:      TypeTranslate,
			RequestID: "blocking",
		})
		firstDone <- sendResult{resp, err}
	}()

	// Deterministic sync point: connection 1's handler is now running and
	// will not return until release is closed below.
	<-blockedStarted

	// Connection 2: must be accepted and handled while connection 1 is
	// still blocked in its handler.
	secondDone := make(chan sendResult, 1)
	go func() {
		resp, err := SendRequest(socketPath, Request{
			Version:   ProtocolVersion,
			Type:      TypeStatus,
			RequestID: "second",
		})
		secondDone <- sendResult{resp, err}
	}()

	var second sendResult
	select {
	case second = <-secondDone:
		if second.err != nil {
			t.Fatalf("second request: %v", second.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second request blocked while the first handler was still running: Serve handles connections serially")
	}
	if second.resp.RequestID != "second" {
		t.Errorf("second response request_id = %q, want %q", second.resp.RequestID, "second")
	}
	if !second.resp.OK {
		t.Errorf("second response OK=false, error=%q", second.resp.Error)
	}

	// Only now let the first handler finish; both requests must succeed.
	close(release)

	first := <-firstDone
	if first.err != nil {
		t.Fatalf("first request: %v", first.err)
	}
	if first.resp.RequestID != "blocking" {
		t.Errorf("first response request_id = %q, want %q", first.resp.RequestID, "blocking")
	}
	if !first.resp.OK {
		t.Errorf("first response OK=false, error=%q", first.resp.Error)
	}

	cancel()
	// Drain Serve's return without asserting it: on graceful shutdown
	// cleanup() reports "no such file" because net.UnixListener already
	// unlinked the socket on Close. That pre-existing condition is swallowed
	// in production (runtime checks ctx.Err() == nil before reporting).
	<-serveErr
}

// TestServerConnectionDeadlineClosesSilentConnection proves handleConnection
// sets a read deadline: a client that connects and never sends a request
// gets its connection closed after the server deadline instead of holding a
// goroutine open forever. Without a deadline this test fails via the
// watchdog.
func TestServerConnectionDeadlineClosesSilentConnection(t *testing.T) {
	socketPath := tempSocketPath(t)

	handlerCalled := make(chan struct{}, 1)
	handler := func(ctx context.Context, req Request) Response {
		handlerCalled <- struct{}{}
		return Response{Version: ProtocolVersion, RequestID: req.RequestID, OK: true}
	}

	cancel, serveErr := startTestServer(t, socketPath, handler, 100*time.Millisecond)
	defer cancel()

	// Connect but deliberately send nothing.
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
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
			t.Errorf("silent connection read = %v, want io.EOF (server closed it after the deadline)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never closed a connection that sent no request: no deadline enforced")
	}

	select {
	case <-handlerCalled:
		t.Error("handler ran for a connection that never sent a request")
	default:
	}

	cancel()
	<-serveErr
}

// TestClientDeadlineBoundsHungServer proves SendRequest cannot hang forever
// on a server that accepts the connection but never responds.
func TestClientDeadlineBoundsHungServer(t *testing.T) {
	socketPath := tempSocketPath(t)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	hold := make(chan struct{})
	defer close(hold)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-hold // accept, then never respond
	}()

	result := make(chan error, 1)
	go func() {
		_, err := sendRequest(socketPath, Request{
			Version:   ProtocolVersion,
			Type:      TypeStatus,
			RequestID: "hung",
		}, 100*time.Millisecond)
		result <- err
	}()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected a deadline error from a server that never responds, got success")
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Errorf("error = %v, want os.ErrDeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendRequest hung: no client-side deadline")
	}
}

// TestDefaultConnDeadlineExceedsProviderTimeout pins the safety invariant:
// the IPC connection deadline must stay well above the default provider
// timeout (30s in internal/config) so a legitimate slow translation is never
// cut off mid-flight. Keep this margin when touching defaultConnDeadline.
func TestDefaultConnDeadlineExceedsProviderTimeout(t *testing.T) {
	const defaultProviderTimeout = 30 * time.Second
	if defaultConnDeadline <= 2*defaultProviderTimeout {
		t.Errorf("defaultConnDeadline = %v, must be well above the default provider timeout (%v)",
			defaultConnDeadline, defaultProviderTimeout)
	}
}
