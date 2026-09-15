package ipc

import (
	"context"
	"fmt"
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

func (s *Server) ListenAndServe(ctx context.Context) error {
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

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	var req Request
	if err := ReadMessage(conn, &req); err != nil {
		log.Printf("read request error: %v", err)
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
