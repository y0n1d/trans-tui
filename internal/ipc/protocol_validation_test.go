package ipc

// These tests cover P2-4: protocol hygiene at the IPC boundary. They pin the
// wire contract of ProtocolVersion = 1 and the legal request-type set
// (translate, status, enter_input_mode, display_text) on the production
// path — a real ipc.Server via startTestServer and the real SendRequest
// client — never by re-implementing validation inside the test. No sleeps
// are used for sequencing: Listen binds synchronously and every response is
// observed through the actual socket exchange.

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// recordingHandler returns a handler that records every request it receives
// and answers OK, so tests can assert both what the caller observed and
// whether the handler ran at all.
func recordingHandler(calls chan<- Request) Handler {
	return func(ctx context.Context, req Request) Response {
		calls <- req
		return Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        true,
		}
	}
}

// assertHandlerNotCalled fails if the handler ran for a request that the
// server must have rejected before dispatch. The handler executes — and
// sends — before the server writes any response on the same connection, so
// once the client has observed the response, a buggy dispatch would already
// be visible in the buffered channel.
func assertHandlerNotCalled(t *testing.T, calls chan Request) {
	t.Helper()
	select {
	case called := <-calls:
		t.Errorf("handler ran for an invalid request: %+v", called)
	default:
	}
}

// TestServerRejectsUnsupportedProtocolVersion proves the server enforces the
// request version before dispatch: anything other than ProtocolVersion —
// including the zero value produced by a request that omits the field — must
// get an explicit OK=false error response, must not reach the handler, and
// must still get the request_id echoed so the caller can correlate the
// failure.
func TestServerRejectsUnsupportedProtocolVersion(t *testing.T) {
	cases := []struct {
		name    string
		version int
	}{
		{name: "future version", version: ProtocolVersion + 1},
		{name: "arbitrary version", version: 99},
		{name: "omitted field decodes to zero", version: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			socketPath := tempSocketPath(t)
			calls := make(chan Request, 1)

			cancel, _ := startTestServer(t, socketPath, recordingHandler(calls), 0)
			defer cancel()

			const requestID = "bad-version"
			resp, err := SendRequest(socketPath, Request{
				Version:   tc.version,
				Type:      TypeTranslate,
				RequestID: requestID,
				Text:      "hello",
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if resp.OK {
				t.Errorf("OK = true for protocol version %d, want explicit rejection", tc.version)
			}
			if !strings.Contains(resp.Error, "protocol version") {
				t.Errorf("error = %q, want it to mention the unsupported protocol version", resp.Error)
			}
			if resp.Version != ProtocolVersion {
				t.Errorf("response version = %d, want the server's own version %d", resp.Version, ProtocolVersion)
			}
			if resp.RequestID != requestID {
				t.Errorf("response request_id = %q, want %q", resp.RequestID, requestID)
			}
			assertHandlerNotCalled(t, calls)
		})
	}
}

// TestServerRejectsInvalidRequestType proves the legal type set is enforced
// before dispatch: an unknown type must not fall through to some default
// handler branch, and an empty/missing type — the zero value of the field —
// must be rejected explicitly rather than treated as a valid request.
func TestServerRejectsInvalidRequestType(t *testing.T) {
	cases := []struct {
		name    string
		typ     string
		wantErr string
	}{
		{name: "unknown type", typ: "frobnicate", wantErr: `unknown request type "frobnicate"`},
		{name: "empty type", typ: "", wantErr: "missing request type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			socketPath := tempSocketPath(t)
			calls := make(chan Request, 1)

			cancel, _ := startTestServer(t, socketPath, recordingHandler(calls), 0)
			defer cancel()

			const requestID = "bad-type"
			resp, err := SendRequest(socketPath, Request{
				Version:   ProtocolVersion,
				Type:      tc.typ,
				RequestID: requestID,
				Text:      "hello",
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if resp.OK {
				t.Errorf("OK = true for request type %q, want explicit rejection", tc.typ)
			}
			if !strings.Contains(resp.Error, tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", resp.Error, tc.wantErr)
			}
			if resp.RequestID != requestID {
				t.Errorf("response request_id = %q, want %q", resp.RequestID, requestID)
			}
			assertHandlerNotCalled(t, calls)
		})
	}
}

// TestServerAcceptsAllValidRequestTypes pins the other side of the contract:
// every currently legal request type, sent with the current ProtocolVersion,
// is dispatched to the handler and answered OK. This is the regression guard
// that the stricter validation never rejects a legitimate request.
func TestServerAcceptsAllValidRequestTypes(t *testing.T) {
	for _, typ := range []string{TypeTranslate, TypeStatus, TypeEnterInputMode, TypeDisplayText} {
		t.Run(typ, func(t *testing.T) {
			socketPath := tempSocketPath(t)
			calls := make(chan Request, 1)

			cancel, _ := startTestServer(t, socketPath, recordingHandler(calls), 0)
			defer cancel()

			requestID := "valid-" + typ
			resp, err := SendRequest(socketPath, Request{
				Version:   ProtocolVersion,
				Type:      typ,
				RequestID: requestID,
				Text:      "hello",
			})
			if err != nil {
				t.Fatalf("SendRequest: %v", err)
			}
			if !resp.OK {
				t.Errorf("legal request type %q rejected: %s", typ, resp.Error)
			}
			if resp.RequestID != requestID {
				t.Errorf("response request_id = %q, want %q", resp.RequestID, requestID)
			}

			// The handler ran before the response was written, so its record
			// is already buffered; no polling needed.
			select {
			case called := <-calls:
				if called.Type != typ {
					t.Errorf("handler saw type %q, want %q", called.Type, typ)
				}
			default:
				t.Errorf("handler did not run for legal request type %q", typ)
			}
		})
	}
}

// TestSendRequestRejectsUnsupportedResponseVersion covers the client half of
// the version contract: SendRequest must not hand a response to its caller
// when the peer speaks a different protocol version — or omits the version
// field entirely, which decodes to the zero value. The fake peer speaks the
// real framing (WriteMessage/ReadMessage), only the version number is wrong.
func TestSendRequestRejectsUnsupportedResponseVersion(t *testing.T) {
	cases := []struct {
		name    string
		version int
	}{
		{name: "future version", version: ProtocolVersion + 1},
		{name: "omitted field decodes to zero", version: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			socketPath := tempSocketPath(t)

			ln, err := net.Listen("unix", socketPath)
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer ln.Close()

			go func() {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				var req Request
				if err := ReadMessage(conn, &req); err != nil {
					return
				}
				_ = WriteMessage(conn, Response{
					Version:     tc.version,
					RequestID:   req.RequestID,
					OK:          true,
					Translation: "must not be trusted",
				})
			}()

			resp, err := SendRequest(socketPath, Request{
				Version:   ProtocolVersion,
				Type:      TypeStatus,
				RequestID: "bad-resp-version",
			})
			if err == nil {
				t.Fatalf("SendRequest accepted a response with protocol version %d: %+v", tc.version, resp)
			}
			if !strings.Contains(err.Error(), "protocol version") {
				t.Errorf("error = %q, want it to mention the unsupported protocol version", err.Error())
			}
		})
	}
}

// TestServerMalformedRequestNeverReachesHandler pins the existing semantics
// for a frame that cannot be parsed at all: there is no request_id to echo,
// so the server logs the parse failure and closes the connection without a
// response — which the caller observes as an explicit read error (EOF), and
// the handler never runs.
func TestServerMalformedRequestNeverReachesHandler(t *testing.T) {
	socketPath := tempSocketPath(t)
	calls := make(chan Request, 1)

	cancel, _ := startTestServer(t, socketPath, recordingHandler(calls), 0)
	defer cancel()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// A correctly framed payload that is not valid JSON.
	payload := []byte(`{"version":1,"type":`)
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := conn.Write(lenBuf[:]); err != nil {
		t.Fatalf("write length prefix: %v", err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 8)
		_, err := conn.Read(buf)
		readDone <- err
	}()

	select {
	case err := <-readDone:
		if !errors.Is(err, io.EOF) {
			t.Errorf("read after malformed request = %v, want io.EOF (server rejected it by closing)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server never closed the connection carrying a malformed request")
	}

	assertHandlerNotCalled(t, calls)
}
