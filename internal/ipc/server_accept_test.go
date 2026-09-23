package ipc

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// These tests cover the accept-loop error handling (P3-3): which Accept
// errors stop Serve, which are retried, and that no non-shutdown error can
// turn the loop into a busy loop. All failures are scripted through a fake
// listener with their real net wrapping — no real socket error, no sleep
// guessing. The 5s selects are watchdogs that only fire on a broken
// implementation; every success path is event-driven.

// fakeListener is a scripted net.Listener: the accept callback decides what
// each Accept call returns, and the counters make "re-entered Accept" and
// "tore the listener down" observable facts instead of timing guesses.
type fakeListener struct {
	accept func(call int64) (net.Conn, error)
	calls  atomic.Int64
	closes atomic.Int64
}

func (l *fakeListener) Accept() (net.Conn, error) {
	return l.accept(l.calls.Add(1))
}

func (l *fakeListener) Close() error {
	l.closes.Add(1)
	return nil
}

func (l *fakeListener) Addr() net.Addr { return fakeAddr{} }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "unix" }
func (fakeAddr) String() string  { return "fake.sock" }

// startFakeServe runs Serve on a server wired directly to the fake listener,
// bypassing Listen so no real socket is ever created. It mirrors
// startTestServer's (cancel, serveErr) shape.
func startFakeServe(t *testing.T, ln net.Listener) (context.CancelFunc, <-chan error) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	srv := NewServer(tempSocketPath(t), func(context.Context, Request) Response {
		return Response{Version: ProtocolVersion, OK: true}
	})
	srv.listener = ln

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ctx) }()
	return cancel, serveErr
}

// TestServeReturnsOnNonTemporaryAcceptError locks the busy-loop fix. A
// non-temporary accept error can never clear by retrying: the old
// implementation logged it and immediately re-Accepted unconditionally,
// which for such an error is an infinite 100%-CPU loop emitting one log line
// per iteration — this test's watchdog reproduces exactly that on the old
// code ("Serve kept re-Accepting..."). Serve must instead stop after the
// single failing Accept, tear the listener down, and return the original
// cause so runtime's non-shutdown Serve report shows the real error.
func TestServeReturnsOnNonTemporaryAcceptError(t *testing.T) {
	// A (buggy) spinning implementation would otherwise flood the test
	// output during the watchdog window; the assertion is on control flow,
	// not on log volume.
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// EINVAL through the same wrapping net uses for accept failures; probed
	// to classify Temporary() == false under the current Go version.
	errAccept := &net.OpError{Op: "accept", Net: "unix", Err: os.NewSyscallError("accept4", syscall.EINVAL)}
	ln := &fakeListener{accept: func(int64) (net.Conn, error) {
		return nil, errAccept
	}}

	cancel, serveErr := startFakeServe(t, ln)
	defer cancel()

	select {
	case err := <-serveErr:
		if !errors.Is(err, errAccept) {
			t.Errorf("Serve error = %v, want it to wrap the original accept error %v", err, errAccept)
		}
	case <-time.After(5 * time.Second):
		// Stop the old implementation's spin before flagging the failure so
		// it cannot keep flooding stderr for the rest of the suite.
		cancel()
		t.Fatal("Serve kept re-Accepting a non-temporary accept error instead of stopping: busy loop")
	}

	if got := ln.calls.Load(); got != 1 {
		t.Errorf("Accept called %d times, want exactly 1: a non-temporary error must stop the loop", got)
	}
	if got := ln.closes.Load(); got != 1 {
		t.Errorf("listener Close called %d times, want 1: stopping must tear the listener down, not leave a bound socket nobody accepts", got)
	}
}

// TestServeShutdownAcceptErrorStopsLoop locks the shutdown semantics that
// must survive any accept-error handling: when the Accept failure is caused
// by shutdown — ctx cancelled, listener closed by Listen's goroutine — Serve
// returns through cleanup after that single failure and must never fall into
// the retry or failure paths. The ctx is cancelled before Serve observes the
// error, mirroring the real ordering: the listener is only ever closed after
// ctx.Done has fired. The return value is deliberately not asserted (same
// pre-existing cleanup()'s os.Remove condition as the other shutdown tests).
func TestServeShutdownAcceptErrorStopsLoop(t *testing.T) {
	ln := &fakeListener{accept: func(int64) (net.Conn, error) {
		return nil, &net.OpError{Op: "accept", Net: "unix", Err: net.ErrClosed}
	}}

	cancel, serveErr := startFakeServe(t, ln)
	cancel() // cancellation precedes the failing Accept, exactly like Listen's ctx→Close sequence

	select {
	case <-serveErr:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after a shutdown-caused accept error")
	}

	if got := ln.calls.Load(); got != 1 {
		t.Errorf("Accept called %d times, want exactly 1: shutdown must not re-enter Accept", got)
	}
	if got := ln.closes.Load(); got != 1 {
		t.Errorf("listener Close called %d times, want 1: shutdown must run cleanup", got)
	}
}

// TestServeRetriesTemporaryAcceptErrorWithoutExiting locks the other side
// of the classification: a temporary accept error — realistically EMFILE
// from fd exhaustion, built here with its real net wrapping — must be
// retried, never returned. Serve has to come back for a second Accept (the
// deterministic signal below) and stop only once shutdown arrives. This
// fails an implementation that mechanically returns on every accept error.
func TestServeRetriesTemporaryAcceptErrorWithoutExiting(t *testing.T) {
	// Probed to classify Temporary() == true under the current Go version.
	errTemporary := &net.OpError{Op: "accept", Net: "unix", Err: os.NewSyscallError("accept4", syscall.EMFILE)}
	errClosed := &net.OpError{Op: "accept", Net: "unix", Err: net.ErrClosed}

	reachedSecondAccept := make(chan struct{})
	proceed := make(chan struct{})
	var signaled sync.Once
	ln := &fakeListener{accept: func(call int64) (net.Conn, error) {
		if call == 1 {
			return nil, errTemporary
		}
		signaled.Do(func() { close(reachedSecondAccept) })
		<-proceed
		return nil, errClosed
	}}

	cancel, serveErr := startFakeServe(t, ln)
	defer cancel()

	select {
	case <-reachedSecondAccept:
		// Serve re-entered Accept after the temporary error: it retried
		// instead of exiting.
	case err := <-serveErr:
		t.Fatalf("Serve returned on a temporary accept error that must be retried: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve never re-entered Accept after a temporary accept error")
	}

	cancel()       // shutdown first: cancellation is what closes the real listener
	close(proceed) // then let the pending Accept fail as a closed listener would

	select {
	case <-serveErr:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancellation during the retry path")
	}

	if got := ln.calls.Load(); got != 2 {
		t.Errorf("Accept called %d times, want exactly 2 (one temporary failure, then one shutdown failure)", got)
	}
}

// TestNextAcceptRetryDelay locks the backoff ladder that keeps a persistent
// temporary error from becoming a busy loop: first retry after 5ms (prev == 0,
// also after a successful accept resets it), then doubling, capped at 1s so a
// long fd-pressure episode still retries once per second. The growth is a
// pure function — no timers, no sleeps.
func TestNextAcceptRetryDelay(t *testing.T) {
	cases := []struct {
		prev time.Duration
		want time.Duration
	}{
		{0, 5 * time.Millisecond}, // first retry / reset after success
		{5 * time.Millisecond, 10 * time.Millisecond},
		{10 * time.Millisecond, 20 * time.Millisecond},
		{640 * time.Millisecond, time.Second}, // doubling overshoots the cap
		{time.Second, time.Second},            // cap holds under continued failure
	}
	for _, tc := range cases {
		if got := nextAcceptRetryDelay(tc.prev); got != tc.want {
			t.Errorf("nextAcceptRetryDelay(%v) = %v, want %v", tc.prev, got, tc.want)
		}
	}
}
