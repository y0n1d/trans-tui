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
)

type Handler func(ctx context.Context, req Request) Response

type Server struct {
	socketPath string
	handler    Handler
	listener   net.Listener
	mu         sync.Mutex
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
// and removes the socket file. It must follow a successful Listen; a Serve
// error therefore only ever surfaces a shutdown or an unexpected stop, never
// an unnoticed bind failure.
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

		s.handleConnection(ctx, conn)
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

	var req Request
	if err := ReadMessage(conn, &req); err != nil {
		if !errors.Is(err, io.EOF) {
			log.Printf("read request error: %v", err)
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
