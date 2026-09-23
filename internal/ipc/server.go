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

// Accept-retry backoff bounds how fast the accept loop retries after a
// temporary error. The ladder (5ms doubling to 1s) is net/http's long-standing
// policy for the identical situation: retrying immediately would busy-loop and
// flood the log for as long as the condition — realistically EMFILE/ENFILE fd
// pressure — lasts.
const (
	initialAcceptRetryDelay = 5 * time.Millisecond
	maxAcceptRetryDelay     = time.Second
)

// nextAcceptRetryDelay returns the wait before the next accept retry given the
// previous wait: initialAcceptRetryDelay for the first retry (prev == 0, which
// also follows every successful accept, resetting the sequence), then doubling
// up to maxAcceptRetryDelay.
func nextAcceptRetryDelay(prev time.Duration) time.Duration {
	if prev == 0 {
		return initialAcceptRetryDelay
	}
	next := prev * 2
	if next > maxAcceptRetryDelay {
		return maxAcceptRetryDelay
	}
	return next
}

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
// cannot block subsequent clients in the accept loop. A temporary Accept
// failure is retried behind a bounded, cancellation-aware backoff; a
// non-temporary one stops the loop and is returned as-is. It must follow a
// successful Listen; a Serve error therefore only ever surfaces a shutdown or
// an unexpected stop, never an unnoticed bind failure.
func (s *Server) Serve(ctx context.Context) error {
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln == nil {
		return fmt.Errorf("serve without listener: call Listen first")
	}

	var retryDelay time.Duration
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Shutdown first: the listener is only ever closed because ctx
			// was cancelled (the goroutine started in Listen waits on
			// ctx.Done, and cleanup runs solely from this branch), so a
			// closed-listener error must return through cleanup before any
			// retry/failure classification runs.
			select {
			case <-ctx.Done():
				return s.cleanup()
			default:
			}

			// Temporary errors — realistically fd exhaustion (EMFILE/ENFILE);
			// EINTR/EAGAIN/ECONNABORTED never surface because internal/poll
			// retries them itself — can clear on their own, so retry, but
			// behind a bounded backoff: immediately re-Accepting would
			// busy-loop and flood the log while the condition lasts. The wait
			// selects on ctx.Done so shutdown stays prompt. Temporary is
			// deprecated but remains the standard library's own classifier —
			// net/http's accept loop uses it identically.
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				retryDelay = nextAcceptRetryDelay(retryDelay)
				log.Printf("accept error: %v (retrying in %v)", err, retryDelay)
				select {
				case <-ctx.Done():
					return s.cleanup()
				case <-time.After(retryDelay):
				}
				continue
			}

			// Non-temporary (e.g. EINVAL, or a closed listener with no
			// cancellation): retrying cannot help — stop instead of spinning,
			// tear the listener down so no bound socket without an accept
			// loop is left behind, and surface the real cause. runtime
			// reports a Serve return that happens outside shutdown.
			log.Printf("accept error: %v", err)
			_ = s.cleanup()
			return fmt.Errorf("accept failed: %w", err)
		}
		retryDelay = 0 // a successful accept restarts the backoff ladder

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
