package ipc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Handler func(ctx context.Context, req Request) Response

// defaultConnDeadline bounds one connection end-to-end: reading the request,
// running the handler (a synchronous Translate bounded by the provider HTTP
// timeout, default 30s) and writing the response. It must stay well above
// that provider timeout so slow-but-legitimate translations are never cut
// off; 2 minutes is 4x the default. The ipc package cannot read the
// configured provider timeout (config lives in internal/config, and
// threading it through would touch runtime, outside this change), so this is
// a deliberately conservative constant, overridable per server for tests.
const defaultConnDeadline = 2 * time.Minute

type Server struct {
	socketPath string
	handler    Handler
	listener   net.Listener
	mu         sync.Mutex

	// connDeadline overrides defaultConnDeadline when non-zero (tests).
	connDeadline time.Duration
}

func NewServer(socketPath string, handler Handler) *Server {
	return &Server{
		socketPath: socketPath,
		handler:    handler,
	}
}

// Listen binds the unix socket. The whole startup sequence — socket directory
// creation, stale socket removal and net.Listen — runs synchronously so every
// bind failure is returned to the caller before serving begins. Callers can
// therefore observe "no socket exists" instead of discovering it later.
func (s *Server) Listen(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.listener != nil {
			s.listener.Close()
		}
	}()

	return nil
}

// Serve accepts connections until ctx is cancelled, then closes the listener
// and removes the socket file. Each accepted connection is handled in its own
// goroutine so one slow request (e.g. a provider translate near its timeout)
// cannot block subsequent clients in the accept loop. It must follow a
// successful Listen; a Serve error therefore only ever surfaces a shutdown or
// an unexpected stop, never an unnoticed bind failure.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln == nil {
		return fmt.Errorf("serve without listener: call Listen first")
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return s.cleanup()
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}

		go s.handleConnection(ctx, conn)
	}
}

// ListenAndServe binds the socket and serves until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if err := s.Listen(ctx); err != nil {
		return err
	}
	return s.Serve(ctx)
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	// Bound the connection's I/O — reading the request and writing the
	// response — so a stuck peer cannot leak this goroutine forever. The
	// deadline spans the handler as well, but cannot interrupt handler code
	// itself (SetDeadline only cuts off blocked syscalls); a handler that
	// never returns is bounded by its own provider timeout instead.
	deadline := s.connDeadline
	if deadline == 0 {
		deadline = defaultConnDeadline
	}
	_ = conn.SetDeadline(time.Now().Add(deadline))

	var req Request
	if err := ReadMessage(conn, &req); err != nil {
		if !errors.Is(err, io.EOF) {
			log.Printf("read request error: %v", err)
		}
		return
	}

	// Protocol hygiene: reject an unsupported version or an unknown/empty
	// request type with an explicit error response before the handler runs,
	// instead of letting it fall through to whatever the handler does with
	// unrecognized input. The response carries the server's own version and
	// echoes the request_id so the caller can correlate the failure.
	if err := validateRequest(req); err != nil {
		if werr := WriteMessage(conn, Response{
			Version:   ProtocolVersion,
			RequestID: req.RequestID,
			OK:        false,
			Error:     err.Error(),
		}); werr != nil {
			log.Printf("write validation error response: %v", werr)
		}
		return
	}

	resp := s.handler(ctx, req)

	if err := WriteMessage(conn, resp); err != nil {
		log.Printf("write response error: %v", err)
	}
}

func (s *Server) cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
	return os.Remove(s.socketPath)
}
